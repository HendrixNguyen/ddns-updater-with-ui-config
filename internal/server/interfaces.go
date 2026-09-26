package server

import (
	"context"
	"encoding/json"

	"github.com/qdm12/ddns-updater/internal/manager"
	"github.com/qdm12/ddns-updater/internal/records"
)

type Database interface {
	SelectAll() (records []records.Record)
	// Count returns the number of records currently held by the database.
	Count() (count int)
}

type UpdateForcer interface {
	ForceUpdate(ctx context.Context) (errors []error)
}

type Logger interface {
	Info(s string)
	Warn(s string)
	Error(s string)
}

// SettingsManager reads and writes the DNS settings configuration and applies
// the changes to the running updater.
type SettingsManager interface {
	Filepath() string
	Entries() (entries []json.RawMessage, warnings []string, err error)
	Validate(entries []json.RawMessage) (warnings []string, err error)
	AddEntry(entry json.RawMessage) (result manager.Result, err error)
	ReplaceEntry(index int, entry json.RawMessage) (result manager.Result, err error)
	DeleteEntry(index int) (result manager.Result, err error)
	ReplaceAllEntries(entries []json.RawMessage) (result manager.Result, err error)
	Reload() (warnings []string, err error)
}
