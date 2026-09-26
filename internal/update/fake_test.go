package update

import (
	"context"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/qdm12/ddns-updater/internal/models"
	librecords "github.com/qdm12/ddns-updater/internal/records"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
)

// testProvider is a minimal provider.Provider fake. onUpdate is called while
// the update is in flight, which is where a real provider performs its slow
// network calls and where a configuration reload may occur.
type testProvider struct {
	name       string
	domain     string
	owner      string
	ipVersion  ipversion.IPVersion
	ipv6Suffix netip.Prefix
	proxied    bool

	onUpdate func()

	mu          sync.Mutex
	updateCalls int
}

func newTestProvider(domain, owner string) *testProvider {
	return &testProvider{
		name:      "test",
		domain:    domain,
		owner:     owner,
		ipVersion: ipversion.IP4,
	}
}

func (p *testProvider) String() string {
	return p.name + "|" + p.owner
}

func (p *testProvider) Domain() string {
	return p.domain
}

func (p *testProvider) Owner() string {
	return p.owner
}

func (p *testProvider) BuildDomainName() string {
	if p.owner == "" {
		return p.domain
	}
	return p.domain + "." + p.owner
}

func (p *testProvider) HTML() models.HTMLRow {
	return models.HTMLRow{
		Domain:    p.domain,
		Owner:     p.owner,
		Provider:  p.name,
		IPVersion: p.ipVersion.String(),
	}
}

func (p *testProvider) Proxied() bool {
	return p.proxied
}

func (p *testProvider) IPVersion() ipversion.IPVersion {
	return p.ipVersion
}

func (p *testProvider) IPv6Suffix() netip.Prefix {
	return p.ipv6Suffix
}

func (p *testProvider) Update(_ context.Context, _ *http.Client, ip netip.Addr) (
	newIP netip.Addr, err error,
) {
	p.mu.Lock()
	p.updateCalls++
	p.mu.Unlock()

	if p.onUpdate != nil {
		p.onUpdate()
	}

	return ip, nil
}

func (p *testProvider) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.updateCalls
}

// testShoutrrrClient records the notifications it is given.
type testShoutrrrClient struct {
	mu       sync.Mutex
	messages []string
}

func (c *testShoutrrrClient) Notify(message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, message)
}

func (c *testShoutrrrClient) notifications() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.messages...)
}

// testDebugLogger records the debug messages it is given.
type testDebugLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *testDebugLogger) Debug(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, s)
}

func (l *testDebugLogger) debugMessages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.messages...)
}

// testPersistentDatabase records the new IP addresses it is asked to store.
type testPersistentDatabase struct {
	mu        sync.Mutex
	storedIPs []netip.Addr
}

func (d *testPersistentDatabase) Close() (err error) {
	return nil
}

func (d *testPersistentDatabase) StoreNewIP(_, _ string, ip netip.Addr, _ time.Time) (err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.storedIPs = append(d.storedIPs, ip)
	return nil
}

func (d *testPersistentDatabase) newIPs() []netip.Addr {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]netip.Addr(nil), d.storedIPs...)
}

// testDatabase is a Database fake returning a scripted error for each
// successive Update call.
type testDatabase struct {
	mu           sync.Mutex
	record       librecords.Record
	updateErrors []error
	updateCalls  int
}

func (d *testDatabase) Select(_ uint) (record librecords.Record, err error) {
	return d.record, nil
}

func (d *testDatabase) SelectAll() (all []librecords.Record) {
	return []librecords.Record{d.record}
}

func (d *testDatabase) Update(_ uint, _ librecords.Record) (err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.updateCalls++
	if len(d.updateErrors) == 0 {
		return nil
	}
	err = d.updateErrors[0]
	d.updateErrors = d.updateErrors[1:]

	return err
}

func (d *testDatabase) calls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.updateCalls
}
