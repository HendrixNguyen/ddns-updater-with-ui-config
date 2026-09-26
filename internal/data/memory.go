package data

import (
	"errors"
	"fmt"

	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/internal/records"
)

var ErrRecordNotFound = errors.New("record not found")

// cloneRecord returns a copy of the record given which shares no mutable
// state with it. Records hold a history slice and a last ban time pointer, so
// handing out the record as-is would let the caller append to the very
// backing array stored in the database, racing with (and corrupting) any
// concurrent reader of that database.
func cloneRecord(record records.Record) records.Record {
	cloned := record
	cloned.History = make(models.History, len(record.History))
	copy(cloned.History, record.History)
	if record.LastBan != nil {
		lastBan := *record.LastBan
		cloned.LastBan = &lastBan
	}
	return cloned
}

// cloneRecords returns a copy of each record given, sharing no mutable state
// with them, so that the caller can iterate over the result concurrently
// with a Reload and always observe a single coherent snapshot.
func cloneRecords(rs []records.Record) []records.Record {
	cloned := make([]records.Record, len(rs))
	for i := range rs {
		cloned[i] = cloneRecord(rs[i])
	}
	return cloned
}

func (db *Database) Select(id uint) (record records.Record, err error) {
	db.RLock()
	defer db.RUnlock()
	if id >= uint(len(db.data)) {
		return record, fmt.Errorf("%w: for id %d", ErrRecordNotFound, id)
	}
	return cloneRecord(db.data[id]), nil
}

func (db *Database) SelectAll() (all []records.Record) {
	db.RLock()
	defer db.RUnlock()
	// The records are copied instead of returning db.data itself, so that a
	// caller iterating over the result concurrently with a Reload always
	// observes a single coherent snapshot and never aliases the backing array
	// of the database. Returning db.data would also be safe in Go, since
	// Reload assigns a brand new slice rather than mutating the existing one,
	// but the copy makes the guarantee explicit.
	return cloneRecords(db.data)
}
