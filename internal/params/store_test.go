package params

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/qdm12/ddns-updater/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testFilePerm    = fs.FileMode(0o600)
	entryCloudflare = `{"provider":"cloudflare","domain":"sub.example.com",` +
		`"token":"token","zone_identifier":"zone","ttl":600}`
	entryDuckDNS = `{"provider":"duckdns","domain":"mydomain.duckdns.org",` +
		`"token":"00000000-0000-0000-0000-000000000000"}`
)

type testLogger struct {
	mutex sync.Mutex
	infos []string
}

func (l *testLogger) Info(s string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.infos = append(l.infos, s)
}

func (l *testLogger) Debug(string) {}

func (l *testLogger) Infos() []string {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	return append([]string(nil), l.infos...)
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "config.json")

	return NewStore(&testLogger{}, filePath)
}

func newTestStoreWithFile(t *testing.T, content string) *Store {
	t.Helper()

	store := newTestStore(t)
	require.NoError(t, os.WriteFile(store.Filepath(), []byte(content), testFilePerm))

	return store
}

func toRawMessages(t *testing.T, entries ...string) []json.RawMessage {
	t.Helper()

	rawMessages := make([]json.RawMessage, len(entries))
	for i, entry := range entries {
		rawMessages[i] = json.RawMessage(entry)
	}

	return rawMessages
}

// readTestDocument reads the settings file and checks it is a JSON object
// ending with a newline, as written by the store.
func readTestDocument(t *testing.T, filePath string) map[string]json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(filePath) //nolint:gosec // test file path.
	require.NoError(t, err)
	assert.True(t, string(data[len(data)-1:]) == "\n",
		"settings file should end with a newline")

	parsed := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal(data, &parsed))

	return parsed
}

// compactJSON compacts the given JSON text, since the store indents the whole
// document it writes, including each of its settings entries.
func compactJSON(t *testing.T, data string) string {
	t.Helper()

	buffer := &bytes.Buffer{}
	require.NoError(t, json.Compact(buffer, []byte(data)))

	return buffer.String()
}

// settingsOf returns the compacted settings entries of the given JSON document.
func settingsOf(t *testing.T, data []byte) []string {
	t.Helper()

	doc := document{}
	require.NoError(t, json.Unmarshal(data, &doc))

	return entryStrings(t, doc.Settings)
}

func entryStrings(t *testing.T, entries []json.RawMessage) []string {
	t.Helper()

	entryStrings := make([]string, len(entries))
	for i, entry := range entries {
		entryStrings[i] = compactJSON(t, string(entry))
	}

	return entryStrings
}

func Test_NewStore(t *testing.T) {
	t.Parallel()

	store := NewStore(nil, "/some/path/config.json")

	assert.Equal(t, "/some/path/config.json", store.Filepath())
	store.logger.Info("logging with a nil logger does not panic")
}

func Test_Store_Entries(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileMissing bool
		fileContent string
		entries     []json.RawMessage
		warnings    []string
		errWrapped  error
	}{
		"file_missing": {
			fileMissing: true,
		},
		"empty_object": {
			fileContent: `{}`,
		},
		"null_settings": {
			fileContent: `{"settings":null}`,
		},
		"empty_settings": {
			fileContent: `{"settings":[]}`,
		},
		"blank_file": {
			fileContent: "  \n",
		},
		"unknown_top_level_field": {
			fileContent: `{"unknown":{"a":1},"settings":[` + entryDuckDNS + `]}`,
			entries:     []json.RawMessage{json.RawMessage(entryDuckDNS)},
		},
		"two_settings": {
			fileContent: `{"settings":[` + entryCloudflare + `,` + entryDuckDNS + `]}`,
			entries: []json.RawMessage{
				json.RawMessage(entryCloudflare), json.RawMessage(entryDuckDNS)},
		},
		"malformed_json": {
			fileContent: `{"settings":`,
			errWrapped:  errUnmarshalRaw,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var store *Store
			if testCase.fileMissing {
				store = newTestStore(t)
			} else {
				store = newTestStoreWithFile(t, testCase.fileContent)
			}

			entries, warnings, err := store.Entries()

			assert.ErrorIs(t, err, testCase.errWrapped)
			assert.Equal(t, testCase.entries, entries)
			assert.Equal(t, testCase.warnings, warnings)
		})
	}
}

func Test_Store_Save(t *testing.T) {
	t.Parallel()

	t.Run("writes_settings", func(t *testing.T) {
		t.Parallel()

		store := newTestStore(t)

		err := store.Save(toRawMessages(t, entryDuckDNS))

		require.NoError(t, err)
		parsed := readTestDocument(t, store.Filepath())
		assert.Equal(t, `[`+entryDuckDNS+`]`, compactJSON(t, string(parsed[settingsKey])))
		assert.NotContains(t, parsed, envSeedKey)
	})

	t.Run("writes_empty_settings", func(t *testing.T) {
		t.Parallel()

		store := newTestStore(t)

		err := store.Save(nil)

		require.NoError(t, err)
		parsed := readTestDocument(t, store.Filepath())
		assert.Equal(t, `[]`, compactJSON(t, string(parsed[settingsKey])))
	})

	t.Run("preserves_env_seed", func(t *testing.T) {
		t.Parallel()

		store := newTestStoreWithFile(t, `{"_env_seed":"abcd","settings":[]}`)

		err := store.Save(toRawMessages(t, entryDuckDNS))

		require.NoError(t, err)
		parsed := readTestDocument(t, store.Filepath())
		assert.Equal(t, `"abcd"`, string(parsed[envSeedKey]))
		assert.Equal(t, `[`+entryDuckDNS+`]`, compactJSON(t, string(parsed[settingsKey])))
	})

	t.Run("preserves_unknown_fields", func(t *testing.T) {
		t.Parallel()

		store := newTestStoreWithFile(t, `{"unknown":[1,2],"settings":[]}`)

		err := store.Save(toRawMessages(t, entryDuckDNS))

		require.NoError(t, err)
		parsed := readTestDocument(t, store.Filepath())
		assert.Equal(t, `[1,2]`, compactJSON(t, string(parsed["unknown"])))
		assert.Equal(t, `[`+entryDuckDNS+`]`, compactJSON(t, string(parsed[settingsKey])))
	})

	t.Run("save_with_seed_overrides_env_seed", func(t *testing.T) {
		t.Parallel()

		store := newTestStoreWithFile(t, `{"_env_seed":"abcd","settings":[]}`)

		err := store.SaveWithSeed(toRawMessages(t, entryDuckDNS), "efgh")

		require.NoError(t, err)
		parsed := readTestDocument(t, store.Filepath())
		assert.Equal(t, `"efgh"`, string(parsed[envSeedKey]))
	})

	t.Run("save_with_empty_seed_removes_env_seed", func(t *testing.T) {
		t.Parallel()

		store := newTestStoreWithFile(t, `{"_env_seed":"abcd","settings":[]}`)

		err := store.SaveWithSeed(toRawMessages(t, entryDuckDNS), "")

		require.NoError(t, err)
		parsed := readTestDocument(t, store.Filepath())
		assert.NotContains(t, parsed, envSeedKey)
	})

	t.Run("env_seed_survives_crud_operations", func(t *testing.T) {
		t.Parallel()

		const seed = "0123456789abcdef"
		store := newTestStoreWithFile(t,
			`{"_env_seed":"`+seed+`","settings":[`+entryCloudflare+`]}`)

		_, err := store.Add(json.RawMessage(entryDuckDNS))
		require.NoError(t, err)
		_, err = store.Replace(0, json.RawMessage(entryDuckDNS))
		require.NoError(t, err)
		_, err = store.Delete(0)
		require.NoError(t, err)
		_, err = store.ReplaceAll(toRawMessages(t, entryCloudflare))
		require.NoError(t, err)

		parsed := readTestDocument(t, store.Filepath())
		assert.Equal(t, `"`+seed+`"`, string(parsed[envSeedKey]))
		assert.Equal(t, `[`+entryCloudflare+`]`, compactJSON(t, string(parsed[settingsKey])))
	})
}

func Test_Store_writeAtomic(t *testing.T) {
	t.Parallel()

	t.Run("writes_temp_file_and_renames_it", func(t *testing.T) {
		t.Parallel()

		var writtenPaths, renamedFrom, renamedTo, removedPaths []string
		store := newTestStore(t)
		store.writeFile = func(filename string, _ []byte, _ fs.FileMode) error {
			writtenPaths = append(writtenPaths, filename)
			return nil
		}
		store.rename = func(oldpath, newpath string) error {
			renamedFrom = append(renamedFrom, oldpath)
			renamedTo = append(renamedTo, newpath)
			return nil
		}
		store.remove = func(name string) error {
			removedPaths = append(removedPaths, name)
			return nil
		}

		err := store.Save(toRawMessages(t, entryDuckDNS))

		require.NoError(t, err)
		tempPath := store.Filepath() + tempFileSuffix
		assert.Equal(t, []string{tempPath}, writtenPaths)
		assert.Equal(t, []string{tempPath}, renamedFrom)
		assert.Equal(t, []string{store.Filepath()}, renamedTo)
		assert.Empty(t, removedPaths)
	})

	t.Run("removes_temp_file_when_rename_fails", func(t *testing.T) {
		t.Parallel()

		var removedPaths []string
		store := newTestStore(t)
		store.writeFile = func(string, []byte, fs.FileMode) error { return nil }
		store.rename = func(string, string) error { return errors.New("rename error") }
		store.remove = func(name string) error {
			removedPaths = append(removedPaths, name)
			return nil
		}

		err := store.Save(toRawMessages(t, entryDuckDNS))

		assert.ErrorIs(t, err, errRenameTempFile)
		assert.Equal(t, []string{store.Filepath() + tempFileSuffix}, removedPaths)
	})

	t.Run("keeps_temp_file_removal_failure_ignored", func(t *testing.T) {
		t.Parallel()

		store := newTestStore(t)
		store.writeFile = func(string, []byte, fs.FileMode) error { return nil }
		store.rename = func(string, string) error { return errors.New("rename error") }
		store.remove = func(string) error { return errors.New("remove error") }

		err := store.Save(toRawMessages(t, entryDuckDNS))

		assert.ErrorIs(t, err, errRenameTempFile)
	})

	t.Run("write_error", func(t *testing.T) {
		t.Parallel()

		var renamed bool
		store := newTestStore(t)
		store.writeFile = func(string, []byte, fs.FileMode) error {
			return errors.New("write error")
		}
		store.rename = func(string, string) error {
			renamed = true
			return nil
		}

		err := store.Save(toRawMessages(t, entryDuckDNS))

		assert.ErrorIs(t, err, errWriteTempFile)
		assert.False(t, renamed)
	})

	t.Run("logs_outcome", func(t *testing.T) {
		t.Parallel()

		logger := &testLogger{}
		filePath := filepath.Join(t.TempDir(), "config.json")
		store := NewStore(logger, filePath)

		err := store.Save(toRawMessages(t, entryDuckDNS))

		require.NoError(t, err)
		assert.Contains(t, logger.Infos(), "settings file written to "+filePath)
	})

	t.Run("keeps_the_permissions_of_the_file_it_replaces", func(t *testing.T) {
		t.Parallel()

		const secretPerm = fs.FileMode(0o600)
		filePath := filepath.Join(t.TempDir(), "config.json")
		require.NoError(t, os.WriteFile(filePath, []byte(`{}`), secretPerm))
		store := NewStore(nil, filePath)

		require.NoError(t, store.Save(toRawMessages(t, entryDuckDNS)))

		info, err := os.Stat(filePath)
		require.NoError(t, err)
		assert.Equal(t, secretPerm, info.Mode().Perm())
	})

	t.Run("uses_default_permissions_when_there_is_no_file_yet", func(t *testing.T) {
		t.Parallel()

		var writtenPerm fs.FileMode
		store := newTestStore(t)
		store.writeFile = func(_ string, _ []byte, perm fs.FileMode) error {
			writtenPerm = perm
			return nil
		}
		store.rename = func(string, string) error { return nil }

		require.NoError(t, store.Save(toRawMessages(t, entryDuckDNS)))

		assert.Equal(t, fs.FileMode(0o666), writtenPerm)
	})
}

func Test_Store_Add(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileMissing bool
		fileContent string
		entry       string
		entries     []string
		errWrapped  error
	}{
		"add_to_missing_file": {
			fileMissing: true,
			entry:       entryDuckDNS,
			entries:     []string{entryDuckDNS},
		},
		"add_to_empty_file": {
			fileContent: `{}`,
			entry:       entryCloudflare,
			entries:     []string{entryCloudflare},
		},
		"append_to_existing": {
			fileContent: `{"settings":[` + entryCloudflare + `]}`,
			entry:       entryDuckDNS,
			entries:     []string{entryCloudflare, entryDuckDNS},
		},
		"malformed_file": {
			fileContent: `nope`,
			entry:       entryDuckDNS,
			errWrapped:  errUnmarshalRaw,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var store *Store
			if testCase.fileMissing {
				store = newTestStore(t)
			} else {
				store = newTestStoreWithFile(t, testCase.fileContent)
			}

			entries, err := store.Add(json.RawMessage(testCase.entry))

			assert.ErrorIs(t, err, testCase.errWrapped)
			if testCase.errWrapped != nil {
				return
			}
			assert.Equal(t, testCase.entries, entryStrings(t, entries))
			assert.Equal(t, testCase.entries,
				settingsOf(t, mustReadFile(t, store.Filepath())))
		})
	}
}

func Test_Store_Replace(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileContent string
		index       int
		entry       string
		entries     []string
		errWrapped  error
	}{
		"replace_first": {
			fileContent: `{"settings":[` + entryCloudflare + `,` + entryDuckDNS + `]}`,
			index:       0,
			entry:       entryDuckDNS,
			entries:     []string{entryDuckDNS, entryDuckDNS},
		},
		"replace_last": {
			fileContent: `{"settings":[` + entryCloudflare + `,` + entryDuckDNS + `]}`,
			index:       1,
			entry:       entryCloudflare,
			entries:     []string{entryCloudflare, entryCloudflare},
		},
		"replace_with_empty_file": {
			fileContent: `{}`,
			index:       0,
			entry:       entryDuckDNS,
			errWrapped:  ErrEntryNotFound,
		},
		"index_out_of_range": {
			fileContent: `{"settings":[` + entryCloudflare + `]}`,
			index:       1,
			entry:       entryDuckDNS,
			errWrapped:  ErrEntryNotFound,
		},
		"negative_index": {
			fileContent: `{"settings":[` + entryCloudflare + `]}`,
			index:       -1,
			entry:       entryDuckDNS,
			errWrapped:  ErrEntryNotFound,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newTestStoreWithFile(t, testCase.fileContent)

			entries, err := store.Replace(testCase.index, json.RawMessage(testCase.entry))

			assert.ErrorIs(t, err, testCase.errWrapped)
			if testCase.errWrapped != nil {
				// the file must be left untouched.
				assert.Equal(t, testCase.fileContent,
					string(mustReadFile(t, store.Filepath())))
				return
			}
			assert.Equal(t, testCase.entries, entryStrings(t, entries))
			assert.Equal(t, testCase.entries,
				settingsOf(t, mustReadFile(t, store.Filepath())))
		})
	}
}

func Test_Store_Delete(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileContent string
		index       int
		entries     []string
		errWrapped  error
	}{
		"delete_first": {
			fileContent: `{"settings":[` + entryCloudflare + `,` + entryDuckDNS + `]}`,
			index:       0,
			entries:     []string{entryDuckDNS},
		},
		"delete_last": {
			fileContent: `{"settings":[` + entryCloudflare + `,` + entryDuckDNS + `]}`,
			index:       1,
			entries:     []string{entryCloudflare},
		},
		"delete_only": {
			fileContent: `{"settings":[` + entryCloudflare + `]}`,
			index:       0,
			entries:     []string{},
		},
		"index_out_of_range": {
			fileContent: `{"settings":[` + entryCloudflare + `]}`,
			index:       3,
			errWrapped:  ErrEntryNotFound,
		},
		"empty_file": {
			fileContent: `{}`,
			index:       0,
			errWrapped:  ErrEntryNotFound,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newTestStoreWithFile(t, testCase.fileContent)

			entries, err := store.Delete(testCase.index)

			assert.ErrorIs(t, err, testCase.errWrapped)
			if testCase.errWrapped != nil {
				assert.Equal(t, testCase.fileContent,
					string(mustReadFile(t, store.Filepath())))
				return
			}
			assert.Equal(t, testCase.entries, entryStrings(t, entries))
			assert.Equal(t, testCase.entries,
				settingsOf(t, mustReadFile(t, store.Filepath())))
		})
	}
}

func mustReadFile(t *testing.T, filePath string) []byte {
	t.Helper()

	data, err := os.ReadFile(filePath) //nolint:gosec // test file path.
	require.NoError(t, err)

	return data
}

func Test_Store_ReplaceAll(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileMissing bool
		fileContent string
		entries     []json.RawMessage
		saved       []json.RawMessage
		fileEntries []string
		errWrapped  error
	}{
		"missing_file": {
			fileMissing: true,
			entries:     []json.RawMessage{json.RawMessage(entryDuckDNS)},
			saved:       []json.RawMessage{json.RawMessage(entryDuckDNS)},
			fileEntries: []string{entryDuckDNS},
		},
		"replace_with_two": {
			fileContent: `{"settings":[` + entryCloudflare + `]}`,
			entries: []json.RawMessage{
				json.RawMessage(entryDuckDNS), json.RawMessage(entryCloudflare)},
			saved: []json.RawMessage{
				json.RawMessage(entryDuckDNS), json.RawMessage(entryCloudflare)},
			fileEntries: []string{entryDuckDNS, entryCloudflare},
		},
		"empty_slice": {
			fileContent: `{"settings":[` + entryCloudflare + `]}`,
			entries:     []json.RawMessage{},
			saved:       []json.RawMessage{},
			fileEntries: []string{},
		},
		"nil_slice": {
			fileContent: `{"settings":[` + entryCloudflare + `]}`,
			entries:     nil,
			saved:       nil,
			fileEntries: []string{},
		},
		"malformed_file": {
			fileContent: `nope`,
			entries:     toRawMessages(t, entryDuckDNS),
			errWrapped:  errUnmarshalRaw,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var store *Store
			if testCase.fileMissing {
				store = newTestStore(t)
			} else {
				store = newTestStoreWithFile(t, testCase.fileContent)
			}

			saved, err := store.ReplaceAll(testCase.entries)

			assert.ErrorIs(t, err, testCase.errWrapped)
			if testCase.errWrapped != nil {
				return
			}
			assert.Equal(t, testCase.saved, saved)
			fileData := mustReadFile(t, store.Filepath())
			assert.Equal(t, testCase.fileEntries, settingsOf(t, fileData))
		})
	}
}

func Test_Store_concurrentAccess(t *testing.T) {
	t.Parallel()

	const goroutines = 8
	store := newTestStore(t)
	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines * 2)

	for range goroutines {
		go func() {
			defer waitGroup.Done()
			_, err := store.Add(json.RawMessage(entryDuckDNS))
			assert.NoError(t, err)
		}()
		go func() {
			defer waitGroup.Done()
			_, _, err := store.Entries()
			assert.NoError(t, err)
		}()
	}

	waitGroup.Wait()

	entries, _, err := store.Entries()
	require.NoError(t, err)
	assert.Len(t, entries, goroutines)
}

func Test_ValidateEntry(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		entry       string
		warnings    []string
		errWrapped  error
		errContains string
	}{
		"valid_cloudflare": {
			entry: entryCloudflare,
		},
		"valid_with_deprecated_field_warning": {
			entry: `{"provider":"cloudflare","domain":"sub.example.com","token":"token",` +
				`"zone_identifier":"zone","ttl":600,"provider_ip":true}`,
			warnings: []string{`for domain example.com and ip version ipv4 or ipv6: ` +
				`the field "provider_ip" is deprecated and no longer used`},
		},
		"invalid_domain": {
			entry:       `{"provider":"cloudflare","domain":"not a domain"}`,
			errContains: "extracting owners from domains",
		},
		"empty_domain": {
			entry:       `{"provider":"cloudflare","domain":""}`,
			errContains: "extracting owners from domains",
		},
		"unknown_provider": {
			entry:      `{"provider":"unknownprovider","domain":"example.com"}`,
			errWrapped: provider.ErrProviderUnknown,
		},
		"google_not_supported": {
			entry:      `{"provider":"google","domain":"example.com"}`,
			errWrapped: ErrProviderNoLongerSupported,
		},
		"non_object_array": {
			entry:      `[]`,
			errWrapped: ErrEntryNotObject,
		},
		"non_object_string": {
			entry:      `"hello"`,
			errWrapped: ErrEntryNotObject,
		},
		"non_object_number": {
			entry:      `42`,
			errWrapped: ErrEntryNotObject,
		},
		"non_object_null": {
			entry:      `null`,
			errWrapped: ErrEntryNotObject,
		},
		"invalid_json": {
			entry:      `{"provider":`,
			errWrapped: ErrEntryInvalidJSON,
		},
		"empty": {
			entry:      ``,
			errWrapped: ErrEntryInvalidJSON,
		},
		"missing_provider": {
			entry:      `{"domain":"example.com"}`,
			errWrapped: provider.ErrProviderUnknown,
		},
		"empty_object": {
			entry:       `{}`,
			errContains: "cannot derive eTLD+1",
		},
		"missing_domain": {
			entry:       `{"provider":"cloudflare"}`,
			errContains: "cannot derive eTLD+1",
		},
		"missing_provider_specific_settings": {
			entry:       `{"provider":"cloudflare","domain":"example.com"}`,
			errContains: "zone identifier",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			warnings, err := ValidateEntry(json.RawMessage(testCase.entry))

			switch {
			case testCase.errWrapped != nil:
				assert.ErrorIs(t, err, testCase.errWrapped)
			case testCase.errContains != "":
				require.Error(t, err)
				assert.ErrorContains(t, err, testCase.errContains)
			default:
				require.NoError(t, err)
			}
			assert.Equal(t, testCase.warnings, warnings)
		})
	}
}

func Test_Store_ValidateAll(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	t.Run("all_valid", func(t *testing.T) {
		t.Parallel()

		warnings, err := store.ValidateAll(toRawMessages(t, entryCloudflare, entryDuckDNS))

		require.NoError(t, err)
		assert.Empty(t, warnings)
	})

	t.Run("no_entries", func(t *testing.T) {
		t.Parallel()

		warnings, err := store.ValidateAll(nil)

		require.NoError(t, err)
		assert.Empty(t, warnings)
	})

	t.Run("all_failing_indexes_are_reported", func(t *testing.T) {
		t.Parallel()

		entries := toRawMessages(t, entryCloudflare, `[]`, `{}`, `{"provider":"nope",`+
			`"domain":"example.com"}`)

		warnings, err := store.ValidateAll(entries)

		require.Error(t, err)
		assert.Empty(t, warnings)
		assert.ErrorContains(t, err, "setting 1: setting entry is not a JSON object")
		assert.ErrorContains(t, err, "setting 2: ")
		assert.ErrorContains(t, err, `setting 3: unknown provider`)
		assert.NotContains(t, err.Error(), "setting 0:")
		assert.ErrorIs(t, err, ErrEntryNotObject)
		assert.ErrorIs(t, err, provider.ErrProviderUnknown)
	})
}
