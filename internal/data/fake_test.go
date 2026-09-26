package data

import (
	"context"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
)

// testProvider is a minimal provider.Provider fake used to build records
// without pulling in a real DNS provider.
type testProvider struct {
	name       string
	domain     string
	owner      string
	ipVersion  ipversion.IPVersion
	ipv6Suffix netip.Prefix
	proxied    bool

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
	defer p.mu.Unlock()
	p.updateCalls++
	return ip, nil
}

func (p *testProvider) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.updateCalls
}

// testPersistentDatabase records the new IP addresses it is asked to store.
type testPersistentDatabase struct {
	mu           sync.Mutex
	closeCalls   int
	storedIPs    []netip.Addr
	storedOwners []string
}

func (d *testPersistentDatabase) Close() (err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closeCalls++
	return nil
}

func (d *testPersistentDatabase) StoreNewIP(domain, owner string, ip netip.Addr, _ time.Time) (
	err error,
) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.storedIPs = append(d.storedIPs, ip)
	d.storedOwners = append(d.storedOwners, domain+"/"+owner)
	return nil
}

func (d *testPersistentDatabase) newIPs() []netip.Addr {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]netip.Addr(nil), d.storedIPs...)
}
