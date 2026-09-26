package data

import (
	"net/netip"
	"testing"
	"time"

	"github.com/qdm12/ddns-updater/internal/constants"
	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/internal/records"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Update(t *testing.T) {
	t.Parallel()

	var (
		initialTime = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		updateTime  = time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
		initialIP   = netip.MustParseAddr("1.2.3.4")
		newIP       = netip.MustParseAddr("5.6.7.8")
	)

	// storedRecord builds a record already having a successful history.
	storedRecord := func(recordProvider *testProvider) records.Record {
		record := records.New(recordProvider, []models.HistoryEvent{
			{IP: initialIP, Time: initialTime},
		})
		record.Status = constants.SUCCESS
		record.Message = "changed to " + initialIP.String()
		record.Time = initialTime
		return record
	}

	testCases := map[string]struct {
		storedRecords   []records.Record
		id              uint
		incomingRecord  func(stored records.Record) records.Record
		errMessage      string
		expectedStored  func(stored records.Record) records.Record
		expectedNewIPs  []netip.Addr
		expectNoPersist bool
	}{
		"same identity stores the new IP": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			incomingRecord: func(stored records.Record) records.Record {
				updated := stored
				updated.Status = constants.UPDATING
				updated.History = append(updated.History, models.HistoryEvent{
					IP:   newIP,
					Time: updateTime,
				})
				return updated
			},
			expectedStored: func(stored records.Record) records.Record {
				updated := stored
				updated.Status = constants.UPDATING
				updated.History = append(updated.History, models.HistoryEvent{
					IP:   newIP,
					Time: updateTime,
				})
				return updated
			},
			expectedNewIPs: []netip.Addr{newIP},
		},
		"same identity without a new IP is not persisted": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			incomingRecord: func(stored records.Record) records.Record {
				updated := stored
				updated.Status = constants.UPDATING
				return updated
			},
			expectedStored: func(stored records.Record) records.Record {
				updated := stored
				updated.Status = constants.UPDATING
				return updated
			},
		},
		"different owner is rejected": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			incomingRecord: func(stored records.Record) records.Record {
				reconfigured := stored
				reconfigured.Provider = newTestProvider("example.com", "other")
				return reconfigured
			},
			errMessage: "record was changed by a configuration reload: for id 0",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
		"different domain is rejected": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			incomingRecord: func(stored records.Record) records.Record {
				reconfigured := stored
				reconfigured.Provider = newTestProvider("other.com", "sub")
				return reconfigured
			},
			errMessage: "record was changed by a configuration reload: for id 0",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
		"different provider is rejected": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			incomingRecord: func(stored records.Record) records.Record {
				reconfigured := stored
				reconfiguredProvider := newTestProvider("example.com", "sub")
				reconfiguredProvider.name = "cloudflare"
				reconfigured.Provider = reconfiguredProvider
				return reconfigured
			},
			errMessage: "record was changed by a configuration reload: for id 0",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
		"different IP version is rejected": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			incomingRecord: func(stored records.Record) records.Record {
				reconfigured := stored
				reconfiguredProvider := newTestProvider("example.com", "sub")
				reconfiguredProvider.ipVersion = ipversion.IP6
				reconfigured.Provider = reconfiguredProvider
				return reconfigured
			},
			errMessage: "record was changed by a configuration reload: for id 0",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
		"different proxied is rejected": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			incomingRecord: func(stored records.Record) records.Record {
				reconfigured := stored
				reconfiguredProvider := newTestProvider("example.com", "sub")
				reconfiguredProvider.proxied = true
				reconfigured.Provider = reconfiguredProvider
				return reconfigured
			},
			errMessage: "record was changed by a configuration reload: for id 0",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
		"different IPv6 suffix is rejected": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			incomingRecord: func(stored records.Record) records.Record {
				reconfigured := stored
				reconfiguredProvider := newTestProvider("example.com", "sub")
				reconfiguredProvider.ipv6Suffix = netip.MustParsePrefix("2001:db8::/64")
				reconfigured.Provider = reconfiguredProvider
				return reconfigured
			},
			errMessage: "record was changed by a configuration reload: for id 0",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
		"record shifted to another index is rejected": {
			storedRecords: []records.Record{
				storedRecord(newTestProvider("example.com", "a")),
				storedRecord(newTestProvider("example.com", "b")),
			},
			id: 0,
			incomingRecord: func(_ records.Record) records.Record {
				// Simulates a record that was deleted from the
				// configuration, shifting index 1 to index 0.
				return storedRecord(newTestProvider("example.com", "b"))
			},
			errMessage: "record was changed by a configuration reload: for id 0",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
		"id out of range is not found": {
			storedRecords: []records.Record{storedRecord(newTestProvider("example.com", "sub"))},
			id:            1,
			incomingRecord: func(stored records.Record) records.Record {
				return stored
			},
			errMessage: "record not found: for id 1",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
		"id out of range in an empty database is not found": {
			incomingRecord: func(stored records.Record) records.Record {
				return stored
			},
			errMessage: "record not found: for id 0",
			expectedStored: func(stored records.Record) records.Record {
				return stored
			},
			expectNoPersist: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			persistentDB := &testPersistentDatabase{}
			db := NewDatabase(testCase.storedRecords, persistentDB)

			var storedRecordUnderTest records.Record
			if len(testCase.storedRecords) > 0 {
				storedRecordUnderTest = cloneRecord(testCase.storedRecords[0])
			}

			err := db.Update(testCase.id, testCase.incomingRecord(cloneRecord(storedRecordUnderTest)))

			if testCase.errMessage == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, testCase.errMessage)
			}

			all := db.SelectAll()
			require.Len(t, all, len(testCase.storedRecords))
			if len(all) > 0 {
				assertStoredRecord(t,
					testCase.expectedStored(cloneRecord(storedRecordUnderTest)),
					all[0])
			}
			assert.Equal(t, testCase.expectedNewIPs, nilIfEmpty(persistentDB.newIPs()))
		})
	}
}

func Test_Update_rejects_a_stale_record_after_reload(t *testing.T) {
	t.Parallel()

	var (
		updateTime = time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
		newIP      = netip.MustParseAddr("5.6.7.8")
	)

	persistentDB := &testPersistentDatabase{}
	db := NewDatabase([]records.Record{
		records.New(newTestProvider("example.com", "a"), nil),
	}, persistentDB)

	// The update cycle selects the record.
	selected, err := db.Select(0)
	require.NoError(t, err)
	selected.Status = constants.SUCCESS
	selected.History = append(selected.History, models.HistoryEvent{
		IP:   newIP,
		Time: updateTime,
	})

	// A configuration reload swaps the record at index 0 for a different one.
	db.Reload([]records.Record{
		records.New(newTestProvider("example.com", "b"), nil),
	})

	err = db.Update(0, selected)
	require.ErrorIs(t, err, ErrRecordChanged)
	require.Error(t, err)

	// The freshly loaded record must be left completely untouched.
	stored, err := db.Select(0)
	require.NoError(t, err)
	assert.Equal(t, "example.com.b", stored.Provider.BuildDomainName())
	assert.Equal(t, constants.UNSET, stored.Status)
	assert.Empty(t, stored.Message)
	assert.Empty(t, stored.History)
	// No IP address may have been persisted for the wrong domain.
	assert.Empty(t, persistentDB.newIPs())
}

// assertStoredRecord checks the fields an update may alter.
func assertStoredRecord(t *testing.T, expected, actual records.Record) {
	t.Helper()

	assert.Equal(t, expected.Provider.BuildDomainName(), actual.Provider.BuildDomainName())
	assert.Equal(t, expected.Status, actual.Status)
	assert.Equal(t, expected.Message, actual.Message)
	assert.Equal(t, expected.History, actual.History)
	assert.Equal(t, expected.Time, actual.Time)
}

func nilIfEmpty(ips []netip.Addr) []netip.Addr {
	if len(ips) == 0 {
		return nil
	}
	return ips
}
