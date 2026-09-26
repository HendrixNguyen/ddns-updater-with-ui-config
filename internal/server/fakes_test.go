package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	appconstants "github.com/qdm12/ddns-updater/internal/constants"
	"github.com/qdm12/ddns-updater/internal/manager"
	"github.com/qdm12/ddns-updater/internal/provider"
	providerconstants "github.com/qdm12/ddns-updater/internal/provider/constants"
	"github.com/qdm12/ddns-updater/internal/records"
	"github.com/qdm12/ddns-updater/pkg/publicip/ipversion"
	"github.com/stretchr/testify/require"
)

// fakeDB is a Database returning a fixed set of records.
type fakeDB struct {
	records []records.Record
}

func (f *fakeDB) SelectAll() []records.Record { return f.records }

func (f *fakeDB) Count() int { return len(f.records) }

// fakeRunner is an UpdateForcer returning a fixed set of errors.
type fakeRunner struct {
	mutex  sync.Mutex
	calls  int
	errors []error
}

func (f *fakeRunner) ForceUpdate(_ context.Context) []error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.calls++
	return f.errors
}

func (f *fakeRunner) Calls() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.calls
}

// fakeSettings is a SettingsManager recording every call it receives and
// returning canned results.
type fakeSettings struct {
	mutex sync.Mutex

	filepath string

	entries        []json.RawMessage
	entriesWarning []string
	entriesErr     error

	validateWarnings []string
	validateErr      error

	result manager.Result

	addErr        error
	addEntry      json.RawMessage
	addCalls      int
	replaceErr    error
	replaceIndex  int
	replaceEntry  json.RawMessage
	replaceCalls  int
	deleteErr     error
	deleteIndex   int
	deleteCalls   int
	replaceAllErr error
	replaceAll    []json.RawMessage
	replaceAllOps int

	reloadWarnings []string
	reloadErr      error
	reloadCalls    int
}

func newFakeSettings(t *testing.T) *fakeSettings {
	t.Helper()

	return &fakeSettings{filepath: filepath.Join(t.TempDir(), "config.json")}
}

func (f *fakeSettings) Filepath() string { return f.filepath }

func (f *fakeSettings) Entries() ([]json.RawMessage, []string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.entries, f.entriesWarning, f.entriesErr
}

func (f *fakeSettings) Validate(entries []json.RawMessage) ([]string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.replaceAll = entries
	return f.validateWarnings, f.validateErr
}

func (f *fakeSettings) AddEntry(entry json.RawMessage) (manager.Result, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.addCalls++
	f.addEntry = entry
	return f.result, f.addErr
}

func (f *fakeSettings) ReplaceEntry(index int, entry json.RawMessage) (manager.Result, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.replaceCalls++
	f.replaceIndex = index
	f.replaceEntry = entry
	return f.result, f.replaceErr
}

func (f *fakeSettings) DeleteEntry(index int) (manager.Result, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.deleteCalls++
	f.deleteIndex = index
	return f.result, f.deleteErr
}

func (f *fakeSettings) ReplaceAllEntries(entries []json.RawMessage) (manager.Result, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.replaceAllOps++
	f.replaceAll = entries
	return f.result, f.replaceAllErr
}

func (f *fakeSettings) Reload() ([]string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.reloadCalls++
	return f.reloadWarnings, f.reloadErr
}

func (f *fakeSettings) AddCalls() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.addCalls
}

func (f *fakeSettings) AddEntryReceived() json.RawMessage {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.addEntry
}

// newTestHandler builds the full HTTP handler the server mounts, with the root
// URL trimmed to the empty string so that routes are mounted at "/".
func newTestHandler(t *testing.T, db Database, settings SettingsManager,
	runner UpdateForcer,
) http.Handler {
	t.Helper()

	if db == nil {
		db = &fakeDB{}
	}
	if settings == nil {
		settings = newFakeSettings(t)
	}
	if runner == nil {
		runner = &fakeRunner{}
	}

	return newHandler(context.Background(), "/", db, settings, runner)
}

// doTestRequest sends the request given to the handler and returns the
// recorder holding the response. An empty body sends no body at all and no
// content type, which lets a test exercise the content type check.
func doTestRequest(t *testing.T, handler http.Handler,
	method, path, body string,
) *httptest.ResponseRecorder {
	t.Helper()

	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// decodeJSONResponse decodes the JSON body of the response into a generic
// map, and fails the test if the body is not valid JSON.
func decodeJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var data map[string]any
	err := json.Unmarshal(recorder.Body.Bytes(), &data)
	require.NoError(t, err)

	return data
}

// makeRecords builds count records backed by a real Cloudflare provider, so
// that the record mapping is exercised against a production provider.
func makeRecords(count int) []records.Record {
	data := json.RawMessage(`{"token":"token","zone_identifier":"zone","ttl":600}`)

	all := make([]records.Record, count)
	for i := range all {
		newProvider, err := provider.New(providerconstants.Cloudflare, data,
			"example.com", fmt.Sprintf("sub%d", i), ipversion.IP4, netip.Prefix{})
		if err != nil {
			panic(err)
		}
		all[i] = records.New(newProvider, nil)
		all[i].Status = appconstants.SUCCESS
		all[i].Message = "changed to 1.2.3.4"
		all[i].Time = time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
	}

	return all
}
