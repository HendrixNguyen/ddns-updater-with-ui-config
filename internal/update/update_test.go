package update

import (
	"context"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/qdm12/ddns-updater/internal/constants"
	"github.com/qdm12/ddns-updater/internal/data"
	librecords "github.com/qdm12/ddns-updater/internal/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDatabaseFailure = assert.AnError

func newTestUpdater(db Database, shoutrrrClient ShoutrrrClient, logger DebugLogger) *Updater {
	timeNow := func() time.Time {
		return time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
	}

	return NewUpdater(db, http.DefaultClient, shoutrrrClient, logger, timeNow, false)
}

func Test_Updater_Update(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		updateErrors      []error
		expectedErr       string
		expectedCalls     int
		expectedNotified  bool
		expectedDebugLogs bool
	}{
		"success": {
			expectedCalls:    2,
			expectedNotified: true,
		},
		"record changed before the update starts": {
			updateErrors:      []error{data.ErrRecordChanged},
			expectedCalls:     1,
			expectedDebugLogs: true,
		},
		"record changed while the update is in flight": {
			updateErrors:      []error{nil, data.ErrRecordChanged},
			expectedCalls:     2,
			expectedDebugLogs: true,
		},
		"record not found": {
			updateErrors:  []error{data.ErrRecordNotFound},
			expectedErr:   data.ErrRecordNotFound.Error(),
			expectedCalls: 1,
		},
		"database failure is reported": {
			updateErrors:  []error{errDatabaseFailure},
			expectedErr:   errDatabaseFailure.Error(),
			expectedCalls: 1,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recordProvider := newTestProvider("example.com", "sub")
			db := &testDatabase{
				record:       librecords.New(recordProvider, nil),
				updateErrors: testCase.updateErrors,
			}
			shoutrrrClient := &testShoutrrrClient{}
			logger := &testDebugLogger{}
			updater := newTestUpdater(db, shoutrrrClient, logger)

			err := updater.Update(context.Background(), 0, netip.MustParseAddr("1.2.3.4"))

			if testCase.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, testCase.expectedErr)
			}

			assert.Equal(t, testCase.expectedCalls, db.calls())
			assert.Equal(t, testCase.expectedNotified, len(shoutrrrClient.notifications()) > 0)
			assert.Equal(t, testCase.expectedDebugLogs, len(logger.debugMessages()) > 0)
		})
	}
}

// Test_Updater_Update_recordChangedBeforeUpdate checks that a record replaced
// by a configuration reload aborts the update before any network call and
// without notifying the user.
func Test_Updater_Update_recordChangedBeforeUpdate(t *testing.T) {
	t.Parallel()

	recordProvider := newTestProvider("example.com", "sub")
	db := &testDatabase{
		record:       librecords.New(recordProvider, nil),
		updateErrors: []error{data.ErrRecordChanged},
	}
	shoutrrrClient := &testShoutrrrClient{}
	logger := &testDebugLogger{}
	updater := newTestUpdater(db, shoutrrrClient, logger)

	err := updater.Update(context.Background(), 0, netip.MustParseAddr("1.2.3.4"))

	require.NoError(t, err)
	// No network call must have been made.
	assert.Zero(t, recordProvider.calls())
	// The user must not be notified about a discarded update.
	assert.Empty(t, shoutrrrClient.notifications())
	// The reason must be traceable in the debug logs.
	assert.NotEmpty(t, logger.debugMessages())
}

// Test_Updater_Update_recordChangedDuringUpdate uses a real database which is
// reloaded while the provider network call is in flight, which is the exact
// race the identity guard protects against.
func Test_Updater_Update_recordChangedDuringUpdate(t *testing.T) {
	t.Parallel()

	newIP := netip.MustParseAddr("1.2.3.4")

	recordProvider := newTestProvider("example.com", "a")
	reloadedProvider := newTestProvider("example.com", "b")

	persistentDB := &testPersistentDatabase{}
	db := data.NewDatabase([]librecords.Record{
		librecords.New(recordProvider, nil),
	}, persistentDB)

	// The configuration reload happens in the middle of the provider call.
	recordProvider.onUpdate = func() {
		db.Reload([]librecords.Record{librecords.New(reloadedProvider, nil)})
	}

	shoutrrrClient := &testShoutrrrClient{}
	logger := &testDebugLogger{}
	updater := newTestUpdater(db, shoutrrrClient, logger)

	err := updater.Update(context.Background(), 0, newIP)

	require.NoError(t, err)
	// The network call did happen, but its result must be discarded.
	assert.Equal(t, 1, recordProvider.calls())
	// The user must not be notified about a result which was discarded.
	assert.Empty(t, shoutrrrClient.notifications())
	assert.NotEmpty(t, logger.debugMessages())

	// The reloaded record must be left untouched: no status, no message,
	// no history and no persisted IP address.
	stored, err := db.Select(0)
	require.NoError(t, err)
	assert.Equal(t, "example.com.b", stored.Provider.BuildDomainName())
	assert.Equal(t, constants.UNSET, stored.Status)
	assert.Empty(t, stored.Message)
	assert.Empty(t, stored.History)
	assert.Empty(t, persistentDB.newIPs())
	assert.Equal(t, 1, db.Count())
}

// Test_Updater_Update_sameRecordStillPersists checks the guard does not
// reject legitimate updates of an unchanged record.
func Test_Updater_Update_sameRecordStillPersists(t *testing.T) {
	t.Parallel()

	newIP := netip.MustParseAddr("1.2.3.4")

	recordProvider := newTestProvider("example.com", "sub")
	persistentDB := &testPersistentDatabase{}
	db := data.NewDatabase([]librecords.Record{
		librecords.New(recordProvider, nil),
	}, persistentDB)

	shoutrrrClient := &testShoutrrrClient{}
	updater := newTestUpdater(db, shoutrrrClient, &testDebugLogger{})

	require.NoError(t, updater.Update(context.Background(), 0, newIP))

	stored, err := db.Select(0)
	require.NoError(t, err)
	assert.Equal(t, constants.SUCCESS, stored.Status)
	assert.Equal(t, "changed to "+newIP.String(), stored.Message)
	require.Len(t, stored.History, 1)
	assert.Equal(t, newIP, stored.History.GetCurrentIP())
	assert.Equal(t, []netip.Addr{newIP}, persistentDB.newIPs())
	assert.Len(t, shoutrrrClient.notifications(), 1)
}

func Test_Updater_Update_recordNotFound(t *testing.T) {
	t.Parallel()

	db := data.NewDatabase(nil, &testPersistentDatabase{})
	shoutrrrClient := &testShoutrrrClient{}
	updater := newTestUpdater(db, shoutrrrClient, &testDebugLogger{})

	err := updater.Update(context.Background(), 0, netip.MustParseAddr("1.2.3.4"))

	require.ErrorIs(t, err, data.ErrRecordNotFound)
	assert.Empty(t, shoutrrrClient.notifications())
}
