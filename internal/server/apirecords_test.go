package server

import (
	"net/http"
	"net/netip"
	"testing"
	"time"

	appconstants "github.com/qdm12/ddns-updater/internal/constants"
	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/internal/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_apiRecords(t *testing.T) {
	t.Parallel()

	t.Run("no record", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(t, &fakeDB{}, nil, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodGet, "/api/records", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"records":[]`)
		data := decodeJSONResponse(t, recorder)
		assert.Equal(t, []any{}, data["records"])
		assert.Equal(t, float64(0), data["count"])
	})

	t.Run("mapped fields", func(t *testing.T) {
		t.Parallel()

		record := makeRecords(1)[0]
		record.Status = appconstants.SUCCESS
		record.Message = "changed to 1.2.3.4"
		record.Time = time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
		record.History = models.History{
			{IP: netip.MustParseAddr("9.9.9.9"), Time: time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC)},
			{IP: netip.MustParseAddr("1.2.3.4"), Time: time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)},
		}
		handler := newTestHandler(t, &fakeDB{records: []records.Record{record}}, nil, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodGet, "/api/records", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		data := decodeJSONResponse(t, recorder)
		assert.Equal(t, float64(1), data["count"])

		all, ok := data["records"].([]any)
		require.True(t, ok)
		require.Len(t, all, 1)
		first, ok := all[0].(map[string]any)
		require.True(t, ok)

		assert.Equal(t, "sub0.example.com", first["domain"])
		assert.Equal(t, "sub0", first["owner"])
		assert.Equal(t, "cloudflare", first["provider"])
		assert.Equal(t, "ipv4", first["ipVersion"])
		assert.Equal(t, "success", first["status"])
		assert.Equal(t, "changed to 1.2.3.4", first["message"])
		assert.Equal(t, "1.2.3.4", first["currentIP"])
		assert.Equal(t, []any{"9.9.9.9"}, first["previousIPs"])
		assert.Equal(t, "2026-01-02T15:04:05Z", first["lastUpdate"])
		assert.Nil(t, first["lastBan"])
	})

	t.Run("last ban and no history", func(t *testing.T) {
		t.Parallel()

		record := makeRecords(1)[0]
		record.Status = appconstants.FAIL
		record.History = nil
		ban := time.Date(2026, time.March, 4, 1, 2, 3, 0, time.UTC)
		record.LastBan = &ban
		handler := newTestHandler(t, &fakeDB{records: []records.Record{record}}, nil, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodGet, "/api/records", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		data := decodeJSONResponse(t, recorder)
		all, ok := data["records"].([]any)
		require.True(t, ok)
		require.Len(t, all, 1)
		first, ok := all[0].(map[string]any)
		require.True(t, ok)

		assert.Equal(t, "failure", first["status"])
		assert.Equal(t, "", first["currentIP"])
		assert.Equal(t, []any{}, first["previousIPs"])
		assert.Nil(t, first["lastUpdate"])
		assert.Equal(t, "2026-03-04T01:02:03Z", first["lastBan"])
	})

	t.Run("record without provider", func(t *testing.T) {
		t.Parallel()

		handler := newTestHandler(t,
			&fakeDB{records: []records.Record{{Status: appconstants.UNSET}}}, nil, &fakeRunner{})

		recorder := doTestRequest(t, handler, http.MethodGet, "/api/records", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		data := decodeJSONResponse(t, recorder)
		all, ok := data["records"].([]any)
		require.True(t, ok)
		require.Len(t, all, 1)
		first, ok := all[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "", first["domain"])
		assert.Equal(t, "unset", first["status"])
	})
}

func Test_providerNameOf(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		providerString string
		expected       string
	}{
		"cloudflare": {
			providerString: "[domain: example.com | owner: sub | provider: cloudflare | ip: ipv4]",
			expected:       "cloudflare",
		},
		"underscore name": {
			providerString: "[domain: h.net | owner: dyn | provider: hetznercloud | ip: ipv6]",
			expected:       "hetznercloud",
		},
		"unrecognized": {
			providerString: "something else",
			expected:       "",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, providerNameOf(testCase.providerString))
		})
	}
}
