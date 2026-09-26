package params

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_extractFromDomainField(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		domainField      string
		domainRegistered string
		owners           []string
		errWrapped       error
		errMessage       string
	}{
		"root_domain": {
			domainField:      "example.com",
			domainRegistered: "example.com",
			owners:           []string{"@"},
		},
		"subdomain": {
			domainField:      "abc.example.com",
			domainRegistered: "example.com",
			owners:           []string{"abc"},
		},
		"two_dots_tld": {
			domainField:      "abc.example.co.uk",
			domainRegistered: "example.co.uk",
			owners:           []string{"abc"},
		},
		"wildcard": {
			domainField:      "*.example.com",
			domainRegistered: "example.com",
			owners:           []string{"*"},
		},
		"multiple": {
			domainField:      "*.example.com,example.com",
			domainRegistered: "example.com",
			owners:           []string{"*", "@"},
		},
		"different_domains": {
			domainField: "*.example.com,abc.something.com",
			errWrapped:  ErrMultipleDomainsSpecified,
			errMessage:  "multiple domains specified: \"example.com\" and \"something.com\"",
		},
		"goip.de": {
			domainField:      "my.domain.goip.de",
			domainRegistered: "domain.goip.de",
			owners:           []string{"my"},
		},
		"duckdns.org": {
			domainField:      "my.domain.duckdns.org",
			domainRegistered: "domain.duckdns.org",
			owners:           []string{"my"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			domainRegistered, owners, err := extractFromDomainField(testCase.domainField)

			assert.ErrorIs(t, err, testCase.errWrapped)
			if testCase.errWrapped != nil {
				assert.EqualError(t, err, testCase.errMessage)
			}
			assert.Equal(t, testCase.domainRegistered, domainRegistered)
			assert.Equal(t, testCase.owners, owners)
		})
	}
}

func Test_envSeedOf(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		value string
		seed  string
	}{
		"empty": {
			value: "",
			seed:  "",
		},
		"empty_json_object": {
			value: "{}",
			// echo -n "{}" | sha256sum
			seed: "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			seed := envSeedOf(testCase.value)

			assert.Equal(t, testCase.seed, seed)
			assert.Equal(t, testCase.seed, EnvSeedOf(testCase.value))
		})
	}

	t.Run("different_values_give_different_seeds", func(t *testing.T) {
		t.Parallel()

		seedA := EnvSeedOf(`{"settings":[]}`)
		seedB := EnvSeedOf(`{"settings":[{}]}`)

		assert.Len(t, seedA, 64)
		assert.NotEqual(t, seedA, seedB)
	})
}

func Test_resolveRawSettingsFrom(t *testing.T) {
	t.Parallel()

	const envValue = `{"settings":[` + entryCloudflare + `]}`
	envSeed := EnvSeedOf(envValue)

	testCases := map[string]struct {
		envValue     string
		fileMissing  bool
		fileContent  string
		fromEnv      bool
		settings     []string
		seed         string
		fileSettings []string
		fileSeed     string
		fileExact    string
	}{
		"env_set_and_no_file": {
			envValue:     envValue,
			fileMissing:  true,
			fromEnv:      true,
			settings:     []string{entryCloudflare},
			seed:         envSeed,
			fileSettings: []string{entryCloudflare},
			fileSeed:     envSeed,
		},
		"env_set_and_matching_seed_file_wins": {
			envValue: envValue,
			fileContent: `{"_env_seed":"` + envSeed + `","settings":[` +
				entryCloudflare + `,` + entryDuckDNS + `]}`,
			fromEnv:  false,
			settings: []string{entryCloudflare, entryDuckDNS},
			seed:     envSeed,
			// the file content, including the settings added through the web UI,
			// must be used as is and must not be overwritten.
			fileExact: `{"_env_seed":"` + envSeed + `","settings":[` +
				entryCloudflare + `,` + entryDuckDNS + `]}`,
		},
		"env_set_and_different_seed_env_wins": {
			envValue:     envValue,
			fileContent:  `{"_env_seed":"deadbeef","settings":[` + entryDuckDNS + `]}`,
			fromEnv:      true,
			settings:     []string{entryCloudflare},
			seed:         envSeed,
			fileSettings: []string{entryCloudflare},
			fileSeed:     envSeed,
		},
		"env_set_and_legacy_file_without_seed_env_wins": {
			envValue:     envValue,
			fileContent:  `{"settings":[` + entryDuckDNS + `]}`,
			fromEnv:      true,
			settings:     []string{entryCloudflare},
			seed:         envSeed,
			fileSettings: []string{entryCloudflare},
			fileSeed:     envSeed,
		},
		"env_set_and_malformed_file_env_wins": {
			envValue:     envValue,
			fileContent:  `{"settings":`,
			fromEnv:      true,
			settings:     []string{entryCloudflare},
			seed:         envSeed,
			fileSettings: []string{entryCloudflare},
			fileSeed:     envSeed,
		},
		"env_unset_and_file_with_ui_edits": {
			envValue:     "",
			fileContent:  `{"settings":[` + entryCloudflare + `,` + entryDuckDNS + `]}`,
			fromEnv:      false,
			settings:     []string{entryCloudflare, entryDuckDNS},
			fileSettings: []string{entryCloudflare, entryDuckDNS},
			fileExact:    `{"settings":[` + entryCloudflare + `,` + entryDuckDNS + `]}`,
		},
		"env_unset_and_no_file": {
			envValue:    "",
			fileMissing: true,
			fromEnv:     false,
			settings:    []string{},
			fileExact:   emptyDocument,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reader := NewReader(&testLogger{})
			var filePath string
			if testCase.fileMissing {
				filePath = filepath.Join(t.TempDir(), "config.json")
			} else {
				store := newTestStoreWithFile(t, testCase.fileContent)
				filePath = store.Filepath()
			}

			rawBytes, fromEnv, err := reader.resolveRawSettingsFrom(filePath, testCase.envValue)

			require.NoError(t, err)
			assert.Equal(t, testCase.fromEnv, fromEnv)
			assert.Equal(t, testCase.settings, settingsOf(t, rawBytes))
			doc := document{}
			require.NoError(t, json.Unmarshal(rawBytes, &doc))
			assert.Equal(t, testCase.seed, doc.EnvSeed)

			fileData, err := os.ReadFile(filePath) //nolint:gosec // test file path.
			require.NoError(t, err, "the settings file should exist")
			switch {
			case testCase.fileExact != "":
				assert.Equal(t, testCase.fileExact, string(fileData),
					"the settings file should not be modified")
			default:
				assert.Equal(t, testCase.fileSettings, settingsOf(t, fileData))
				fileDoc := document{}
				require.NoError(t, json.Unmarshal(fileData, &fileDoc))
				assert.Equal(t, testCase.fileSeed, fileDoc.EnvSeed)
				assert.Equal(t, rawBytes, fileData,
					"the file written should match the returned settings")
			}
		})
	}
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel.
func Test_resolveRawSettings(t *testing.T) {
	t.Run("env_set_and_no_file", func(t *testing.T) {
		t.Setenv(configEnvVar, `{"settings":[`+entryCloudflare+`]}`)

		reader := NewReader(&testLogger{})
		filePath := filepath.Join(t.TempDir(), "config.json")

		rawBytes, fromEnv, err := reader.resolveRawSettings(filePath)

		require.NoError(t, err)
		assert.True(t, fromEnv)
		assert.Equal(t, []string{entryCloudflare}, settingsOf(t, rawBytes))
		fileDoc := document{}
		require.NoError(t, json.Unmarshal(mustReadFile(t, filePath), &fileDoc))
		assert.Equal(t, EnvSeedOf(os.Getenv(configEnvVar)), fileDoc.EnvSeed)
	})

	t.Run("env_unset_and_no_file", func(t *testing.T) {
		t.Setenv(configEnvVar, "")

		reader := NewReader(&testLogger{})
		filePath := filepath.Join(t.TempDir(), "config.json")

		rawBytes, fromEnv, err := reader.resolveRawSettings(filePath)

		require.NoError(t, err)
		assert.False(t, fromEnv)
		assert.Equal(t, emptyDocument, string(rawBytes))
		assert.Equal(t, emptyDocument, string(mustReadFile(t, filePath)))
	})
}

//nolint:paralleltest // t.Setenv is incompatible with t.Parallel.
func Test_JSONProviders(t *testing.T) {
	t.Run("env_set_and_no_file", func(t *testing.T) {
		t.Setenv(configEnvVar, `{"settings":[`+entryCloudflare+`]}`)

		reader := NewReader(&testLogger{})
		filePath := filepath.Join(t.TempDir(), "config.json")

		providers, warnings, err := reader.JSONProviders(filePath)

		require.NoError(t, err)
		assert.Empty(t, warnings)
		assert.Len(t, providers, 1)
		assert.FileExists(t, filePath)
	})

	t.Run("env_set_and_matching_seed_keeps_ui_edits", func(t *testing.T) {
		envValue := `{"settings":[` + entryCloudflare + `]}`
		t.Setenv(configEnvVar, envValue)
		// settings added through the web UI after the last synchronization.
		fileContent := `{"_env_seed":"` + EnvSeedOf(envValue) + `","settings":[` +
			entryCloudflare + `,` + entryDuckDNS + `]}`

		reader := NewReader(&testLogger{})
		store := newTestStoreWithFile(t, fileContent)

		providers, _, err := reader.JSONProviders(store.Filepath())

		require.NoError(t, err)
		assert.Len(t, providers, 2)
		assert.Equal(t, fileContent, string(mustReadFile(t, store.Filepath())))
	})

	t.Run("env_set_and_different_seed_env_wins", func(t *testing.T) {
		t.Setenv(configEnvVar, `{"settings":[`+entryCloudflare+`]}`)

		reader := NewReader(&testLogger{})
		store := newTestStoreWithFile(t,
			`{"_env_seed":"deadbeef","settings":[`+entryCloudflare+`,`+entryDuckDNS+`]}`)

		providers, _, err := reader.JSONProviders(store.Filepath())

		require.NoError(t, err)
		assert.Len(t, providers, 1)
		assert.Equal(t, []string{entryCloudflare},
			settingsOf(t, mustReadFile(t, store.Filepath())))
	})

	t.Run("env_unset_uses_file", func(t *testing.T) {
		t.Setenv(configEnvVar, "")

		reader := NewReader(&testLogger{})
		store := newTestStoreWithFile(t, `{"settings":[`+entryCloudflare+`]}`)

		providers, _, err := reader.JSONProviders(store.Filepath())

		require.NoError(t, err)
		assert.Len(t, providers, 1)
		assert.Equal(t, `{"settings":[`+entryCloudflare+`]}`,
			string(mustReadFile(t, store.Filepath())))
	})

	t.Run("env_set_and_malformed_env_value", func(t *testing.T) {
		t.Setenv(configEnvVar, `not json`)

		reader := NewReader(&testLogger{})
		filePath := filepath.Join(t.TempDir(), "config.json")

		providers, _, err := reader.JSONProviders(filePath)

		assert.ErrorIs(t, err, errUnmarshalCommon)
		assert.ErrorContains(t, err, "configuration given: ")
		assert.Empty(t, providers)
		assert.NoFileExists(t, filePath)
	})
}

func Test_JSONProviders_readError(t *testing.T) {
	t.Parallel()

	reader := NewReader(&testLogger{})
	reader.readFile = func(string) ([]byte, error) {
		return nil, errUnmarshalCommon
	}

	providers, warnings, err := reader.JSONProviders("/does/not/matter")

	assert.ErrorIs(t, err, errUnmarshalCommon)
	assert.Empty(t, providers)
	assert.Empty(t, warnings)
}
