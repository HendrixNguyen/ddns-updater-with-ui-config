package data

import (
	"fmt"

	"github.com/qdm12/ddns-updater/internal/records"
)

func (db *Database) Update(id uint, record records.Record) (err error) {
	db.Lock()
	defer db.Unlock()
	if id >= uint(len(db.data)) {
		return fmt.Errorf("%w: for id %d", ErrRecordNotFound, id)
	}

	// The record ID is the index in the database slice, so a configuration
	// reload may have replaced the record living at this index, or shifted
	// every subsequent record by a different amount. Refuse to write in that
	// case, otherwise we would apply stale data (status, history, IP address)
	// to a completely different DNS target. The comparison and the mutation
	// below are both done under the same write lock, so the check can never
	// be invalidated by a concurrent reload.
	if identity(db.data[id]) != identity(record) {
		return fmt.Errorf("%w: for id %d", ErrRecordChanged, id)
	}

	currentCount := len(db.data[id].History)
	newCount := len(record.History)
	db.data[id] = record
	// new IP address added
	if newCount > currentCount {
		if err := db.persistentDB.StoreNewIP(
			record.Provider.Domain(),
			record.Provider.Owner(),
			record.History.GetCurrentIP(),
			record.History.GetSuccessTime(),
		); err != nil {
			return err
		}
	}
	return nil
}
