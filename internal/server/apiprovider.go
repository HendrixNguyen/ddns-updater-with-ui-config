package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"github.com/qdm12/ddns-updater/internal/provider/constants"
)

// Field types a settings entry field can have, so the web UI knows which form
// control to render.
const (
	fieldTypeText     = "text"
	fieldTypePassword = "password"
	fieldTypeNumber   = "number"
	fieldTypeCheckbox = "checkbox"
	fieldTypeTextarea = "textarea"
)

// docFilepathFormat is the format of the repository relative path of the
// documentation page of a provider.
const docFilepathFormat = "docs/%s.md"

// apiField describes a single provider specific settings entry key, so the web
// UI can build a form for it.
type apiField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Help        string `json:"help"`
	Placeholder string `json:"placeholder"`
}

// apiProvider describes a DNS provider and the fields its settings entry
// accepts. An empty Fields slice tells the web UI to fall back to a free-form
// JSON editor.
type apiProvider struct {
	Name   string     `json:"name"`
	Label  string     `json:"label"`
	Doc    string     `json:"doc"`
	Fields []apiField `json:"fields"`
}

// providersResponse is the body of a providers response.
type providersResponse struct {
	Providers []apiProvider `json:"providers"`
}

// docFilenames maps a provider name to the file name of its documentation
// page, for the providers whose documentation file is not named after the
// provider. Providers absent from this map use `docs/<name>.md`.
var docFilenames = map[string]string{ //nolint:gochecknoglobals
	"ddnss":       "ddnss.de",
	"dyn":         "dyndns",
	"he":          "he.net",
	"name.com":    "name.com",
	"selfhost.de": "selfhost.de",
}

// curatedProviderFields describes the provider specific settings entry keys of
// the most commonly used providers. Every key, label, type and required flag
// below was read from the `extraSettings` struct and the `validateSettings`
// function of the matching `internal/provider/providers/<name>/provider.go`
// file. A provider absent from this map is still returned by the API with an
// empty field list.
var curatedProviderFields = map[string][]apiField{ //nolint:gochecknoglobals
	"aliyun": {
		{Key: "access_key_id", Label: "Access Key ID", Type: fieldTypeText, Required: true,
			Help: "Aliyun access key ID", Placeholder: "LTAI5t..."},
		{Key: "access_secret", Label: "Access Secret", Type: fieldTypePassword, Required: true,
			Help: "Aliyun access key secret"},
		{Key: "region", Label: "Region", Type: fieldTypeText,
			Help: "Aliyun region, for example cn-hangzhou", Placeholder: "cn-hangzhou"},
	},
	"allinkl": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "all-inkl.com account username or sub account"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the all-inkl.com account"},
	},
	"bunny": {
		{Key: "api_key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "bunny.net API key", Placeholder: "..."},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, between 60 and 3600", Placeholder: "3600"},
	},
	"changeip": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "changeip.com account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the changeip.com account"},
	},
	"cloudflare": {
		{Key: "zone_identifier", Label: "Zone Identifier", Type: fieldTypeText, Required: true,
			Help: "Zone ID of the zone to update, as shown in the Cloudflare dashboard"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber, Required: true,
			Help: "Record TTL in seconds", Placeholder: "600"},
		{Key: "token", Label: "API Token", Type: fieldTypePassword,
			Help: "Scoped API token with Zone:DNS:Edit permission", Placeholder: "cf_..."},
		{Key: "user_service_key", Label: "User Service Key", Type: fieldTypePassword,
			Help: "Alternative to the API token, of the form v1.0-<user>-<key>"},
		{Key: "email", Label: "Email", Type: fieldTypeText,
			Help: "Account email, required together with the global API key"},
		{Key: "key", Label: "Global API Key", Type: fieldTypePassword,
			Help: "Global API key, required together with the account email"},
		{Key: "proxied", Label: "Proxied", Type: fieldTypeCheckbox,
			Help: "Route the record through the Cloudflare proxy"},
	},
	"dd24": {
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "DD24 dynamic DNS API password"},
	},
	"ddnss": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "ddnss.de account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the ddnss.de account"},
		{Key: "dual_stack", Label: "Dual Stack", Type: fieldTypeCheckbox,
			Help: "Let ddnss.de manage both the IPv4 and IPv6 records"},
	},
	"desec": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "deSEC.io API token", Placeholder: "..."},
	},
	"digitalocean": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "DigitalOcean API token with domain read and write scope"},
	},
	"dnsomatic": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "DNS-O-Matic account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the DNS-O-Matic account"},
	},
	"dnspod": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "DNSPod API token created from the DNSPod console"},
	},
	"domeneshop": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "Domeneshop API token"},
		{Key: "secret", Label: "Secret", Type: fieldTypePassword, Required: true,
			Help: "Domeneshop API secret"},
	},
	"dondominio": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "DonDominio account username"},
		{Key: "key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "DonDominio API key"},
		{Key: "password", Label: "Password", Type: fieldTypePassword,
			Help: "Retro-compatible password instead of the API key"},
	},
	"dreamhost": {
		{Key: "key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "Dreamhost API key"},
	},
	"duckdns": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help:        "DuckDNS token, a UUID shown in the duckdns.org account page",
			Placeholder: "00000000-0000-0000-0000-000000000000"},
	},
	"dyn": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "Dyn account username"},
		{Key: "client_key", Label: "Client Key", Type: fieldTypePassword, Required: true,
			Help: "Dyn client key, used instead of the password"},
		{Key: "password", Label: "Password", Type: fieldTypePassword,
			Help: "Retro-compatible password instead of the client key"},
	},
	"dynu": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "DynU account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the DynU account"},
		{Key: "group", Label: "Group", Type: fieldTypeText,
			Help: "Optional DynU group name", Placeholder: "default"},
	},
	"dynv6": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "dynv6 API token"},
	},
	"easydns": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "EasyDNS account username"},
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "EasyDNS API token"},
	},
	"freedns": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "FreeDNS token, found under the dynamic DNS account page"},
	},
	"gandi": {
		{Key: "key", Label: "API Key", Type: fieldTypePassword,
			Help: "Gandi LiveDNS API key"},
		{Key: "personal_access_token", Label: "Personal Access Token", Type: fieldTypePassword,
			Help: "Gandi personal access token, an alternative to the API key"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, defaults to the Gandi minimum of 300"},
	},
	"gcp": {
		{Key: "project", Label: "Project", Type: fieldTypeText, Required: true,
			Help: "Google Cloud project ID"},
		{Key: "zone", Label: "Zone", Type: fieldTypeText, Required: true,
			Help: "Name of the managed zone to update"},
		{Key: "credentials", Label: "Credentials", Type: fieldTypeTextarea, Required: true,
			Help:        "Service account credentials JSON, with a type field",
			Placeholder: "{\"type\":\"service_account\",...}"},
	},
	"gigahostno": {
		{Key: "apikey", Label: "API Key", Type: fieldTypePassword,
			Help: "Gigahost.no API key, an alternative to the email and password"},
		{Key: "email", Label: "Email", Type: fieldTypeText,
			Help: "Account email, required with the password when there is no API key"},
		{Key: "password", Label: "Password", Type: fieldTypePassword,
			Help: "Account password, required with the email when there is no API key"},
	},
	"godaddy": {
		{Key: "key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "GoDaddy API key"},
		{Key: "secret", Label: "API Secret", Type: fieldTypePassword, Required: true,
			Help: "GoDaddy API secret"},
	},
	"goip": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "goip.de account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the goip.de account"},
	},
	"he": {
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Hurricane Electric dynamic DNS password"},
	},
	"hetzner": {
		{Key: "zone_identifier", Label: "Zone Identifier", Type: fieldTypeText, Required: true,
			Help: "Zone ID of the legacy Hetzner DNS zone"},
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "Legacy Hetzner DNS API token"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds"},
	},
	"hetznercloud": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help:        "Hetzner Cloud API token, at least 60 seconds TTL is enforced",
			Placeholder: "..."},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, at least 60"},
	},
	"hostinger": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "Hostinger API token"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, between 60 and 3600"},
	},
	"infomaniak": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "Infomaniak account email or username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the Infomaniak account"},
	},
	"inwx": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "INWX account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "INWX account password or API password"},
	},
	"ionos": {
		{Key: "api_key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "IONOS API key"},
	},
	"ipv64": {
		{Key: "key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "ipv64.de API key"},
	},
	"linode": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "Linode API token with domain read and write scope"},
	},
	"loopia": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "Loopia account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the Loopia account"},
	},
	"luadns": {
		{Key: "email", Label: "Email", Type: fieldTypeText, Required: true,
			Help: "LuaDNS account email"},
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "LuaDNS API token"},
	},
	"myaddr": {
		{Key: "key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "myaddr.tools API key"},
	},
	"name.com": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "name.com account username"},
		{Key: "token", Label: "API Token", Type: fieldTypePassword, Required: true,
			Help: "name.com API token, at least a 300 second TTL is enforced"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, at least 300"},
	},
	"namecheap": {
		{Key: "password", Label: "API Password", Type: fieldTypePassword, Required: true,
			Help: "Namecheap API access password, IPv6 only, IPv4 needs a script"},
	},
	"namesilo": {
		{Key: "key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "NameSilo API key"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, between 3600 and 2592001"},
	},
	"netcup": {
		{Key: "customer_number", Label: "Customer Number", Type: fieldTypeText, Required: true,
			Help: "Netcup customer number"},
		{Key: "api_key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "Netcup API key"},
		{Key: "password", Label: "API Password", Type: fieldTypePassword, Required: true,
			Help: "Netcup API password"},
	},
	"njalla": {
		{Key: "key", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "Njalla API token"},
	},
	"noip": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "No-IP account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the No-IP account"},
	},
	"nowdns": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "NowDNS account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the NowDNS account"},
	},
	"opendns": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "OpenDNS account username"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the OpenDNS account"},
	},
	"ovh": {
		{Key: "mode", Label: "Authentication Mode", Type: fieldTypeText,
			Help:        `Set to "api" to use the application credentials, or leave empty for the web credentials`,
			Placeholder: "api"},
		{Key: "username", Label: "Username", Type: fieldTypeText,
			Help: "OVH account username, required when the mode is not api"},
		{Key: "password", Label: "Password", Type: fieldTypePassword,
			Help: "OVH account password, required when the mode is not api"},
		{Key: "api_endpoint", Label: "API Endpoint", Type: fieldTypeText,
			Help: "OVH API endpoint, for example ovh-eu", Placeholder: "ovh-eu"},
		{Key: "app_key", Label: "Application Key", Type: fieldTypePassword,
			Help: "Required when the mode is api"},
		{Key: "app_secret", Label: "Application Secret", Type: fieldTypePassword,
			Help: "Required when the mode is api"},
		{Key: "consumer_key", Label: "Consumer Key", Type: fieldTypePassword,
			Help: "Required when the mode is api"},
	},
	"porkbun": {
		{Key: "api_key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "Porkbun API key, the PBPK- prefixed key"},
		{Key: "secret_api_key", Label: "Secret API Key", Type: fieldTypePassword, Required: true,
			Help: "Porkbun secret API key, the skpb- prefixed key"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds"},
	},
	"route53": {
		{Key: "access_key", Label: "Access Key", Type: fieldTypeText, Required: true,
			Help: "AWS access key ID"},
		{Key: "secret_key", Label: "Secret Key", Type: fieldTypePassword, Required: true,
			Help: "AWS secret access key"},
		{Key: "zone_id", Label: "Zone ID", Type: fieldTypeText, Required: true,
			Help: "Route53 hosted zone ID"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, must be a multiple of 60"},
	},
	"scaleway": {
		{Key: "secret_key", Label: "Secret Key", Type: fieldTypePassword, Required: true,
			Help: "Scaleway secret key"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds"},
	},
	"servercow": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "Servercow email address"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the Servercow account"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds"},
	},
	"spaceship": {
		{Key: "api_key", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "Spaceship API key"},
		{Key: "api_secret", Label: "API Secret", Type: fieldTypePassword, Required: true,
			Help: "Spaceship API secret"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, between 60 and 3600"},
	},
	"spdyn": {
		{Key: "token", Label: "Token", Type: fieldTypePassword,
			Help: "Secure DNS token, an alternative to the user and password"},
		{Key: "user", Label: "User", Type: fieldTypeText,
			Help: "Spdyn account user name, required without a token"},
		{Key: "password", Label: "Password", Type: fieldTypePassword,
			Help: "Spdyn account password, required without a token"},
	},
	"strato": {
		{Key: "password", Label: "Dynamic DNS Password", Type: fieldTypePassword, Required: true,
			Help: "STRATO dynamic DNS password"},
	},
	"variomedia": {
		{Key: "email", Label: "Email", Type: fieldTypeText, Required: true,
			Help: "Vario Media account email"},
		{Key: "password", Label: "Password", Type: fieldTypePassword, Required: true,
			Help: "Password of the Vario Media account"},
	},
	"vercel": {
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "Vercel API token"},
		{Key: "team_id", Label: "Team ID", Type: fieldTypeText,
			Help: "Vercel team to update the records of"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds"},
	},
	"vultr": {
		{Key: "apikey", Label: "API Key", Type: fieldTypePassword, Required: true,
			Help: "Vultr API key"},
		{Key: "ttl", Label: "TTL", Type: fieldTypeNumber,
			Help: "Record TTL in seconds, between 60 and 3600"},
	},
	"zoneedit": {
		{Key: "username", Label: "Username", Type: fieldTypeText, Required: true,
			Help: "ZoneEdit account username"},
		{Key: "token", Label: "Token", Type: fieldTypePassword, Required: true,
			Help: "ZoneEdit API token"},
	},
}

// apiProviders returns every provider the updater supports, sorted by name,
// together with the settings entry fields it accepts.
func (h *handlers) apiProviders(w http.ResponseWriter, _ *http.Request) {
	choices := constants.ProviderChoices()
	providers := make([]apiProvider, 0, len(choices))
	for _, choice := range choices {
		name := string(choice)
		fields, ok := curatedProviderFields[name]
		if !ok {
			fields = []apiField{}
		} else {
			fields = append([]apiField(nil), fields...) // copy to keep the map immutable.
		}
		providers = append(providers, apiProvider{
			Name:   name,
			Label:  providerLabel(name),
			Doc:    fmtDocFilepath(name),
			Fields: fields,
		})
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})

	writeJSON(w, http.StatusOK, providersResponse{Providers: providers})
}

// fmtDocFilepath returns the repository relative path of the documentation
// page of the provider name given.
func fmtDocFilepath(name string) string {
	filename, ok := docFilenames[name]
	if !ok {
		filename = name
	}
	return fmt.Sprintf(docFilepathFormat, filename)
}

// providerLabel returns the provider name given in title case, splitting it
// on the underscores, dots and dashes it may contain.
func providerLabel(name string) string {
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '.' || r == '-'
	})
	for i, field := range fields {
		runes := []rune(field)
		runes[0] = unicode.ToUpper(runes[0])
		fields[i] = string(runes)
	}
	return strings.Join(fields, " ")
}
