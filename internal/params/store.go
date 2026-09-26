package params

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
)

const (
	// settingsKey is the JSON object key holding the array of settings entries.
	settingsKey = "settings"
	// envSeedKey is the JSON object key holding the CONFIG environment variable seed.
	envSeedKey = "_env_seed"
	// tempFileSuffix is appended to the settings file path to build the name of
	// the temporary file used for atomic writes.
	tempFileSuffix = ".tmp"
	// indent is the string used to indent the JSON document written to file.
	indent = "  "
)

// document is the JSON document persisted in the DDNS settings file.
// Unknown top level fields are tolerated when reading the document, so that
// files written by other versions of the program keep working.
type document struct {
	Settings []json.RawMessage `json:"settings"`
	// EnvSeed is the hex SHA-256 of the CONFIG environment variable value at the
	// time the file was last synchronized from it. It lets us detect whether the
	// environment variable changed between runs, so that edits made through the
	// web UI are not overwritten on restart.
	EnvSeed string `json:"_env_seed,omitempty"`
}

// noopLogger is used when a nil logger is given to NewStore or NewReader,
// so that logging never panics.
type noopLogger struct{}

func (noopLogger) Info(string)  {}
func (noopLogger) Debug(string) {}

// Store reads and writes the DDNS settings JSON configuration file.
// All its methods are safe for concurrent use.
type Store struct {
	mutex     sync.RWMutex
	logger    Logger
	filepath  string
	readFile  func(filename string) ([]byte, error)
	writeFile func(filename string, data []byte, perm fs.FileMode) error
	rename    func(oldpath, newpath string) error
	remove    func(name string) error
}

// NewStore creates a new settings store operating on the file at the given path.
func NewStore(logger Logger, filepath string) *Store {
	if logger == nil {
		logger = noopLogger{}
	}

	return &Store{
		logger:    logger,
		filepath:  filepath,
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		rename:    os.Rename,
		remove:    os.Remove,
	}
}

// Filepath returns the path of the settings file the store operates on.
func (s *Store) Filepath() string {
	return s.filepath
}

// Entries returns the settings entries found in the settings file.
// A missing file, an empty file, or a file without any settings entry yields
// a nil entries slice and no error. The warnings returned are currently always
// nil and are reserved for future use.
func (s *Store) Entries() (entries []json.RawMessage, warnings []string, err error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	doc, _, err := s.read()
	if err != nil {
		return nil, nil, err
	}

	if len(doc.Settings) == 0 {
		return nil, nil, nil
	}

	return doc.Settings, nil, nil
}

// Save writes the given settings entries to the settings file, preserving the
// environment variable seed currently stored in the file, if any.
func (s *Store) Save(entries []json.RawMessage) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	doc, extra, err := s.read()
	if err != nil {
		return err
	}

	return s.write(extra, entries, doc.EnvSeed)
}

// SaveWithSeed writes the given settings entries and environment variable seed
// to the settings file, replacing any environment variable seed already
// stored in the file.
func (s *Store) SaveWithSeed(entries []json.RawMessage, envSeed string) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, extra, err := s.read()
	if err != nil {
		return err
	}

	return s.write(extra, entries, envSeed)
}

// Add appends the given settings entry to the settings file and returns the
// new complete list of settings entries.
func (s *Store) Add(entry json.RawMessage) (entries []json.RawMessage, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	doc, extra, err := s.read()
	if err != nil {
		return nil, err
	}

	doc.Settings = append(doc.Settings, entry)
	err = s.write(extra, doc.Settings, doc.EnvSeed)
	if err != nil {
		return nil, err
	}

	return doc.Settings, nil
}

// Replace overwrites the settings entry at the given index and returns the
// new complete list of settings entries. If the index is out of range,
// ErrEntryNotFound is returned and the file is left untouched.
func (s *Store) Replace(index int, entry json.RawMessage) (
	entries []json.RawMessage, err error,
) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	doc, extra, err := s.read()
	if err != nil {
		return nil, err
	}

	if index < 0 || index >= len(doc.Settings) {
		return nil, fmt.Errorf("%w: index %d for %d settings",
			ErrEntryNotFound, index, len(doc.Settings))
	}

	entries = make([]json.RawMessage, len(doc.Settings))
	copy(entries, doc.Settings)
	entries[index] = entry

	err = s.write(extra, entries, doc.EnvSeed)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// Delete removes the settings entry at the given index and returns the new
// complete list of settings entries. If the index is out of range,
// ErrEntryNotFound is returned and the file is left untouched.
func (s *Store) Delete(index int) (entries []json.RawMessage, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	doc, extra, err := s.read()
	if err != nil {
		return nil, err
	}

	if index < 0 || index >= len(doc.Settings) {
		return nil, fmt.Errorf("%w: index %d for %d settings",
			ErrEntryNotFound, index, len(doc.Settings))
	}

	entries = make([]json.RawMessage, 0, len(doc.Settings)-1)
	entries = append(entries, doc.Settings[:index]...)
	entries = append(entries, doc.Settings[index+1:]...)

	err = s.write(extra, entries, doc.EnvSeed)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ReplaceAll overwrites all the settings entries and returns the saved list
// of settings entries.
func (s *Store) ReplaceAll(entries []json.RawMessage) (saved []json.RawMessage, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	doc, extra, err := s.read()
	if err != nil {
		return nil, err
	}

	err = s.write(extra, entries, doc.EnvSeed)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ValidateAll validates each of the given settings entries and reports every
// entry which failed validation in the returned error, so that the caller can
// show all the problems at once.
func (s *Store) ValidateAll(entries []json.RawMessage) (warnings []string, err error) {
	errs := make([]error, 0, len(entries))
	for i, entry := range entries {
		newWarnings, err := ValidateEntry(entry)
		warnings = append(warnings, newWarnings...)
		if err != nil {
			errs = append(errs, fmt.Errorf("setting %d: %w", i, err))
		}
	}

	if len(errs) > 0 {
		return warnings, errors.Join(errs...)
	}

	return warnings, nil
}

// read returns the document found in the settings file together with the
// unknown top level fields it contains. A missing or empty file yields a zero
// value document and no error.
func (s *Store) read() (doc document, extra map[string]json.RawMessage, err error) {
	data, err := s.readFile(s.filepath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return document{}, nil, nil
		}

		return document{}, nil, fmt.Errorf("%w: %w", errReadConfigFile, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return document{}, nil, nil
	}

	return parseDocument(data)
}

// write marshals the given document and saves it atomically to the settings
// file, keeping the given unknown top level fields.
func (s *Store) write(extra map[string]json.RawMessage, entries []json.RawMessage,
	envSeed string) (err error,
) {
	doc := document{Settings: entries, EnvSeed: envSeed}
	if doc.Settings == nil {
		doc.Settings = []json.RawMessage{}
	}

	data, err := marshalDocument(extra, doc)
	if err != nil {
		return err
	}

	err = s.writeAtomic(data)
	if err != nil {
		return err
	}

	s.logger.Info("settings file written to " + s.filepath)

	return nil
}

// writeAtomic writes the data to a temporary file and renames it over the
// settings file, which is atomic on POSIX systems. The temporary file is
// created with the permissions of the settings file it replaces, so that
// saving through the web UI never widens the access to the credentials it
// holds. A settings file which does not exist yet gets the default
// permissions. If anything fails, the temporary file is removed on a best
// effort basis.
func (s *Store) writeAtomic(data []byte) (err error) {
	const defaultFilePerm = fs.FileMode(0o666)
	filePerm := defaultFilePerm
	info, statErr := os.Stat(s.filepath)
	switch {
	case statErr == nil:
		filePerm = info.Mode().Perm()
	case errors.Is(statErr, os.ErrNotExist):
	default:
		s.logger.Info("cannot read the permissions of " + s.filepath +
			", using the default ones: " + statErr.Error())
	}
	tempPath := s.filepath + tempFileSuffix
	s.logger.Debug("writing settings to temporary file " + tempPath)
	err = s.writeFile(tempPath, data, filePerm)
	if err != nil {
		return fmt.Errorf("%w: %w", errWriteTempFile, err)
	}

	err = s.rename(tempPath, s.filepath)
	if err != nil {
		s.removeTempFile(tempPath)
		return fmt.Errorf("%w: %w", errRenameTempFile, err)
	}

	s.logger.Debug("temporary settings file renamed to " + s.filepath)

	return nil
}

func (s *Store) removeTempFile(tempPath string) {
	err := s.remove(tempPath)
	if err != nil {
		s.logger.Info("could not remove temporary file " + tempPath + ": " + err.Error())
		return
	}

	s.logger.Debug("removed temporary file " + tempPath)
}

var (
	// ErrEntryNotFound is returned when a settings entry index is out of range.
	ErrEntryNotFound = errors.New("setting entry not found")
	// ErrEntryNotObject is returned when a settings entry is not a JSON object.
	ErrEntryNotObject = errors.New("setting entry is not a JSON object")
	// ErrEntryInvalidJSON is returned when a settings entry is not valid JSON.
	ErrEntryInvalidJSON = errors.New("setting entry is not valid JSON")
	// ErrEntryNoProvider is returned when a settings entry produces no provider.
	ErrEntryNoProvider = errors.New("setting entry produces no provider")
)

var (
	errReadConfigFile  = errors.New("cannot read configuration file")
	errMarshalDocument = errors.New("cannot marshal settings document")
	errWriteTempFile   = errors.New("cannot write temporary settings file")
	errRenameTempFile  = errors.New("cannot rename temporary settings file")
)

// ValidateEntry checks a single settings entry by parsing it with the same code
// path used at startup. Warnings are returned even on success.
func ValidateEntry(entry json.RawMessage) (warnings []string, err error) {
	entry = json.RawMessage(bytes.TrimSpace(entry))
	if len(entry) == 0 || !json.Valid(entry) {
		return nil, fmt.Errorf("%w: %s", ErrEntryInvalidJSON, string(entry))
	}

	if entry[0] != '{' {
		return nil, fmt.Errorf("%w: %s", ErrEntryNotObject, string(entry))
	}

	wrapper := struct {
		Settings []json.RawMessage `json:"settings"`
	}{Settings: []json.RawMessage{entry}}

	data, err := json.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errMarshalDocument, err)
	}

	providers, warnings, err := extractAllSettings(data)
	if err != nil {
		return warnings, err
	}

	if len(providers) == 0 {
		return warnings, fmt.Errorf("%w: %s", ErrEntryNoProvider, string(entry))
	}

	return warnings, nil
}

// parseDocument parses the given JSON document, and also returns its unknown
// top level fields so that they can be preserved when writing it back.
func parseDocument(data []byte) (doc document, extra map[string]json.RawMessage, err error) {
	err = json.Unmarshal(data, &doc)
	if err != nil {
		return document{}, nil, fmt.Errorf("%w: %w", errUnmarshalRaw, err)
	}

	extra = make(map[string]json.RawMessage)
	err = json.Unmarshal(data, &extra)
	if err != nil {
		return document{}, nil, fmt.Errorf("%w: %w", errUnmarshalRaw, err)
	}

	delete(extra, settingsKey)
	delete(extra, envSeedKey)

	return doc, extra, nil
}

// marshalDocument marshals the given document with an indentation, keeping the
// given unknown top level fields, and appends a trailing newline to the result.
func marshalDocument(extra map[string]json.RawMessage, doc document) (data []byte, err error) {
	all := make(map[string]json.RawMessage, len(extra)+2)
	for key, value := range extra {
		all[key] = value
	}

	settings, err := json.Marshal(doc.Settings)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errMarshalDocument, err)
	}
	all[settingsKey] = settings

	if doc.EnvSeed != "" {
		seed, err := json.Marshal(doc.EnvSeed)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errMarshalDocument, err)
		}
		all[envSeedKey] = seed
	}

	data, err = json.MarshalIndent(all, "", indent)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errMarshalDocument, err)
	}

	return append(data, '\n'), nil
}
