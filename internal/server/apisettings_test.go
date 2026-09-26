package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qdm12/ddns-updater/internal/manager"
	"github.com/qdm12/ddns-updater/internal/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	entryCloudflare = `{"provider":"cloudflare","token":"t","zone_identifier":"z",` +
		`"ttl":600,"domain":"sub.example.com","ip_version":"ipv4"}`
	entryDuckDNS = `{"provider":"duckdns","token":"00000000-0000-0000-0000-000000000000",` +
		`"domain":"name.duckdns.org"}`
)

func Test_apiGetSettings(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		entries      []json.RawMessage
		warnings     []string
		entriesErr   error
		recordCount  int
		expectedCode int
	}{
		"success": {
			entries:      []json.RawMessage{json.RawMessage(entryCloudflare)},
			recordCount:  3,
			expectedCode: http.StatusOK,
		},
		"empty settings is an empty array": {
			recordCount:  0,
			expectedCode: http.StatusOK,
		},
		"nil entries and warnings marshal as arrays": {
			entries:      nil,
			warnings:     nil,
			expectedCode: http.StatusOK,
		},
		"read failure": {
			entriesErr:   errors.New("cannot read"),
			expectedCode: http.StatusInternalServerError,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			settings := newFakeSettings(t)
			settings.entries = testCase.entries
			settings.entriesWarning = testCase.warnings
			settings.entriesErr = testCase.entriesErr
			db := &fakeDB{records: makeRecords(testCase.recordCount)}
			handler := newTestHandler(t, db, settings, &fakeRunner{})

			recorder := doTestRequest(t, handler, http.MethodGet, "/api/settings", "")

			assert.Equal(t, testCase.expectedCode, recorder.Code)
			if testCase.expectedCode != http.StatusOK {
				return
			}

			body := recorder.Body.String()
			assert.Contains(t, body, `"settings":[`)
			assert.NotContains(t, body, `"settings":null`)
			assert.NotContains(t, body, `"warnings":null`)

			data := decodeJSONResponse(t, recorder)
			assert.Equal(t, float64(testCase.recordCount), data["records"])
			assert.Equal(t, "unset", data["env"])
			assert.Equal(t, settings.filepath, data["filePath"])
			assert.IsType(t, []any{}, data["settings"])
			assert.IsType(t, []any{}, data["warnings"])

			entriesValue, ok := data["settings"].([]any)
			require.True(t, ok)
			assert.Len(t, entriesValue, len(testCase.entries))
			if len(testCase.entries) > 0 {
				firstEntry, ok := entriesValue[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "cloudflare", firstEntry["provider"])
				assert.Equal(t, "sub.example.com", firstEntry["domain"])
			}
		})
	}
}

func Test_apiAddSetting(t *testing.T) {
	t.Parallel()

	t.Run("valid entry", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		settings.result = manager.Result{
			Settings: []json.RawMessage{json.RawMessage(entryDuckDNS)},
			Source:   "file",
			Env:      "unset",
			FilePath: "/updater/data/config.json",
			Records:  1,
		}
		handler := newTestHandler(t, &fakeDB{records: makeRecords(1)}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPost, "/api/settings", entryDuckDNS)

		assert.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, 1, settings.AddCalls())
		assert.JSONEq(t, entryDuckDNS, string(settings.AddEntryReceived()))

		data := decodeJSONResponse(t, recorder)
		assert.Equal(t, "file", data["source"])
		assert.Equal(t, float64(1), data["records"])

		entriesValue, ok := data["settings"].([]any)
		require.True(t, ok)
		require.Len(t, entriesValue, 1)
		entry, ok := entriesValue[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "duckdns", entry["provider"])
	})

	errorCases := map[string]struct {
		err          error
		expectedCode int
	}{
		"validation error": {
			err:          &manager.ValidationError{Err: params.ErrEntryNoProvider},
			expectedCode: http.StatusUnprocessableEntity,
		},
		"entry not found": {
			err:          fmt.Errorf("%w: index 5", params.ErrEntryNotFound),
			expectedCode: http.StatusNotFound,
		},
		"generic error": {
			err:          errors.New("something broke"),
			expectedCode: http.StatusInternalServerError,
		},
	}

	for name, testCase := range errorCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			settings := newFakeSettings(t)
			settings.addErr = testCase.err
			handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

			recorder := doTestRequest(t, handler, http.MethodPost, "/api/settings", entryDuckDNS)

			assert.Equal(t, testCase.expectedCode, recorder.Code)
			assert.Equal(t, "application/json", strings.SplitN(
				recorder.Header().Get("Content-Type"), ";", 2)[0])
			assert.Equal(t, []string{testCase.err.Error()}, decodeErrors(t, recorder))
		})
	}
}

func Test_apiReplaceSetting(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		settings.result = manager.Result{
			Settings: []json.RawMessage{json.RawMessage(entryCloudflare)},
			Records:  1,
		}
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPut,
			"/api/settings/2", entryCloudflare)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, settings.replaceCalls)
		assert.Equal(t, 2, settings.replaceIndex)
		assert.JSONEq(t, entryCloudflare, string(settings.replaceEntry))
	})

	badIndexes := map[string]string{
		"not a number":     "abc",
		"negative":         "-1",
		"out of int range": "99999999999999999999999",
		"floating point":   "1.5",
	}

	for name, index := range badIndexes {
		t.Run("bad index "+name, func(t *testing.T) {
			t.Parallel()

			settings := newFakeSettings(t)
			handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

			recorder := doTestRequest(t, handler, http.MethodPut,
				"/api/settings/"+index, entryCloudflare)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Equal(t, 0, settings.replaceCalls)
			require.Len(t, decodeErrors(t, recorder), 1)
		})
	}
}

func Test_apiDeleteSetting(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		settings.result = manager.Result{Records: 0}
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodDelete, "/api/settings/0", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, settings.deleteCalls)
		assert.Equal(t, 0, settings.deleteIndex)
		data := decodeJSONResponse(t, recorder)
		assert.Equal(t, []any{}, data["settings"])
	})

	t.Run("non numeric index", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodDelete, "/api/settings/xyz", "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Equal(t, 0, settings.deleteCalls)
		require.Len(t, decodeErrors(t, recorder), 1)
	})
}

func Test_apiReplaceAllSettings(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		settings.result = manager.Result{Records: 2}
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		body := `{"settings":[` + entryCloudflare + `,` + entryDuckDNS + `]}`
		recorder := doTestRequest(t, handler, http.MethodPut, "/api/settings", body)

		assert.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, 1, settings.replaceAllOps)
		require.Len(t, settings.replaceAll, 2)
		assert.JSONEq(t, entryDuckDNS, string(settings.replaceAll[1]))
	})

	t.Run("empty settings array", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPut, "/api/settings", `{"settings":[]}`)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, []json.RawMessage{}, settings.replaceAll)
	})

	t.Run("malformed body", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(t, nil, nil, nil)

		recorder := doTestRequest(t, handler, http.MethodPut, "/api/settings", `{"settings":`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Len(t, decodeErrors(t, recorder), 1)
	})

	t.Run("missing content type", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(t, nil, nil, nil)
		request := httptest.NewRequest(http.MethodPut, "/api/settings",
			strings.NewReader(`{"settings":[]}`))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusUnsupportedMediaType, recorder.Code)
		require.Len(t, decodeErrors(t, recorder), 1)
	})

	t.Run("wrong content type", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(t, nil, nil, nil)
		request := httptest.NewRequest(http.MethodPut, "/api/settings",
			strings.NewReader(`{"settings":[]}`))
		request.Header.Set("Content-Type", "text/plain")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusUnsupportedMediaType, recorder.Code)
	})

	t.Run("body too large", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(t, nil, nil, nil)
		oversized := `{"settings":[{"padding":"` + strings.Repeat("a", maxRequestBodyBytes) + `"}]}`
		request := httptest.NewRequest(http.MethodPut, "/api/settings",
			strings.NewReader(oversized))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
		require.Len(t, decodeErrors(t, recorder), 1)
	})

	t.Run("validation error", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		settings.replaceAllErr = &manager.ValidationError{Err: params.ErrEntryInvalidJSON}
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPut, "/api/settings",
			`{"settings":[`+entryCloudflare+`]}`)

		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	})
}

func Test_apiValidateSettings(t *testing.T) {
	t.Parallel()

	t.Run("valid wrapped settings", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		handler := newTestHandler(t, &fakeDB{records: makeRecords(2)}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPost, "/api/settings/validate",
			`{"settings":[`+entryCloudflare+`]}`)

		assert.Equal(t, http.StatusOK, recorder.Code)
		data := decodeJSONResponse(t, recorder)
		assert.Equal(t, true, data["valid"])
		assert.Equal(t, []any{}, data["errors"])
		assert.Equal(t, float64(2), data["recordCount"])
		require.Len(t, settings.replaceAll, 1)
	})

	t.Run("valid single object", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPost, "/api/settings/validate",
			entryCloudflare)

		assert.Equal(t, http.StatusOK, recorder.Code)
		data := decodeJSONResponse(t, recorder)
		assert.Equal(t, true, data["valid"])
		require.Len(t, settings.replaceAll, 1)
		assert.JSONEq(t, entryCloudflare, string(settings.replaceAll[0]))
	})

	t.Run("invalid single object", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		settings.validateErr = &manager.ValidationError{Err: params.ErrEntryNoProvider}
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPost, "/api/settings/validate",
			`{"domain":"example.com"}`)

		assert.Equal(t, http.StatusOK, recorder.Code)
		data := decodeJSONResponse(t, recorder)
		assert.Equal(t, false, data["valid"])
		assert.Equal(t, []any{params.ErrEntryNoProvider.Error()}, data["errors"])
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(t, nil, nil, nil)

		recorder := doTestRequest(t, handler, http.MethodPost, "/api/settings/validate", `{`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func Test_apiReload(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		handler := newTestHandler(t, &fakeDB{records: makeRecords(3)}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPost, "/api/reload", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, settings.reloadCalls)
		data := decodeJSONResponse(t, recorder)
		assert.Equal(t, true, data["reloaded"])
		assert.Equal(t, float64(3), data["records"])
		assert.Equal(t, []any{}, data["warnings"])
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		settings := newFakeSettings(t)
		settings.reloadErr = errors.New("cannot reload")
		handler := newTestHandler(t, &fakeDB{}, settings, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodPost, "/api/reload", "")

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}

func Test_settingsPage(t *testing.T) {
	t.Parallel()

	settings := newFakeSettings(t)
	handler := newTestHandler(t, &fakeDB{records: makeRecords(2)}, settings, &fakeRunner{})

	recorder := doTestRequest(t, handler, http.MethodGet, "/settings", "")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	body := recorder.Body.String()
	assert.Contains(t, body, `<div id="app"></div>`)
	assert.Contains(t, body, `window.__DDNS_BASE__`)
	assert.Contains(t, body, `static/styles.css`)
	assert.Contains(t, body, `data-record-count="2"`)
}

func Test_indexPage(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, &fakeDB{records: makeRecords(1)}, nil, &fakeRunner{})

	recorder := doTestRequest(t, handler, http.MethodGet, "/", "")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Body.String(), "example.com")
}

func Test_updatePage(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		runner := &fakeRunner{}
		handler := newTestHandler(t, &fakeDB{}, nil, runner)

		recorder := doTestRequest(t, handler, http.MethodGet, "/update", "")

		assert.Equal(t, http.StatusAccepted, recorder.Code)
		assert.Equal(t, 1, runner.Calls())
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		runner := &fakeRunner{errors: []error{errors.New("update failed")}}
		handler := newTestHandler(t, &fakeDB{}, nil, runner)

		recorder := doTestRequest(t, handler, http.MethodGet, "/update", "")

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	})
}

// decodeErrors decodes the `{"errors":[...]}` body of the response and fails
// the test if the body does not have that shape.
func decodeErrors(t *testing.T, recorder *httptest.ResponseRecorder) []string {
	t.Helper()

	data := decodeJSONResponse(t, recorder)
	raw, ok := data["errors"]
	require.True(t, ok, "body has no errors key: %s", recorder.Body.String())
	values, ok := raw.([]any)
	require.True(t, ok)

	messages := make([]string, len(values))
	for i, value := range values {
		messages[i], ok = value.(string)
		require.True(t, ok)
	}

	return messages
}

func Test_settingsSourceFor(t *testing.T) {
	t.Parallel()

	envValue := `{"settings":[{"provider":"duckdns","token":"t"}]}`

	testCases := map[string]struct {
		fileContent  *string
		envValue     string
		entriesCount int
		expected     string
	}{
		"missing file and no env": {
			expected: sourceEmpty,
		},
		"missing file with env set": {
			envValue: envValue,
			expected: sourceEnv,
		},
		"file with no entry": {
			fileContent: pointerTo(`{"settings":[]}`),
			expected:    sourceEmpty,
		},
		"file with entries": {
			fileContent:  pointerTo(`{"settings":[{"provider":"duckdns"}]}`),
			entriesCount: 1,
			expected:     sourceFile,
		},
		"file synced from an unchanged env": {
			fileContent: pointerTo(`{"settings":[],"_env_seed":"` +
				params.EnvSeedOf(envValue) + `"}`),
			envValue: envValue,
			expected: sourceEmpty,
		},
		"file synced from a changed env": {
			fileContent: pointerTo(`{"settings":[],"_env_seed":"stale"}`),
			envValue:    envValue,
			expected:    sourceEnv,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			filePath := filepath.Join(t.TempDir(), "config.json")
			if testCase.fileContent != nil {
				err := os.WriteFile(filePath, []byte(*testCase.fileContent), 0o600)
				require.NoError(t, err)
			}

			assert.Equal(t, testCase.expected,
				settingsSourceFor(filePath, testCase.envValue, testCase.entriesCount))
		})
	}
}

func Test_settingsEnvState(t *testing.T) {
	t.Parallel()

	assert.Equal(t, envUnset, settingsEnvState())
}

func pointerTo[T any](value T) *T { return &value }
