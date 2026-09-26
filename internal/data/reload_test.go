package data

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/qdm12/ddns-updater/internal/records"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRecords builds count records with distinct owners so that each of them
// has a distinct identity.
func newRecords(count int) []records.Record {
	newRecords := make([]records.Record, count)
	for i := range count {
		owner := string(rune('a' + i))
		newRecords[i] = records.New(newTestProvider("example.com", owner), nil)
	}

	return newRecords
}

func domainNames(rs []records.Record) []string {
	names := make([]string, len(rs))
	for i := range rs {
		names[i] = rs[i].Provider.BuildDomainName()
	}

	return names
}

func Test_Reload(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		initialRecords []records.Record
		newRecords     []records.Record
	}{
		"no records": {
			newRecords: nil,
		},
		"identical records": {
			initialRecords: newRecords(3),
			newRecords:     newRecords(3),
		},
		"less records": {
			initialRecords: newRecords(3),
			newRecords:     newRecords(1),
		},
		"more records": {
			initialRecords: newRecords(1),
			newRecords:     newRecords(4),
		},
		"different records": {
			initialRecords: newRecords(2),
			newRecords:     newRecords(2),
		},
		"remove all records": {
			initialRecords: newRecords(2),
		},
		"remove one record shifting indices": {
			initialRecords: newRecords(3),
			newRecords:     newRecords(2),
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			db := NewDatabase(testCase.initialRecords, &testPersistentDatabase{})

			db.Reload(testCase.newRecords)

			assert.Equal(t, len(testCase.newRecords), db.Count())

			all := db.SelectAll()
			require.Len(t, all, len(testCase.newRecords))
			assert.Equal(t, domainNames(testCase.newRecords), domainNames(all))

			for i := range testCase.newRecords {
				record, err := db.Select(uint(i))
				require.NoError(t, err)
				assert.Equal(t, testCase.newRecords[i].Provider.BuildDomainName(),
					record.Provider.BuildDomainName())
			}

			_, err := db.Select(uint(len(testCase.newRecords)))
			require.ErrorIs(t, err, ErrRecordNotFound)
		})
	}
}

func Test_SelectAll_returns_a_copy(t *testing.T) {
	t.Parallel()

	db := NewDatabase(newRecords(2), &testPersistentDatabase{})

	all := db.SelectAll()
	all[0] = records.New(newTestProvider("mutated.com", "z"), nil)

	again := db.SelectAll()
	require.Len(t, again, 2)
	assert.Equal(t, "example.com.a", again[0].Provider.BuildDomainName())
	assert.Equal(t, 2, db.Count())
}

func Test_Reload_while_Select_in_flight(t *testing.T) {
	t.Parallel()

	db := NewDatabase(newRecords(1), &testPersistentDatabase{})

	const iterations = 2000

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	go func() {
		defer waitGroup.Done()
		for range iterations {
			for _, record := range db.SelectAll() {
				// A torn read would show up as a nil provider here.
				require.NotNil(t, record.Provider)
			}
		}
	}()

	go func() {
		defer waitGroup.Done()
		for i := range iterations {
			db.Reload(newRecords(i%4 + 1))
		}
	}()

	waitGroup.Wait()
}

func Test_Reload_concurrent_with_SelectAll_and_Update(t *testing.T) {
	t.Parallel()

	persistentDB := &testPersistentDatabase{}
	db := NewDatabase(newRecords(4), persistentDB)

	const (
		duration   = 300 * time.Millisecond
		goroutines = 4
	)

	deadline := time.Now().Add(duration)

	var waitGroup sync.WaitGroup

	for range goroutines {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for time.Now().Before(deadline) {
				for _, record := range db.SelectAll() {
					if record.Provider == nil {
						t.Error("got a record with a nil provider")
						return
					}
				}
			}
		}()
	}

	for range goroutines {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for time.Now().Before(deadline) {
				count := db.Count()
				if count == 0 {
					continue
				}
				// The record is fetched again by ID and written back, which
				// is exactly what the update cycle does. A concurrent reload
				// may make the ID out of range or the identity mismatch, both
				// of which must be reported as errors instead of corrupting
				// or panicking.
				record, err := db.Select(0)
				if err != nil {
					require.ErrorIs(t, err, ErrRecordNotFound)
					continue
				}
				err = db.Update(0, record)
				if err != nil {
					require.ErrorIs(t, err, ErrRecordChanged)
				}
			}
		}()
	}

	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		for i := 0; time.Now().Before(deadline); i++ {
			db.Reload(newRecords(i%4 + 1))
		}
	}()

	waitGroup.Wait()
}

func Test_identity(t *testing.T) {
	t.Parallel()

	base := func() *testProvider {
		recordProvider := newTestProvider("example.com", "sub")
		recordProvider.name = "cloudflare"
		recordProvider.ipVersion = ipversion.IP4
		recordProvider.proxied = true
		recordProvider.ipv6Suffix = netip.MustParsePrefix("0:0:0:0:0:0:0:0/0")
		return recordProvider
	}

	testCases := map[string]struct {
		mutate     func(p *testProvider)
		sameAsBase bool
	}{
		"identical":            {sameAsBase: true},
		"different domain":     {mutate: func(p *testProvider) { p.domain = "other.com" }},
		"different owner":      {mutate: func(p *testProvider) { p.owner = "other" }},
		"different provider":   {mutate: func(p *testProvider) { p.name = "other" }},
		"different ip version": {mutate: func(p *testProvider) { p.ipVersion = ipversion.IP6 }},
		"different suffix": {
			mutate: func(p *testProvider) {
				p.ipv6Suffix = netip.MustParsePrefix("2001:db8::/64")
			},
		},
		"different proxied": {mutate: func(p *testProvider) { p.proxied = false }},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			baseProvider := base()
			otherProvider := base()
			if testCase.mutate != nil {
				testCase.mutate(otherProvider)
			}

			baseIdentity := identity(records.New(baseProvider, nil))
			otherIdentity := identity(records.New(otherProvider, nil))

			if testCase.sameAsBase {
				assert.Equal(t, baseIdentity, otherIdentity)
				return
			}
			assert.NotEqual(t, baseIdentity, otherIdentity)
		})
	}
}

func Test_identity_field_boundaries(t *testing.T) {
	t.Parallel()

	// Field separators must make it impossible to shift a character from one
	// field to the next and end up with the same fingerprint.
	first := newTestProvider("ab", "c")
	second := newTestProvider("a", "bc")

	first.name = "same"
	second.name = "same"
	first.ipVersion = ipversion.IP4
	second.ipVersion = ipversion.IP4

	assert.NotEqual(t,
		identity(records.New(first, nil)),
		identity(records.New(second, nil)))
}

func Test_identity_nil_provider(t *testing.T) {
	t.Parallel()

	var record records.Record
	assert.Empty(t, identity(record))
}
