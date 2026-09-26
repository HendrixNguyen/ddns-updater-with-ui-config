// Package manager owns the pipeline going from the settings configuration file
// to the live in-memory records, and persists every settings change made
// through the HTTP API before hot reloading it into the running updater.
package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/qdm12/ddns-updater/internal/params"
	"github.com/qdm12/ddns-updater/internal/provider"
	recordslib "github.com/qdm12/ddns-updater/internal/records"
)

const (
	// configEnvVar is the name of the environment variable which can hold the
	// settings as a JSON document, and which takes precedence over the file when
	// it changed since it was last written to the file.
	configEnvVar = "CONFIG"
	// File and environment sources reported in a Result.
	sourceEmpty = "empty"
	sourceEnv   = "env"
	sourceFile  = "file"
	// Environment variable states reported in a Result.
	envSet   = "set"
	envUnset = "unset"
)

// Result describes the current state of the settings configuration and of the
// records it produced, ready to be serialized to JSON by the HTTP layer.
type Result struct {
	Settings []json.RawMessage `json:"settings"`
	Warnings []string          `json:"warnings"`
	// Source is where the running configuration came from: "file", "env" or "empty".
	Source string `json:"source"`
	// Env is "set" when the CONFIG environment variable is set, "unset" otherwise.
	Env      string `json:"env"`
	FilePath string `json:"filePath"`
	// Records is the number of in-memory DNS records the settings produced.
	Records int `json:"records"`
}

// Manager owns the settings configuration file of the running program. It
// writes every change to the file and hot reloads the resulting DNS providers
// and records into the in-memory database, without needing a restart.
// All its methods are safe for concurrent use: mutations are serialized.
type Manager struct {
	// mutex serializes the whole read, save, reload and rollback sequence of a
	// mutation, so that concurrent changes cannot interleave and leave the file
	// and the running records out of sync.
	mutex   sync.Mutex
	store   *params.Store
	history HistoryReader
	logger  Logger
	notify  Notifier
	db      Database
}

// New creates a settings manager. The database is given after construction
// because building it requires the providers, which this manager also builds.
func New(store *params.Store, history HistoryReader, logger Logger, notify Notifier) *Manager {
	if logger == nil {
		logger = noopLogger{}
	}

	if notify == nil {
		notify = noopNotifier{}
	}

	return &Manager{
		store:   store,
		history: history,
		logger:  logger,
		notify:  notify,
	}
}

// AttachDatabase gives the manager the in-memory database to hot reload.
// It must be called once, after the database is built, before the HTTP server
// starts. Calling it more than once is a programming error and panics.
func (m *Manager) AttachDatabase(db Database) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.db != nil {
		panic("settings manager database is already attached")
	}

	m.db = db
}

// Providers reads the settings entries from the configuration file and builds
// the DNS providers they describe.
func Providers(store *params.Store) (
	providers []provider.Provider, warnings []string, err error,
) {
	return providersFromStore(store, noopLogger{})
}

// providersFromStore is the same as Providers but logs what it reads using the
// logger given.
func providersFromStore(store *params.Store, logger params.Logger) (
	providers []provider.Provider, warnings []string, err error,
) {
	return params.NewReader(logger).JSONProviders(store.Filepath())
}

// BuildRecords builds the in-memory records for the given providers, restoring
// the IP history of each of them from the persistent database.
func BuildRecords(providers []provider.Provider, history HistoryReader) (
	records []recordslib.Record, err error,
) {
	return buildRecords(providers, history, noopLogger{}, noopNotifier{})
}

// buildRecords is the same as BuildRecords but logs each history it reads and
// notifies the user if a history cannot be read.
func buildRecords(providers []provider.Provider, history HistoryReader,
	logger Logger, notify Notifier,
) (records []recordslib.Record, err error,
) {
	allRecords := make([]recordslib.Record, len(providers))
	for i, recordProvider := range providers {
		logger.Info("Reading history from database: domain " +
			recordProvider.Domain() + " owner " + recordProvider.Owner() +
			" " + recordProvider.IPVersion().String())
		events, err := history.GetEvents(recordProvider.Domain(),
			recordProvider.Owner(), recordProvider.IPVersion())
		if err != nil {
			notify.Notify(err.Error())
			return nil, err
		}
		allRecords[i] = recordslib.New(recordProvider, events)
	}

	return allRecords, nil
}

// Filepath returns the path of the settings file the manager operates on.
func (m *Manager) Filepath() string {
	return m.store.Filepath()
}

// Entries returns the settings entries found in the settings file, together
// with the warnings found while reading them. The returned slices are never
// nil, so the HTTP layer can serialize them as empty JSON arrays.
func (m *Manager) Entries() (entries []json.RawMessage, warnings []string, err error) {
	entries, warnings, err = m.store.Entries()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errReadSettings, err)
	}

	return nonNilEntries(entries), nonNilWarnings(warnings), nil
}

// Validate checks each of the settings entries given without saving them, and
// reports every entry which is invalid as a *ValidationError.
func (m *Manager) Validate(entries []json.RawMessage) (warnings []string, err error) {
	warnings, err = m.store.ValidateAll(entries)
	if err != nil {
		return nonNilWarnings(warnings), &ValidationError{Err: err}
	}

	return nonNilWarnings(warnings), nil
}

// AddEntry appends the settings entry given to the settings file and hot
// reloads the resulting records. An invalid entry is rejected as a
// *ValidationError and leaves the settings file untouched.
func (m *Manager) AddEntry(entry json.RawMessage) (result Result, err error) {
	warnings, err := params.ValidateEntry(entry)
	if err != nil {
		return result, &ValidationError{Err: err}
	}

	return m.mutate(warnings, func(entries []json.RawMessage) ([]json.RawMessage, error) {
		return append(entries, entry), nil
	})
}

// ReplaceEntry replaces the settings entry at the index given and hot reloads
// the resulting records. An invalid entry is rejected as a *ValidationError
// and leaves the settings file untouched. If the index does not exist, the
// error matches errors.Is(err, ErrEntryNotFound).
func (m *Manager) ReplaceEntry(index int, entry json.RawMessage) (result Result, err error) {
	warnings, err := params.ValidateEntry(entry)
	if err != nil {
		return result, &ValidationError{Err: err}
	}

	return m.mutate(warnings, func(entries []json.RawMessage) ([]json.RawMessage, error) {
		if index < 0 || index >= len(entries) {
			return nil, notFoundError(index, len(entries))
		}

		newEntries := copyEntries(entries)
		newEntries[index] = entry

		return newEntries, nil
	})
}

// DeleteEntry removes the settings entry at the index given and hot reloads
// the resulting records. If the index does not exist, the error matches
// errors.Is(err, ErrEntryNotFound).
func (m *Manager) DeleteEntry(index int) (result Result, err error) {
	return m.mutate(nil, func(entries []json.RawMessage) ([]json.RawMessage, error) {
		if index < 0 || index >= len(entries) {
			return nil, notFoundError(index, len(entries))
		}

		newEntries := make([]json.RawMessage, 0, len(entries)-1)
		newEntries = append(newEntries, entries[:index]...)
		newEntries = append(newEntries, entries[index+1:]...)

		return newEntries, nil
	})
}

// ReplaceAllEntries replaces every settings entry and hot reloads the resulting
// records. At least one entry must be invalid to be rejected as a
// *ValidationError, in which case the settings file is left untouched.
func (m *Manager) ReplaceAllEntries(entries []json.RawMessage) (result Result, err error) {
	warnings, err := m.store.ValidateAll(entries)
	if err != nil {
		return result, &ValidationError{Err: err}
	}

	return m.mutate(warnings, func([]json.RawMessage) ([]json.RawMessage, error) {
		return copyEntries(entries), nil
	})
}

// Reload rebuilds every DNS provider and in-memory record from the
// configuration file and swaps them into the database. Updates already in
// flight detect the swap and discard their results.
func (m *Manager) Reload() (warnings []string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.reload()
}

// reload is the same as Reload but expects the caller to hold the mutex.
func (m *Manager) reload() (warnings []string, err error) {
	if m.db == nil {
		return nil, errNoDatabase
	}

	providers, warnings, err := providersFromStore(m.store, m.logger)
	for _, warning := range warnings {
		m.logger.Warn(warning)
		m.notify.Notify(warning)
	}

	if err != nil {
		m.notify.Notify(err.Error())
		return warnings, err
	}

	allRecords, err := buildRecords(providers, m.history, m.logger, m.notify)
	if err != nil { // buildRecords already notified the user
		return warnings, err
	}

	m.db.Reload(allRecords)

	return warnings, nil
}

// mutate applies the change given to the settings file and hot reloads the
// resulting records. If the reload fails, the settings file is restored to the
// entries it had before the change, so the file and the running records stay
// in sync. The caller must have validated the settings entries already, so that
// an invalid entry never reaches the file.
func (m *Manager) mutate(validationWarnings []string,
	apply func(entries []json.RawMessage) (newEntries []json.RawMessage, err error),
) (result Result, err error,
) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	entries, _, err := m.store.Entries()
	if err != nil {
		return result, fmt.Errorf("%w: %w", errReadSettings, err)
	}
	previousEntries := copyEntries(entries)

	newEntries, err := apply(previousEntries)
	if err != nil {
		return result, err
	}

	err = m.store.Save(newEntries)
	if err != nil {
		return result, fmt.Errorf("%w: %w", errSaveSettings, err)
	}

	// The source is determined right after saving, because reloading may
	// synchronize the file from the environment variable, which would make the
	// environment variable look like it lost, or won twice.
	source := m.source(len(newEntries))

	reloadWarnings, err := m.reload()
	if err != nil {
		m.rollback(previousEntries)
		return result, err
	}

	allWarnings := make([]string, 0, len(validationWarnings)+len(reloadWarnings))
	allWarnings = append(allWarnings, validationWarnings...)
	allWarnings = append(allWarnings, reloadWarnings...)

	return m.result(source, allWarnings)
}

// rollback restores the settings entries the file had before a failed change.
// The in-memory database was not reloaded, so it still holds the records of
// these entries and does not need to be reloaded again.
func (m *Manager) rollback(previousEntries []json.RawMessage) {
	err := m.store.Save(previousEntries)
	if err != nil {
		message := errSaveSettings.Error() + " while rolling back: " + err.Error()
		m.logger.Error(message)
		m.notify.Notify(message)
		return
	}

	m.logger.Warn(errSettingsChanged.Error())
	m.notify.Notify(errSettingsChanged.Error())
}

// result builds the state of the settings configuration and of the records it
// currently produces, given the configuration source determined at save time.
func (m *Manager) result(source string, warnings []string) (result Result, err error) {
	entries, readWarnings, err := m.store.Entries()
	if err != nil {
		return result, fmt.Errorf("%w: %w", errReadSettings, err)
	}

	allWarnings := make([]string, 0, len(warnings)+len(readWarnings))
	allWarnings = append(allWarnings, warnings...)
	allWarnings = append(allWarnings, readWarnings...)

	return Result{
		Settings: nonNilEntries(entries),
		Warnings: nonNilWarnings(allWarnings),
		Source:   source,
		Env:      m.env(),
		FilePath: m.store.Filepath(),
		Records:  m.db.Count(),
	}, nil
}

// env returns "set" when the CONFIG environment variable is set to a non-empty
// value, and "unset" otherwise.
func (m *Manager) env() string {
	if os.Getenv(configEnvVar) == "" {
		return envUnset
	}

	return envSet
}

// source returns where the running configuration comes from. The CONFIG
// environment variable only wins over the file when it is set and differs from
// the value last synchronized to the file, which is proven by comparing its
// seed with the seed stamped in the file. Otherwise the file is the source of
// truth, since it may have been edited through the web UI in the meantime.
// A configuration with no settings entry at all yields "empty".
func (m *Manager) source(entriesCount int) string {
	envValue := os.Getenv(configEnvVar)
	if envValue == "" {
		return fileOrEmptySource(entriesCount)
	}

	if envSeedOfFile(m.store.Filepath()) != params.EnvSeedOf(envValue) {
		return sourceEnv
	}

	return fileOrEmptySource(entriesCount)
}

func fileOrEmptySource(entriesCount int) string {
	if entriesCount == 0 {
		return sourceEmpty
	}

	return sourceFile
}

// envSeedOfFile returns the seed of the CONFIG environment variable value the
// settings file was last synchronized from, read from its "_env_seed" object
// key. It returns an empty string if the file cannot be read or holds no seed,
// in which case the environment variable, if set, wins.
func envSeedOfFile(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	doc := struct {
		EnvSeed string `json:"_env_seed"`
	}{}
	err = json.Unmarshal(data, &doc)
	if err != nil {
		return ""
	}

	return doc.EnvSeed
}

func notFoundError(index, entriesCount int) error {
	return fmt.Errorf("%w: index %d for %d settings", ErrEntryNotFound, index, entriesCount)
}

func copyEntries(entries []json.RawMessage) []json.RawMessage {
	copied := make([]json.RawMessage, len(entries))
	copy(copied, entries)

	return copied
}

func nonNilEntries(entries []json.RawMessage) []json.RawMessage {
	if entries == nil {
		return []json.RawMessage{}
	}

	return entries
}

func nonNilWarnings(warnings []string) []string {
	if warnings == nil {
		return []string{}
	}

	return warnings
}
