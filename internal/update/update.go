package update

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/qdm12/ddns-updater/internal/constants"
	"github.com/qdm12/ddns-updater/internal/data"
	"github.com/qdm12/ddns-updater/internal/models"
	settingserrors "github.com/qdm12/ddns-updater/internal/provider/errors"
)

type Updater struct {
	db             Database
	client         *http.Client
	shoutrrrClient ShoutrrrClient
	logger         DebugLogger
	timeNow        func() time.Time
}

func NewUpdater(db Database, client *http.Client, shoutrrrClient ShoutrrrClient,
	logger DebugLogger, timeNow func() time.Time, debugEnabled bool,
) *Updater {
	if debugEnabled {
		client = makeLogClient(client, logger)
	}
	return &Updater{
		db:             db,
		client:         client,
		shoutrrrClient: shoutrrrClient,
		logger:         logger,
		timeNow:        timeNow,
	}
}

func (u *Updater) Update(ctx context.Context, id uint, ip netip.Addr) (err error) {
	record, err := u.db.Select(id)
	if err != nil {
		return err
	}
	record.Time = u.timeNow()
	record.Status = constants.UPDATING
	err = u.db.Update(id, record)
	if err != nil {
		return u.skipIfRecordChanged(id, err)
	}
	record.Status = constants.FAIL
	newIP, err := record.Provider.Update(ctx, u.client, ip)
	if err != nil {
		record.Message = err.Error()
		if errors.Is(err, settingserrors.ErrBannedAbuse) {
			lastBan := time.Unix(u.timeNow().Unix(), 0)
			record.LastBan = &lastBan
			domainName := record.Provider.BuildDomainName()
			message := domainName + ": " + record.Message +
				", no more updates will be attempted for an hour"
			u.shoutrrrClient.Notify(message)
			err = fmt.Errorf("%w: for domain %s, no more update will be attempted for 1h", err, domainName)
		} else {
			record.LastBan = nil // clear a previous ban
		}
		if updateErr := u.db.Update(id, record); updateErr != nil {
			return u.skipIfRecordChanged(id, fmt.Errorf(
				"%w (with database update error: %w)", err, updateErr))
		}
		return err
	}
	record.Status = constants.SUCCESS
	record.Message = "changed to " + ip.String()
	record.History = append(record.History, models.HistoryEvent{
		IP:   newIP,
		Time: u.timeNow(),
	})
	// The result is persisted first (this stores the new IP if needed) and the
	// user is only notified when the record was not reconfigured in the
	// meantime, so a result computed for a previous configuration is never
	// announced nor applied to a different record.
	err = u.db.Update(id, record)
	if recordChanged(err) {
		u.logRecordChanged(id, err)
		return nil
	} else if err != nil {
		return err
	}
	u.shoutrrrClient.Notify(record.Provider.BuildDomainName() + " " + record.Message)

	return nil
}

// recordChanged reports whether err is a data.ErrRecordChanged error, meaning
// the record was reconfigured by a configuration reload while its update was
// in flight, so the update result must be discarded.
func recordChanged(err error) bool {
	return errors.Is(err, data.ErrRecordChanged)
}

// logRecordChanged logs at debug level that an update result was discarded
// because the record was reconfigured by a configuration reload. The newly
// configured record is updated on the next cycle instead, so this is not a
// failure worth alerting the user about.
func (u *Updater) logRecordChanged(id uint, err error) {
	u.logger.Debug("Discarding update result for record id " +
		strconv.FormatUint(uint64(id), 10) +
		" because it was changed by a configuration reload: " + err.Error())
}

// skipIfRecordChanged returns nil without notifying the user when err is a
// data.ErrRecordChanged error, since the record was reconfigured by a
// configuration reload while its update was in flight. The update result
// computed for the previous configuration is discarded, as it does not
// describe the record currently stored at the same index. Any other error is
// returned unchanged.
func (u *Updater) skipIfRecordChanged(id uint, err error) error {
	if !recordChanged(err) {
		return err
	}
	u.logRecordChanged(id, err)

	return nil
}
