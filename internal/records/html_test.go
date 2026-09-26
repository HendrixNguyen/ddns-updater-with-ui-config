package records

import (
	"context"
	"net/http"
	"net/netip"
	"testing"
	"time"

	appconstants "github.com/qdm12/ddns-updater/internal/constants"
	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// escapeTestProvider is a provider.Provider whose domain and owner hold markup,
// to check the index page cannot be injected with settings values.
type escapeTestProvider struct {
	domain string
	owner  string
}

func (p *escapeTestProvider) Settings() bool          { return false }
func (p *escapeTestProvider) BuildDomainName() string { return p.owner + "." + p.domain }
func (p *escapeTestProvider) Domain() string          { return p.domain }
func (p *escapeTestProvider) Owner() string           { return p.owner }
func (p *escapeTestProvider) String() string {
	return "domain: " + p.domain + " | owner: " + p.owner
}
func (p *escapeTestProvider) HTML() models.HTMLRow {
	// Deliberately unescaped, as every provider in the tree still is: the
	// escaping must happen where the row is assembled, not here.
	return models.HTMLRow{
		Domain:    `<a href="http://` + p.BuildDomainName() + `">` + p.BuildDomainName() + `</a>`,
		Owner:     p.Owner(),
		Provider:  "<a href=\"https://example.com\">Example</a>",
		IPVersion: ipversion.IP4.String(),
	}
}
func (p *escapeTestProvider) Proxied() bool                  { return false }
func (p *escapeTestProvider) IPVersion() ipversion.IPVersion { return ipversion.IP4 }
func (p *escapeTestProvider) IPv6Suffix() netip.Prefix       { return netip.Prefix{} }
func (p *escapeTestProvider) Update(_ context.Context, _ *http.Client, ip netip.Addr) (
	newIP netip.Addr, err error,
) {
	return ip, nil
}

// Test_HTML_escapesUserSuppliedValues checks the domain, the owner and the
// provider message cannot inject markup into the index page, which is rendered
// by a text/template and therefore escapes nothing.
func Test_HTML_escapesUserSuppliedValues(t *testing.T) {
	t.Parallel()

	record := Record{
		Provider: &escapeTestProvider{
			domain: "<script>alert(1)</script>.example.com",
			owner:  "<img src=x onerror=alert(2)>",
		},
		Status:  appconstants.FAIL,
		Message: "<svg onload=alert(3)>",
		Time:    time.Now(),
	}

	row := record.HTML(time.Now())

	assert.NotContains(t, row.Domain, "<script>")
	assert.NotContains(t, row.Domain, "<img")
	assert.NotContains(t, row.Owner, "<img")
	assert.NotContains(t, row.Status, "<svg")
	// The markup the record builds itself is preserved.
	assert.Contains(t, row.Domain, "<a href=")
	assert.Contains(t, row.Status, `<span class="error">Failure</span>`)
}

// Test_HTML_plainValuesAreUnchanged checks the escaping is transparent for
// ordinary domain names, so the index page keeps rendering the same.
func Test_HTML_plainValuesAreUnchanged(t *testing.T) {
	t.Parallel()

	record := Record{
		Provider: &escapeTestProvider{domain: "example.com", owner: "sub"},
		Status:   appconstants.SUCCESS,
		Message:  "changed to 1.2.3.4",
		Time:     time.Now(),
	}

	row := record.HTML(time.Now())

	assert.Equal(t, `<a href="http://sub.example.com">sub.example.com</a>`, row.Domain)
	assert.Equal(t, "sub", row.Owner)
	assert.Contains(t, row.Status, "(changed to 1.2.3.4)")
}

func Test_HTML_uptodateMessage(t *testing.T) {
	t.Parallel()

	record := Record{
		Provider: &escapeTestProvider{domain: "example.com", owner: "sub"},
		Status:   appconstants.UPTODATE,
		Time:     time.Now(),
	}
	record.History = models.History{{IP: netip.MustParseAddr("1.2.3.4"), Time: time.Now()}}

	row := record.HTML(time.Now())

	require.NotEmpty(t, row.Status)
	assert.Contains(t, row.Status, "no IP change for")
}
