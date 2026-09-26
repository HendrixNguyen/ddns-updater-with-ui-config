package manager

import (
	"github.com/qdm12/ddns-updater/internal/models"
	recordslib "github.com/qdm12/ddns-updater/internal/records"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
)

// Database is the in-memory record database the manager hot reloads.
type Database interface {
	Reload(records []recordslib.Record)
	Count() int
}

// HistoryReader reads the persisted IP history of a DNS record.
type HistoryReader interface {
	GetEvents(domain, owner string, ipversion ipversion.IPVersion) (
		events []models.HistoryEvent, err error)
}

// Logger is the subset of the logger the manager uses.
type Logger interface {
	Info(s string)
	Warn(s string)
	Debug(s string)
	Error(s string)
}

// Notifier is notified of settings warnings and reload outcomes.
type Notifier interface {
	Notify(s string)
}

// noopLogger discards every log line, and is used when a nil logger is given
// to New or to the package level Providers and BuildRecords functions, so that
// logging never panics.
type noopLogger struct{}

func (noopLogger) Info(string)  {}
func (noopLogger) Warn(string)  {}
func (noopLogger) Debug(string) {}
func (noopLogger) Error(string) {}

// noopNotifier drops every notification, and is used when a nil notifier is
// given to New or to the package level BuildRecords function, so that
// notifying never panics.
type noopNotifier struct{}

func (noopNotifier) Notify(string) {}
