package records

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/qdm12/ddns-updater/internal/constants"
	"github.com/qdm12/ddns-updater/internal/models"
)

func (r *Record) HTML(now time.Time) models.HTMLRow {
	const NotAvailable = "N/A"
	row := r.Provider.HTML()
	// The index page is rendered with a text/template, which does not escape
	// anything, and the only markup of a row is the one built here. The domain
	// and the owner come straight from the user supplied settings, and the
	// message from the provider API response, so they are HTML escaped. The
	// Domain and Owner fields are rebuilt here rather than taken from the
	// provider so that a single place escapes them for every provider.
	domainName := html.EscapeString(r.Provider.BuildDomainName())
	row.Domain = `<a href="http://` + domainName + `">` + domainName + `</a>`
	row.Owner = html.EscapeString(r.Provider.Owner())
	message := html.EscapeString(r.Message)
	if r.Status == constants.UPTODATE {
		message = "no IP change for " + r.History.GetDurationSinceSuccess(now)
	}
	if message != "" {
		message = fmt.Sprintf("(%s)", message)
	}
	if r.Status == "" {
		row.Status = NotAvailable
	} else {
		row.Status = fmt.Sprintf("%s %s, %s",
			convertStatus(r.Status),
			message,
			time.Since(r.Time).Round(time.Second).String()+" ago")
	}
	currentIP := r.History.GetCurrentIP()
	if currentIP.IsValid() {
		row.CurrentIP = `<a href="https://ipinfo.io/` + currentIP.String() + `">` + currentIP.String() + "</a>"
	} else {
		row.CurrentIP = NotAvailable
	}
	previousIPs := r.History.GetPreviousIPs()
	row.PreviousIPs = NotAvailable
	if len(previousIPs) > 0 {
		var previousIPsStr []string
		const maxPreviousIPs = 2
		for i, previousIP := range previousIPs {
			if i == maxPreviousIPs {
				previousIPsStr = append(previousIPsStr, fmt.Sprintf("and %d more", len(previousIPs)-i))
				break
			}
			previousIPsStr = append(previousIPsStr, previousIP.String())
		}
		row.PreviousIPs = strings.Join(previousIPsStr, ", ")
	}
	return row
}

func convertStatus(status models.Status) string {
	switch status {
	case constants.SUCCESS:
		return `<span class="success">Success</span>`
	case constants.FAIL:
		return `<span class="error">Failure</span>`
	case constants.UPTODATE:
		return `<span class="uptodate">Up to date</span>`
	case constants.UPDATING:
		return `<span class="updating">Updating</span>`
	case constants.UNSET:
		return `<span class="unset">Unset</span>`
	default:
		return "Unknown status"
	}
}
