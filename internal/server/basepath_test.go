package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonEscape returns the escape json.Marshal produces for the rune given, for
// the three characters which could otherwise close a script element early.
func jsonEscape(r rune) string {
	return fmt.Sprintf(`\u%04x`, r)
}

// Test_scriptSafeJSON checks the base path is emitted as a JSON string literal
// which cannot close the script element it is embedded in.
func Test_scriptSafeJSON(t *testing.T) {
	t.Parallel()

	lt := jsonEscape('<')
	gt := jsonEscape('>')
	amp := jsonEscape('&')

	testCases := map[string]struct {
		value    string
		expected string
	}{
		"empty":      {value: "", expected: `""`},
		"plain path": {value: "/ddns", expected: `"/ddns"`},
		"script break out": {value: `/"></script><script>alert(1)</script>`,
			expected: `"/\"` + gt + lt + `/script` + gt + lt + `script` + gt + `alert(1)` + lt + `/script` + gt + `"`},
		"ampersand":           {value: "/a&b", expected: `"/a` + amp + `b"`},
		"quote and backslash": {value: `/a"b\c`, expected: `"/a\"b\\c"`},
		"angle brackets":      {value: "/<b>", expected: `"/` + lt + `b` + gt + `"`},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, scriptSafeJSON(testCase.value))
		})
	}
}

// Test_basePath_cannotBreakOutOfScriptElement checks a ROOT_URL holding markup
// cannot inject anything into the pages, since ROOT_URL is not validated.
func Test_basePath_cannotBreakOutOfScriptElement(t *testing.T) {
	t.Parallel()

	const maliciousRootURL = `/x"></script><script>alert(1)</script>`
	handler := newHandler(context.Background(), maliciousRootURL,
		&fakeDB{records: makeRecords(1)}, newFakeSettings(t), &fakeRunner{})

	for _, path := range []string{maliciousRootURL + "/", maliciousRootURL + "/settings"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		body := recorder.Body.String()
		assert.NotContains(t, body, "<script>alert(1)</script>")
		assert.NotContains(t, body, `</script`+`<script>`)
		assert.Equal(t, 1, strings.Count(body,
			`<script id="ddns-base" type="application/json">`))
	}
}

// Test_basePath_isAValidJSONLiteral checks the page JavaScript can still read
// the base path back with JSON.parse.
func Test_basePath_isAValidJSONLiteral(t *testing.T) {
	t.Parallel()

	for _, rootURL := range []string{"/", "/ddns", "/a&b", `/x"></script>`} {
		handler := newHandler(context.Background(), rootURL,
			&fakeDB{records: makeRecords(1)}, newFakeSettings(t), &fakeRunner{})

		mounted := strings.TrimSuffix(rootURL, "/")
		request := httptest.NewRequest(http.MethodGet, mounted+"/", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, mounted, basePathOf(t, recorder.Body.String()))
	}
}

// basePathOf extracts the base path published in the page and decodes it the
// same way the page JavaScript does.
func basePathOf(t *testing.T, body string) string {
	t.Helper()

	const prefix = `<script id="ddns-base" type="application/json">`
	start := strings.Index(body, prefix)
	require.NotEqual(t, -1, start, "no base path element in the page")
	rest := body[start+len(prefix):]
	end := strings.Index(rest, "</script>")
	require.NotEqual(t, -1, end, "unterminated base path element")

	var basePath string
	err := json.Unmarshal([]byte(rest[:end]), &basePath)
	require.NoError(t, err)

	return basePath
}
