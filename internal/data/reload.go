package data

import (
	"errors"
	"strconv"
	"strings"

	"github.com/qdm12/ddns-updater/internal/records"
)

// ErrRecordChanged is returned when a record was replaced by a configuration
// reload while an update was in flight for it, so the update result must be
// discarded instead of applied to a different record.
var ErrRecordChanged = errors.New("record was changed by a configuration reload")

// Reload replaces every record in the database. Existing update operations
// still in flight will detect the change and discard their results through
// ErrRecordChanged, so the new records are never overwritten by stale data.
// The records are copied, so the caller keeps full ownership of the slice it
// passes in and can reuse or modify it afterwards.
func (db *Database) Reload(newRecords []records.Record) {
	db.Lock()
	defer db.Unlock()
	db.data = cloneRecords(newRecords)
}

// Count returns the number of records currently stored in the database.
func (db *Database) Count() int {
	db.RLock()
	defer db.RUnlock()
	return len(db.data)
}

// identity returns a stable fingerprint of the DNS target configuration of
// the record given. Two records sharing an identity are the same DNS target,
// so an update result computed for one of them is still valid for the other.
// It returns an empty string for a record without a provider.
func identity(record records.Record) string {
	recordProvider := record.Provider
	if recordProvider == nil {
		return ""
	}

	// NUL bytes are used as separators since they cannot appear in any of
	// the fields below, which guarantees distinct configurations never
	// produce the same fingerprint.
	var builder strings.Builder
	builder.WriteString(recordProvider.BuildDomainName())
	builder.WriteByte(0)
	builder.WriteString(recordProvider.String())
	builder.WriteByte(0)
	builder.WriteString(recordProvider.IPVersion().String())
	builder.WriteByte(0)
	builder.WriteString(recordProvider.IPv6Suffix().String())
	builder.WriteByte(0)
	builder.WriteString(strconv.FormatBool(recordProvider.Proxied()))

	return builder.String()
}
