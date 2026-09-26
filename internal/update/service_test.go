package update

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/qdm12/ddns-updater/internal/data"
	"github.com/qdm12/ddns-updater/internal/healthchecksio"
	librecords "github.com/qdm12/ddns-updater/internal/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newReloadingRecords alternates between two configurations holding a single
// record each, so that any write landing on a record replaced by a reload
// carries a different DNS target than the one it was computed for.
func newReloadingRecords(index int) []librecords.Record {
	domain, owner := "a.com", "a"
	if index%2 == 0 {
		domain, owner = "b.com", "b"
	}

	return []librecords.Record{
		librecords.New(newTestProvider(domain, owner), nil),
	}
}

// Test_Updater_Update_concurrent_with_reload hammers the update cycle against
// a database being reloaded at the same time. A reload racing with an update
// must never corrupt a record: the stale update result is either applied to
// the very record it was computed for, or rejected with
// data.ErrRecordChanged.
func Test_Updater_Update_concurrent_with_reload(t *testing.T) {
	t.Parallel()

	const (
		duration   = 300 * time.Millisecond
		goroutines = 4
		updateIP   = "1.2.3.4"
	)

	ip := netip.MustParseAddr(updateIP)
	knownDomains := map[string]struct{}{
		"a.com.a": {},
		"b.com.b": {},
	}

	persistentDB := &testPersistentDatabase{}
	db := data.NewDatabase(newReloadingRecords(0), persistentDB)
	shoutrrrClient := &testShoutrrrClient{}
	updater := newTestUpdater(db, shoutrrrClient, &testDebugLogger{})

	deadline := time.Now().Add(duration)
	var waitGroup sync.WaitGroup

	for range goroutines {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for time.Now().Before(deadline) {
				err := updater.Update(context.Background(), 0, ip)
				if err != nil {
					require.ErrorIs(t, err, data.ErrRecordChanged)
				}
			}
		}()
	}

	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		for i := 0; time.Now().Before(deadline); i++ {
			db.Reload(newReloadingRecords(i))
		}
	}()

	waitGroup.Wait()

	// Every record still stored must be one of the two known configurations
	// and must only carry the IP address the updater was asked to set.
	all := db.SelectAll()
	require.Len(t, all, 1)
	_, known := knownDomains[all[0].Provider.BuildDomainName()]
	assert.True(t, known, "unexpected domain %q", all[0].Provider.BuildDomainName())
	for _, historyEvent := range all[0].History {
		assert.Equal(t, ip, historyEvent.IP)
	}
}

// Test_Service_updateNecessary_concurrent_with_reload checks the update cycle
// as a whole survives a database reload happening in the middle of it.
func Test_Service_updateNecessary_concurrent_with_reload(t *testing.T) {
	t.Parallel()

	const duration = 300 * time.Millisecond

	db := data.NewDatabase(newReloadingRecords(0), &testPersistentDatabase{})

	ipGetter := &testPublicIPFetcher{ip: netip.MustParseAddr("1.2.3.4")}
	resolver := &testResolver{}
	logger := &testLogger{}
	healthchecksClient := &testHealthchecksIOClient{}
	service := NewService(db, &testUpdater{}, ipGetter, time.Hour, 0, logger, resolver,
		time.Now, healthchecksClient)

	deadline := time.Now().Add(duration)

	var waitGroup sync.WaitGroup

	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		for time.Now().Before(deadline) {
			db.Reload(newReloadingRecords(int(time.Now().UnixNano())))
		}
	}()

	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		for time.Now().Before(deadline) {
			errs := service.updateNecessary(context.Background())
			for _, err := range errs {
				// Only a genuine failure may be reported; a record changed by
				// a reload is silently skipped.
				assert.NotErrorIs(t, err, data.ErrRecordChanged)
				assert.NotErrorIs(t, err, data.ErrRecordNotFound)
			}
		}
	}()

	waitGroup.Wait()
}

type testPublicIPFetcher struct {
	ip netip.Addr
}

func (f *testPublicIPFetcher) IP(_ context.Context) (netip.Addr, error) {
	return f.ip, nil
}

func (f *testPublicIPFetcher) IP4(_ context.Context) (netip.Addr, error) {
	return f.ip, nil
}

func (f *testPublicIPFetcher) IP6(_ context.Context) (netip.Addr, error) {
	return f.ip, nil
}

type testResolver struct{}

func (r *testResolver) LookupNetIP(_ context.Context, _, _ string) (
	ips []netip.Addr, err error,
) {
	return []netip.Addr{netip.MustParseAddr("1.2.3.4")}, nil
}

type testLogger struct {
	testDebugLogger
}

func (l *testLogger) Info(_ string)  {}
func (l *testLogger) Warn(_ string)  {}
func (l *testLogger) Error(_ string) {}

type testHealthchecksIOClient struct{}

func (c *testHealthchecksIOClient) Ping(_ context.Context, _ healthchecksio.State) (err error) {
	return nil
}

type testUpdater struct{}

func (u *testUpdater) Update(_ context.Context, recordID uint, _ netip.Addr) (err error) {
	return nil
}
