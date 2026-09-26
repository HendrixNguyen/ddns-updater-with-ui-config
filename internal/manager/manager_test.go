package manager

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/internal/params"
	"github.com/qdm12/ddns-updater/internal/provider"
	recordslib "github.com/qdm12/ddns-updater/internal/records"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testFilePerm = 0o600
	entryDuckDNS = `{"provider":"duckdns","domain":"mydomain.duckdns.org",` +
		`"token":"00000000-0000-0000-0000-000000000000"}`
	entryDuckDNSOther = `{"provider":"duckdns","domain":"otherdomain.duckdns.org",` +
		`"token":"11111111-1111-1111-1111-111111111111"}`
	// entryDuckDNSRetroHost uses the deprecated "host" field, which produces a warning.
	entryDuckDNSRetroHost = `{"provider":"duckdns","domain":"mydomain.duckdns.org",` +
		`"host":"@","token":"00000000-0000-0000-0000-000000000000"}`
)

type testLogger struct {
	mutex  sync.Mutex
	infos  []string
	warns  []string
	debugs []string
	errors []string
}

func (l *testLogger) Info(s string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.infos = append(l.infos, s)
}

func (l *testLogger) Warn(s string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.warns = append(l.warns, s)
}

func (l *testLogger) Debug(s string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.debugs = append(l.debugs, s)
}

func (l *testLogger) Error(s string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.errors = append(l.errors, s)
}

func (l *testLogger) Warnings() []string {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return append([]string(nil), l.warns...)
}

func (l *testLogger) Errors() []string {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return append([]string(nil), l.errors...)
}

type testDatabase struct {
	mutex  sync.Mutex
	reload int
	counts []int
}

func (d *testDatabase) Reload(records []recordslib.Record) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.reload++
	d.counts = append(d.counts, len(records))
}

func (d *testDatabase) Count() int {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if len(d.counts) == 0 {
		return 0
	}
	return d.counts[len(d.counts)-1]
}

func (d *testDatabase) ReloadCount() int {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.reload
}

func (d *testDatabase) ReloadCounts() []int {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return append([]int(nil), d.counts...)
}

type testHistory struct {
	mutex  sync.Mutex
	events []models.HistoryEvent
	err    error
}

func (h *testHistory) GetEvents(_, _ string, _ ipversion.IPVersion) (
	events []models.HistoryEvent, err error,
) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if h.err != nil {
		return nil, h.err
	}
	return h.events, nil
}

type testNotifier struct {
	mutex         sync.Mutex
	notifications []string
}

func (n *testNotifier) Notify(s string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.notifications = append(n.notifications, s)
}

func (n *testNotifier) Notifications() []string {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	return append([]string(nil), n.notifications...)
}

type testSetup struct {
	manager  *Manager
	store    *params.Store
	database *testDatabase
	logger   *testLogger
	history  *testHistory
	notifier *testNotifier
}

func newTestSetup(t *testing.T, fileContent string) *testSetup {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "config.json")
	if fileContent != "" {
		require.NoError(t, os.WriteFile(filePath, []byte(fileContent), testFilePerm))
	}

	setup := &testSetup{
		store:    params.NewStore(&testLogger{}, filePath),
		database: &testDatabase{},
		logger:   &testLogger{},
		history:  &testHistory{},
		notifier: &testNotifier{},
	}
	setup.manager = New(setup.store, setup.history, setup.logger, setup.notifier)
	setup.manager.AttachDatabase(setup.database)

	return setup
}

// fileSettings returns the settings entries currently stored in the settings
// file, along with the raw file content.
func (s *testSetup) fileSettings(t *testing.T) (entries []string, content string) {
	t.Helper()

	data, err := os.ReadFile(s.store.Filepath())
	require.NoError(t, err)

	doc := struct {
		Settings []json.RawMessage `json:"settings"`
	}{}
	require.NoError(t, json.Unmarshal(data, &doc))

	entries = make([]string, len(doc.Settings))
	for i, entry := range doc.Settings {
		// The store indents the JSON document it writes, so each entry is
		// compacted back to compare it with the constant it comes from.
		buffer := &bytes.Buffer{}
		require.NoError(t, json.Compact(buffer, entry))
		entries[i] = buffer.String()
	}

	return entries, string(data)
}

var errHistoryUnreadable = errors.New("cannot read the IP history")

func testHistoryEvents() []models.HistoryEvent {
	return []models.HistoryEvent{
		{IP: netip.MustParseAddr("1.2.3.4"), Time: time.Unix(1, 0).UTC()},
	}
}

func Test_Providers(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileContent    string
		providersCount int
		warningsCount  int
		errWrapped     error
		errAny         bool
	}{
		"single_setting": {
			fileContent:    `{"settings":[` + entryDuckDNS + `]}`,
			providersCount: 1,
		},
		"several_settings": {
			fileContent:    `{"settings":[` + entryDuckDNS + `,` + entryDuckDNSOther + `]}`,
			providersCount: 2,
		},
		"warning_propagates": {
			fileContent:    `{"settings":[` + entryDuckDNSRetroHost + `]}`,
			providersCount: 1,
			warningsCount:  1,
		},
		"no_settings": {
			fileContent:    `{"settings":[]}`,
			providersCount: 0,
		},
		"unknown_provider": {
			fileContent: `{"settings":[{"provider":"unknown","domain":"example.com"}]}`,
			errAny:      true,
		},
		"malformed_json": {
			fileContent: `{"settings":[`,
			errAny:      true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := params.NewStore(&testLogger{}, filepath.Join(t.TempDir(), "config.json"))
			require.NoError(t, os.WriteFile(store.Filepath(),
				[]byte(testCase.fileContent), testFilePerm))

			providers, warnings, err := Providers(store)

			if testCase.errAny {
				require.Error(t, err)
				if testCase.errWrapped != nil {
					assert.ErrorIs(t, err, testCase.errWrapped)
				}
				assert.Nil(t, providers)
				return
			}

			require.NoError(t, err)
			assert.Len(t, providers, testCase.providersCount)
			assert.Len(t, warnings, testCase.warningsCount)
		})
	}
}

func Test_BuildRecords(t *testing.T) {
	t.Parallel()

	store := params.NewStore(&testLogger{}, filepath.Join(t.TempDir(), "config.json"))
	require.NoError(t, os.WriteFile(store.Filepath(),
		[]byte(`{"settings":[`+entryDuckDNS+`,`+entryDuckDNSOther+`]}`), testFilePerm))

	providers, warnings, err := Providers(store)
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Len(t, providers, 2)

	events := testHistoryEvents()
	history := &testHistory{events: events}

	allRecords, err := BuildRecords(providers, history)
	require.NoError(t, err)
	require.Len(t, allRecords, 2)

	for i, record := range allRecords {
		assert.Equal(t, providers[i], record.Provider)
		assert.Equal(t, models.History(events), record.History)
	}
}

func Test_Manager_mutations(t *testing.T) {
	t.Parallel()

	const threeSettings = `{"settings":[` + entryDuckDNS + `,` + entryDuckDNSOther +
		`,` + entryDuckDNS + `]}`

	testCases := map[string]struct {
		fileContent       string
		mutate            func(m *Manager) (result Result, err error)
		expectedSettings  []string
		expectedRecordNum int
	}{
		"add_entry": {
			fileContent:       `{"settings":[` + entryDuckDNS + `]}`,
			mutate:            func(m *Manager) (Result, error) { return m.AddEntry(json.RawMessage(entryDuckDNSOther)) },
			expectedSettings:  []string{entryDuckDNS, entryDuckDNSOther},
			expectedRecordNum: 2,
		},
		"replace_entry": {
			fileContent: `{"settings":[` + entryDuckDNS + `,` + entryDuckDNSOther + `]}`,
			mutate: func(m *Manager) (Result, error) {
				return m.ReplaceEntry(0, json.RawMessage(entryDuckDNSOther))
			},
			expectedSettings:  []string{entryDuckDNSOther, entryDuckDNSOther},
			expectedRecordNum: 2,
		},
		"delete_entry": {
			fileContent:       threeSettings,
			mutate:            func(m *Manager) (Result, error) { return m.DeleteEntry(1) },
			expectedSettings:  []string{entryDuckDNS, entryDuckDNS},
			expectedRecordNum: 2,
		},
		"delete_last_entry": {
			fileContent:       `{"settings":[` + entryDuckDNS + `]}`,
			mutate:            func(m *Manager) (Result, error) { return m.DeleteEntry(0) },
			expectedSettings:  []string{},
			expectedRecordNum: 0,
		},
		"replace_all_entries": {
			fileContent: threeSettings,
			mutate: func(m *Manager) (Result, error) {
				return m.ReplaceAllEntries([]json.RawMessage{json.RawMessage(entryDuckDNSOther)})
			},
			expectedSettings:  []string{entryDuckDNSOther},
			expectedRecordNum: 1,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			setup := newTestSetup(t, testCase.fileContent)

			result, err := testCase.mutate(setup.manager)
			require.NoError(t, err)

			// The file on disk is updated.
			settings, _ := setup.fileSettings(t)
			assert.Equal(t, testCase.expectedSettings, settings)

			// The database is reloaded exactly once, with the right record count.
			assert.Equal(t, 1, setup.database.ReloadCount())
			assert.Equal(t, []int{testCase.expectedRecordNum}, setup.database.ReloadCounts())

			// The result always has non-nil slices and matches the database.
			require.NotNil(t, result.Settings)
			require.NotNil(t, result.Warnings)
			assert.Len(t, result.Settings, len(testCase.expectedSettings))
			assert.Equal(t, testCase.expectedRecordNum, result.Records)
			assert.Equal(t, setup.store.Filepath(), result.FilePath)
			assert.Equal(t, envUnset, result.Env)
			if len(testCase.expectedSettings) == 0 {
				assert.Equal(t, sourceEmpty, result.Source)
			} else {
				assert.Equal(t, sourceFile, result.Source)
			}
		})
	}
}

func Test_Manager_AddEntry_invalid(t *testing.T) {
	t.Parallel()

	const fileContent = `{"settings":[` + entryDuckDNS + `]}`

	testCases := map[string]struct {
		entry      string
		errWrapped error
	}{
		"not_a_json_object": {
			entry:      `[{"provider":"duckdns"}]`,
			errWrapped: params.ErrEntryNotObject,
		},
		"not_json": {
			entry:      `{"provider":`,
			errWrapped: params.ErrEntryInvalidJSON,
		},
		"no_provider": {
			entry:      `{"provider":"unknown","domain":"example.com"}`,
			errWrapped: provider.ErrProviderUnknown,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			setup := newTestSetup(t, fileContent)
			_, previousContent := setup.fileSettings(t)

			result, err := setup.manager.AddEntry(json.RawMessage(testCase.entry))

			require.Error(t, err)
			assert.True(t, IsValidationError(err))
			require.ErrorIs(t, err, testCase.errWrapped)
			assert.Equal(t, Result{}, result)

			// The file is untouched and the database is not reloaded.
			settings, content := setup.fileSettings(t)
			assert.Equal(t, []string{entryDuckDNS}, settings)
			assert.Equal(t, previousContent, content)
			assert.Equal(t, 0, setup.database.ReloadCount())
		})
	}
}

func Test_Manager_indexOutOfRange(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileContent string
		mutate      func(m *Manager) (result Result, err error)
	}{
		"replace_too_high": {
			mutate: func(m *Manager) (Result, error) {
				return m.ReplaceEntry(5, json.RawMessage(entryDuckDNS))
			},
		},
		"replace_negative": {
			mutate: func(m *Manager) (Result, error) {
				return m.ReplaceEntry(-1, json.RawMessage(entryDuckDNS))
			},
		},
		"delete_too_high": {
			mutate: func(m *Manager) (Result, error) { return m.DeleteEntry(5) },
		},
		"delete_negative": {
			mutate: func(m *Manager) (Result, error) { return m.DeleteEntry(-1) },
		},
		"delete_from_empty_settings": {
			fileContent: `{"settings":[]}`,
			mutate:      func(m *Manager) (Result, error) { return m.DeleteEntry(0) },
		},
		"replace_into_empty_settings": {
			fileContent: `{"settings":[]}`,
			mutate: func(m *Manager) (Result, error) {
				return m.ReplaceEntry(0, json.RawMessage(entryDuckDNS))
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fileContent := testCase.fileContent
			if fileContent == "" {
				fileContent = `{"settings":[` + entryDuckDNS + `]}`
			}

			setup := newTestSetup(t, fileContent)

			_, err := testCase.mutate(setup.manager)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrEntryNotFound)
			assert.ErrorIs(t, err, params.ErrEntryNotFound)
			assert.False(t, IsValidationError(err))
			assert.Equal(t, 0, setup.database.ReloadCount())
		})
	}
}

func Test_Manager_rollback(t *testing.T) {
	t.Parallel()

	const fileContent = `{"settings":[` + entryDuckDNS + `]}`
	setup := newTestSetup(t, fileContent)
	_, previousContent := setup.fileSettings(t)

	setup.history.err = errHistoryUnreadable

	result, err := setup.manager.AddEntry(json.RawMessage(entryDuckDNSOther))

	require.ErrorIs(t, err, errHistoryUnreadable)
	assert.Equal(t, Result{}, result)

	// The file is restored to its previous content, modulo the
	// indentation the store always writes the JSON document with.
	settings, content := setup.fileSettings(t)
	assert.Equal(t, []string{entryDuckDNS}, settings)
	assert.JSONEq(t, previousContent, content)

	// The database still holds the records of the previous settings.
	assert.Equal(t, 0, setup.database.ReloadCount())
	assert.Equal(t, 0, setup.database.Count())

	// The user is warned about the rollback.
	assert.Equal(t, []string{errSettingsChanged.Error()}, setup.logger.Warnings())
	assert.Contains(t, setup.notifier.Notifications(), errSettingsChanged.Error())
}

func Test_Manager_Reload_warnings(t *testing.T) {
	t.Parallel()

	setup := newTestSetup(t, `{"settings":[`+entryDuckDNSRetroHost+`]}`)

	warnings, err := setup.manager.Reload()
	require.NoError(t, err)
	require.Len(t, warnings, 1)

	assert.Equal(t, warnings, setup.logger.Warnings())
	assert.Equal(t, warnings, setup.notifier.Notifications())
	assert.Equal(t, 1, setup.database.ReloadCount())
	assert.Equal(t, []int{1}, setup.database.ReloadCounts())
}

func Test_Manager_Reload_historyError(t *testing.T) {
	t.Parallel()

	setup := newTestSetup(t, `{"settings":[`+entryDuckDNS+`]}`)
	setup.history.err = errHistoryUnreadable

	warnings, err := setup.manager.Reload()

	require.ErrorIs(t, err, errHistoryUnreadable)
	assert.Empty(t, warnings)
	assert.Equal(t, 0, setup.database.ReloadCount())
	assert.Equal(t, []string{errHistoryUnreadable.Error()}, setup.notifier.Notifications())
}

func Test_Manager_Reload_noDatabase(t *testing.T) {
	t.Parallel()

	store := params.NewStore(&testLogger{}, filepath.Join(t.TempDir(), "config.json"))
	require.NoError(t, os.WriteFile(store.Filepath(),
		[]byte(`{"settings":[`+entryDuckDNS+`]}`), testFilePerm))
	manager := New(store, &testHistory{}, &testLogger{}, &testNotifier{})

	warnings, err := manager.Reload()

	require.Error(t, err)
	assert.ErrorIs(t, err, errNoDatabase)
	assert.Empty(t, warnings)
}

func Test_Manager_AttachDatabase_twice(t *testing.T) {
	t.Parallel()

	setup := newTestSetup(t, `{"settings":[`+entryDuckDNS+`]}`)

	assert.Panics(t, func() {
		setup.manager.AttachDatabase(&testDatabase{})
	})
}

func Test_Manager_Entries(t *testing.T) {
	t.Parallel()

	t.Run("non_nil_when_empty", func(t *testing.T) {
		t.Parallel()

		setup := newTestSetup(t, `{"settings":[]}`)

		entries, warnings, err := setup.manager.Entries()
		require.NoError(t, err)
		assert.NotNil(t, entries)
		assert.NotNil(t, warnings)
		assert.Empty(t, entries)
		assert.Empty(t, warnings)
	})

	t.Run("entries_are_returned", func(t *testing.T) {
		t.Parallel()

		setup := newTestSetup(t, `{"settings":[`+entryDuckDNS+`]}`)

		entries, warnings, err := setup.manager.Entries()
		require.NoError(t, err)
		assert.Equal(t, []json.RawMessage{json.RawMessage(entryDuckDNS)}, entries)
		assert.Empty(t, warnings)
	})
}

func Test_Manager_Validate(t *testing.T) {
	t.Parallel()

	setup := newTestSetup(t, `{"settings":[]}`)

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		warnings, err := setup.manager.Validate(
			[]json.RawMessage{json.RawMessage(entryDuckDNS)})

		require.NoError(t, err)
		assert.NotNil(t, warnings)
		assert.Empty(t, warnings)
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		warnings, err := setup.manager.Validate(
			[]json.RawMessage{json.RawMessage(`{"provider":"unknown"}`)})

		require.Error(t, err)
		assert.True(t, IsValidationError(err))
		assert.NotNil(t, warnings)
	})
}

func Test_Manager_AddEntry_validationWarning(t *testing.T) {
	t.Parallel()

	setup := newTestSetup(t, `{"settings":[]}`)

	result, err := setup.manager.AddEntry(json.RawMessage(entryDuckDNSRetroHost))
	require.NoError(t, err)

	// The warning is reported once for the validation of the entry and once for
	// the reload of the whole configuration, and the reload one is notified.
	require.Len(t, result.Warnings, 2)
	assert.Equal(t, result.Warnings[0], result.Warnings[1])
	assert.Equal(t, []string{result.Warnings[0]}, setup.notifier.Notifications())
	assert.Equal(t, []string{result.Warnings[0]}, setup.logger.Warnings())
}

func Test_Result_marshalsEmptyArrays(t *testing.T) {
	t.Parallel()

	setup := newTestSetup(t, `{"settings":[`+entryDuckDNS+`]}`)

	result, err := setup.manager.ReplaceAllEntries(nil)
	require.NoError(t, err)

	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.JSONEq(t, `{"settings":[],"warnings":[],"source":"empty",`+
		`"env":"unset","filePath":"`+setup.store.Filepath()+`","records":0}`, string(data))
}

func Test_IsValidationError(t *testing.T) {
	t.Parallel()

	assert.False(t, IsValidationError(nil))
	assert.False(t, IsValidationError(errHistoryUnreadable))
	assert.True(t, IsValidationError(&ValidationError{Err: errHistoryUnreadable}))
}

func Test_manager_sourceAndEnv(t *testing.T) { //nolint:paralleltest // t.Setenv is incompatible with t.Parallel.
	t.Run("env_set_and_winning", func(t *testing.T) {
		const envValue = `{"settings":[` + entryDuckDNS + `]}`
		t.Setenv(configEnvVar, envValue)

		setup := newTestSetup(t, `{"settings":[`+entryDuckDNSRetroHost+`]}`)

		result, err := setup.manager.AddEntry(json.RawMessage(entryDuckDNS))
		require.NoError(t, err)

		assert.Equal(t, envSet, result.Env)
		assert.Equal(t, sourceEnv, result.Source)
	})

	t.Run("env_set_but_unchanged_file_wins", func(t *testing.T) {
		const envValue = `{"settings":[` + entryDuckDNS + `]}`
		t.Setenv(configEnvVar, envValue)
		seed := params.EnvSeedOf(envValue)

		setup := newTestSetup(t, `{"settings":[`+entryDuckDNS+`],"_env_seed":"`+seed+`"}`)

		result, err := setup.manager.AddEntry(json.RawMessage(entryDuckDNSOther))
		require.NoError(t, err)

		assert.Equal(t, envSet, result.Env)
		assert.Equal(t, sourceFile, result.Source)
		assert.Len(t, result.Settings, 2)
	})

	t.Run("env_unset_with_settings", func(t *testing.T) {
		t.Setenv(configEnvVar, "")

		setup := newTestSetup(t, `{"settings":[`+entryDuckDNS+`]}`)

		result, err := setup.manager.AddEntry(json.RawMessage(entryDuckDNSOther))
		require.NoError(t, err)

		assert.Equal(t, envUnset, result.Env)
		assert.Equal(t, sourceFile, result.Source)
	})

	t.Run("env_unset_without_settings", func(t *testing.T) {
		t.Setenv(configEnvVar, "")

		setup := newTestSetup(t, `{"settings":[]}`)

		result, err := setup.manager.AddEntry(json.RawMessage(entryDuckDNS))
		require.NoError(t, err)

		assert.Equal(t, envUnset, result.Env)
		assert.Equal(t, sourceFile, result.Source)
	})

	t.Run("env_unset_and_no_entry", func(t *testing.T) {
		t.Setenv(configEnvVar, "")

		setup := newTestSetup(t, `{"settings":[]}`)

		result, err := setup.manager.DeleteEntry(0)
		require.ErrorIs(t, err, ErrEntryNotFound)
		assert.Equal(t, Result{}, result)

		assert.Equal(t, envUnset, setup.manager.env())
		assert.Equal(t, sourceEmpty, setup.manager.source(0))
	})
}
