package server

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/qdm12/ddns-updater/internal/provider/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_apiProviders(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, nil, nil, &fakeRunner{})

	recorder := doTestRequest(t, handler, http.MethodGet, "/api/providers", "")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))

	data := decodeJSONResponse(t, recorder)
	all, ok := data["providers"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, all)

	byName := make(map[string]map[string]any, len(all))
	names := make([]string, len(all))
	for i, value := range all {
		provider, ok := value.(map[string]any)
		require.True(t, ok)
		name, ok := provider["name"].(string)
		require.True(t, ok)
		assert.NotEmpty(t, name)
		byName[name] = provider
		names[i] = name

		assert.NotEmpty(t, provider["label"])
		assert.NotEmpty(t, provider["doc"])
		fields, ok := provider["fields"].([]any)
		require.True(t, ok, "provider %s has no fields array", name)
		for _, rawField := range fields {
			field, ok := rawField.(map[string]any)
			require.True(t, ok)
			assert.NotEmpty(t, field["key"])
			assert.NotEmpty(t, field["label"])
			assert.NotEmpty(t, field["type"])
			assert.IsType(t, false, field["required"])
		}
	}

	assert.True(t, sort.StringsAreSorted(names), "providers are not sorted by name")

	for _, choice := range constants.ProviderChoices() {
		_, ok := byName[string(choice)]
		assert.True(t, ok, "provider %s is missing from the response", choice)
	}

	assert.GreaterOrEqual(t, len(curatedProviderFields), 25)
	assert.Equal(t, len(byName), len(constants.ProviderChoices()))
}

func Test_apiProvidersCuratedFields(t *testing.T) {
	t.Parallel()

	// These field keys and required flags were read from the extraSettings
	// struct and the validateSettings function of the matching provider
	// source file under internal/provider/providers.
	verified := map[string]struct {
		keys     []string
		required []string
	}{
		"cloudflare": {
			keys:     []string{"zone_identifier", "ttl", "token", "user_service_key", "email", "key", "proxied"},
			required: []string{"zone_identifier", "ttl"},
		},
		"duckdns": {
			keys:     []string{"token"},
			required: []string{"token"},
		},
		"godaddy": {
			keys:     []string{"key", "secret"},
			required: []string{"key", "secret"},
		},
		"noip": {
			keys:     []string{"username", "password"},
			required: []string{"username", "password"},
		},
		"ionos": {
			keys:     []string{"api_key"},
			required: []string{"api_key"},
		},
		"ovh": {
			keys:     []string{"username", "password", "mode", "api_endpoint", "app_key", "app_secret", "consumer_key"},
			required: nil,
		},
		"vultr": {
			keys:     []string{"apikey", "ttl"},
			required: []string{"apikey"},
		},
		"namecheap": {
			keys:     []string{"password"},
			required: []string{"password"},
		},
		"spaceship": {
			keys:     []string{"api_key", "api_secret", "ttl"},
			required: []string{"api_key", "api_secret"},
		},
		"porkbun": {
			keys:     []string{"api_key", "secret_api_key", "ttl"},
			required: []string{"api_key", "secret_api_key"},
		},
	}

	for name, testCase := range verified {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fields, ok := curatedProviderFields[name]
			require.True(t, ok, "provider %s is not in the curated map", name)

			keys := make([]string, len(fields))
			for i, field := range fields {
				keys[i] = field.Key
				required := slicesContains(testCase.required, field.Key)
				assert.Equal(t, required, field.Required,
					"field %s of %s has an unexpected required flag", field.Key, name)
			}
			assert.ElementsMatch(t, testCase.keys, keys)
		})
	}
}

func Test_apiProvidersDocFilesExist(t *testing.T) {
	t.Parallel()

	// The repository root is two directories up from this package.
	repoRoot := filepath.Join("..", "..")

	for name := range curatedProviderFields {
		doc := fmtDocFilepath(name)
		assert.True(t, filepath.IsAbs(doc) == false)
		_, err := os.Stat(filepath.Join(repoRoot, doc))
		assert.NoErrorf(t, err, "documentation %s of provider %s does not exist", doc, name)
	}
}

func Test_providerLabel(t *testing.T) {
	t.Parallel()

	testCases := map[string]string{
		"cloudflare":  "Cloudflare",
		"name.com":    "Name Com",
		"noip":        "Noip",
		"luadns":      "Luadns",
		"selfhost.de": "Selfhost De",
	}

	for name, expected := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, expected, providerLabel(name))
		})
	}
}

// slicesContains reports whether the slice contains the value given.
func slicesContains(slice []string, value string) bool {
	for _, element := range slice {
		if element == value {
			return true
		}
	}
	return false
}
