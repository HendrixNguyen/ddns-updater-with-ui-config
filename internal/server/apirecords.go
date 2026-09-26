package server

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/qdm12/ddns-updater/internal/records"
)

// timeFormat is the RFC3339 layout used for every timestamp of the API.
const timeFormat = time.RFC3339

// providerNameRegexp extracts the provider name out of the string
// representation of a provider, which every provider builds with
// utils.ToString and therefore has the form
// "[domain: d | owner: o | provider: p | ip: v]".
var providerNameRegexp = regexp.MustCompile(`provider: ([^|]*)`) //nolint:gochecknoglobals

// apiRecord is the JSON safe view of a single record. It is built here rather
// than reusing models.HTMLRow because the latter contains raw HTML, which
// must never be embedded in a JSON API response.
type apiRecord struct {
	Domain               string   `json:"domain"`
	Owner                string   `json:"owner"`
	Provider             string   `json:"provider"`
	IPVersion            string   `json:"ipVersion"`
	Status               string   `json:"status"`
	Message              string   `json:"message"`
	CurrentIP            string   `json:"currentIP"`
	PreviousIPs          []string `json:"previousIPs"`
	DurationSinceSuccess string   `json:"durationSinceSuccess"`
	LastUpdate           *string  `json:"lastUpdate"`
	LastBan              *string  `json:"lastBan"`
}

// recordsResponse is the body of a records response.
type recordsResponse struct {
	Records []apiRecord `json:"records"`
	Count   int         `json:"count"`
}

// apiRecords returns a JSON safe structured view of every record currently
// held in the database, so that the dashboard can poll it.
func (h *handlers) apiRecords(w http.ResponseWriter, _ *http.Request) {
	all := h.db.SelectAll()
	records := make([]apiRecord, len(all))
	for i := range all {
		records[i] = h.newAPIRecord(all[i])
	}

	writeJSON(w, http.StatusOK, recordsResponse{
		Records: records,
		Count:   len(records),
	})
}

// newAPIRecord maps a record to its JSON safe representation.
func (h *handlers) newAPIRecord(record records.Record) apiRecord {
	view := apiRecord{
		Status:               string(record.Status),
		Message:              record.Message,
		PreviousIPs:          []string{},
		DurationSinceSuccess: record.History.GetDurationSinceSuccess(h.timeNow()),
		LastUpdate:           formatTimePtr(record.History.GetSuccessTime()),
		LastBan:              formatTimePtrValue(record.LastBan),
	}

	if record.Provider != nil {
		view.Domain = record.Provider.BuildDomainName()
		view.Owner = record.Provider.Owner()
		view.Provider = providerNameOf(record.Provider.String())
		view.IPVersion = record.Provider.IPVersion().String()
	}

	currentIP := record.History.GetCurrentIP()
	if currentIP.IsValid() {
		view.CurrentIP = currentIP.String()
	}

	for _, previousIP := range record.History.GetPreviousIPs() {
		view.PreviousIPs = append(view.PreviousIPs, previousIP.String())
	}

	return view
}

// providerNameOf extracts the provider name out of the string representation
// of a provider. It returns an empty string if the format is not recognized.
func providerNameOf(providerString string) string {
	match := providerNameRegexp.FindStringSubmatch(providerString)
	if len(match) < 2 { //nolint:mnd
		return ""
	}
	return strings.TrimSpace(match[1])
}

// formatTimePtr returns the RFC3339 representation of the time given, or nil
// if the time is the zero time.
func formatTimePtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	formatted := t.Format(timeFormat)
	return &formatted
}

// formatTimePtrValue returns the RFC3339 representation of the time pointer
// given, or nil if the pointer is nil.
func formatTimePtrValue(t *time.Time) *string {
	if t == nil {
		return nil
	}
	return formatTimePtr(*t)
}
