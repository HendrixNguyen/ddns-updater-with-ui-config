package params

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"strings"

	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/internal/provider"
	"github.com/qdm12/ddns-updater/internal/provider/constants"
	"github.com/qdm12/ddns-updater/internal/provider/utils"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
	"golang.org/x/net/publicsuffix"
)

const (
	// configEnvVar is the name of the environment variable containing the DDNS
	// settings as a JSON document.
	configEnvVar = "CONFIG"
	// emptyDocument is the JSON document created when the settings file is
	// missing and the environment variable is unset.
	emptyDocument = "{}"
)

type commonSettings struct {
	Provider string `json:"provider"`
	Domain   string `json:"domain"`
	// Host is kept for retro-compatibility and is replaced by Owner.
	Host string `json:"host,omitempty"`
	// Owner is kept for retro-compatibility and is determined from the
	// Domain field.
	Owner      string       `json:"owner,omitempty"`
	IPVersion  string       `json:"ip_version"`
	IPv6Suffix netip.Prefix `json:"ipv6_suffix"`
	// Retro values for warnings
	ProviderIP *bool `json:"provider_ip,omitempty"`
}

// JSONProviders obtain the update settings from the JSON content.
// The file config.json is the source of truth, unless the environment variable
// CONFIG is set and differs from what was last synchronized to the file. In that
// latter case the environment variable wins and is written to the file, so that
// settings edited through the web UI are not clobbered on restart when the
// environment variable did not change.
func (r *Reader) JSONProviders(filePath string) (
	providers []provider.Provider, warnings []string, err error,
) {
	rawBytes, fromEnv, err := r.resolveRawSettings(filePath)
	if err != nil {
		return nil, nil, err
	}

	providers, warnings, err = extractAllSettings(rawBytes)
	if err != nil && fromEnv {
		err = fmt.Errorf("configuration given: %w", err)
	}

	return providers, warnings, err
}

var errWriteConfigToFile = errors.New("cannot write configuration to file")

// resolveRawSettings returns the raw JSON bytes to use to build the providers
// from, reading the environment variable CONFIG value to use. It also returns
// whether these bytes come from the environment variable.
func (r *Reader) resolveRawSettings(filePath string) (rawBytes []byte, fromEnv bool, err error) {
	return r.resolveRawSettingsFrom(filePath, os.Getenv(configEnvVar))
}

// resolveRawSettingsFrom returns the raw JSON bytes to use to build the
// providers from, using the given environment variable CONFIG value.
// The file is only overwritten when the environment variable is set and cannot
// be proven to be unchanged since it was last written to the file.
func (r *Reader) resolveRawSettingsFrom(filePath, envValue string) (
	rawBytes []byte, fromEnv bool, err error,
) {
	seed := envSeedOf(envValue)
	rawBytes, readErr := r.readFile(filePath)
	fileMissing := errors.Is(readErr, os.ErrNotExist)
	if readErr != nil && !fileMissing {
		return nil, false, readErr
	}

	if envValue == "" {
		// The environment variable is unset, so the file is the source of truth.
		if fileMissing {
			r.logger.Info("file not found, creating an empty settings file")

			const filePerm = fs.FileMode(0o666)
			err = r.writeFile(filePath, []byte(emptyDocument), filePerm)
			if err != nil {
				return nil, false, fmt.Errorf("%w: %w", errWriteConfigToFile, err)
			}

			return []byte(emptyDocument), false, nil
		}

		r.logger.Info("reading JSON config from file " + filePath)
		r.logger.Debug("config read: " + string(rawBytes))

		return rawBytes, false, nil
	}

	if !fileMissing && r.envSeedMatches(rawBytes, seed) {
		// The environment variable did not change since it was last synchronized
		// to the file, so the file, possibly edited through the web UI, wins.
		r.logger.Info(configEnvVar + " environment variable unchanged, using config file " + filePath)
		r.logger.Debug("config read: " + string(rawBytes))

		return rawBytes, false, nil
	}

	// Either the seed is missing because the file predates this feature, or the
	// environment variable changed, so the environment variable wins.
	r.logger.Info("reading JSON config from environment variable " + configEnvVar)
	r.logger.Debug("config read: " + envValue)

	envBytes, envErr := documentFromEnv(envValue, seed)
	if envErr != nil {
		// The environment variable cannot be parsed as a JSON document, so the
		// regular parsing path reports the error and the file is left untouched.
		return []byte(envValue), true, nil
	}

	const filePerm = fs.FileMode(0o666)
	err = r.writeFile(filePath, envBytes, filePerm)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %w", errWriteConfigToFile, err)
	}

	return envBytes, true, nil
}

// envSeedMatches returns true if the file at the given raw bytes content was
// last synchronized from an environment variable having the given seed.
func (r *Reader) envSeedMatches(rawBytes []byte, seed string) bool {
	doc, _, err := parseDocument(rawBytes)
	if err != nil { // a malformed document has no seed
		return false
	}

	return doc.EnvSeed != "" && doc.EnvSeed == seed
}

// documentFromEnv builds the JSON document to write to the settings file from
// the environment variable value, stamping it with the given seed.
func documentFromEnv(envValue, seed string) (data []byte, err error) {
	doc, extra, err := parseDocument([]byte(envValue))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errUnmarshalRaw, err)
	}

	doc.EnvSeed = seed

	return marshalDocument(extra, doc)
}

// EnvSeedOf returns the hex SHA-256 seed of the given environment variable
// value, which is stored in the settings file when it is synchronized from the
// environment variable. It returns an empty string for an empty value.
// It allows callers to report whether the running configuration comes from
// the environment variable or from the config file.
func EnvSeedOf(value string) string {
	return envSeedOf(value)
}

func envSeedOf(value string) string {
	if value == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(value))

	return hex.EncodeToString(sum[:])
}

var (
	errUnmarshalCommon = errors.New("cannot unmarshal common settings")
	errUnmarshalRaw    = errors.New("cannot unmarshal raw configuration")
)

func extractAllSettings(jsonBytes []byte) (
	allProviders []provider.Provider, warnings []string, err error,
) {
	config := struct {
		CommonSettings []commonSettings `json:"settings"`
	}{}
	rawConfig := struct {
		Settings []json.RawMessage `json:"settings"`
	}{}
	err = json.Unmarshal(jsonBytes, &config)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errUnmarshalCommon, err)
	}
	err = json.Unmarshal(jsonBytes, &rawConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errUnmarshalRaw, err)
	}
	// TODO(v3): remove retro compatibility with IPV6_PREFIX
	retroIPv6Suffix, err := getRetroIPv6Suffix()
	if err != nil {
		return nil, nil, fmt.Errorf("getting retro-compatible global IPV6 suffix: %w", err)
	}

	for i, common := range config.CommonSettings {
		newProvider, newWarnings, err := makeSettingsFromObject(common, rawConfig.Settings[i],
			retroIPv6Suffix)
		warnings = append(warnings, newWarnings...)
		if err != nil {
			return nil, warnings, err
		}
		allProviders = append(allProviders, newProvider...)
	}

	return allProviders, warnings, nil
}

var (
	ErrProviderNoLongerSupported = errors.New("provider no longer supported")
	ErrProviderMultipleDomains   = errors.New("provider does not support multiple domains")
)

func makeSettingsFromObject(common commonSettings, rawSettings json.RawMessage,
	retroGlobalIPv6Suffix netip.Prefix) (
	providers []provider.Provider, warnings []string, err error,
) {
	if common.Provider == "google" {
		return nil, nil, fmt.Errorf("%w: %s", ErrProviderNoLongerSupported, common.Provider)
	}

	if common.Owner == "" { // retro compatibility
		common.Owner = common.Host
	}

	var domain string
	var owners []string
	if common.Owner != "" { // retro compatibility
		owners = strings.Split(common.Owner, ",")
		domain = common.Domain // single domain only
		domains := make([]string, len(owners))
		for i, owner := range owners {
			domains[i] = utils.BuildURLQueryHostname(owner, common.Domain)
		}
		warnings = append(warnings,
			fmt.Sprintf("you can specify the owner %q directly in the domain field as %q",
				common.Owner, strings.Join(domains, ",")))
	} else { // extract owner(s) from domain(s)
		domain, owners, err = extractFromDomainField(common.Domain)
		if err != nil {
			return nil, nil, fmt.Errorf("extracting owners from domains: %w", err)
		}
	}

	if common.Provider == string(constants.Myaddr) && len(owners) > 1 {
		return nil, nil, fmt.Errorf("%w: %s for parent domain %q",
			ErrProviderMultipleDomains, common.Provider, domain)
	}

	if common.IPVersion == "" {
		common.IPVersion = ipversion.IP4or6.String()
	}
	ipVersion, err := ipversion.Parse(common.IPVersion)
	if err != nil {
		return nil, nil, err
	}

	ipv6Suffix := common.IPv6Suffix
	if !ipv6Suffix.IsValid() {
		ipv6Suffix = retroGlobalIPv6Suffix
	}

	if ipVersion == ipversion.IP4 && ipv6Suffix.IsValid() {
		warnings = append(warnings,
			fmt.Sprintf("IPv6 suffix specified as %s but IP version is %s",
				ipv6Suffix, ipVersion))
	}

	if common.ProviderIP != nil {
		warning := fmt.Sprintf("for domain %s and ip version %s: "+
			`the field "provider_ip" is deprecated and no longer used`,
			domain, ipVersion)
		warnings = append(warnings, warning)
	}

	providerName := models.Provider(common.Provider)
	if providerName == constants.Hetzner {
		warnings = append(warnings,
			"You should use the hetznercloud with the new Hetzner Cloud console instead, "+
				"given this legacy Hetzner API is going to be shutdown soon.")
	}
	providers = make([]provider.Provider, len(owners))
	for i, owner := range owners {
		owner = strings.TrimSpace(owner)
		providers[i], err = provider.New(providerName, rawSettings, domain,
			owner, ipVersion, ipv6Suffix)
		if err != nil {
			return nil, warnings, err
		}
	}
	return providers, warnings, nil
}

var ErrMultipleDomainsSpecified = errors.New("multiple domains specified")

func extractFromDomainField(domainField string) (domainRegistered string,
	owners []string, err error,
) {
	domains := strings.Split(domainField, ",")
	owners = make([]string, len(domains))
	for i, domain := range domains {
		newDomainRegistered, err := publicsuffix.EffectiveTLDPlusOne(domain)
		switch {
		case err != nil:
			return "", nil, fmt.Errorf("extracting effective TLD+1: %w", err)
		case domainRegistered == "":
			domainRegistered = newDomainRegistered
		case domainRegistered != newDomainRegistered:
			return "", nil, fmt.Errorf("%w: %q and %q",
				ErrMultipleDomainsSpecified, domainRegistered, newDomainRegistered)
		}
		if domain == domainRegistered {
			owners[i] = "@"
			continue
		}
		owners[i] = strings.TrimSuffix(domain, "."+domainRegistered)
	}
	return domainRegistered, owners, nil
}
