package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/qdm12/ddns-updater/internal/manager"
	"github.com/qdm12/ddns-updater/internal/params"
)

// Source values reported by the settings API.
const (
	sourceFile  = "file"
	sourceEnv   = "env"
	sourceEmpty = "empty"
)

// Environment variable state values reported by the settings API.
const (
	envSet   = "set"
	envUnset = "unset"
)

// settingsResponse is the body of every settings API response. Settings and
// Warnings are never nil, so that clients always get an array.
type settingsResponse struct {
	Settings []json.RawMessage `json:"settings"`
	Warnings []string          `json:"warnings"`
	Source   string            `json:"source"`
	Env      string            `json:"env"`
	FilePath string            `json:"filePath"`
	Records  int               `json:"records"`
}

// replaceAllRequest is the body of a replace-all settings request.
type replaceAllRequest struct {
	Settings []json.RawMessage `json:"settings"`
}

// validateRequest is the optional wrapper accepted by the validate endpoint.
type validateRequest struct {
	Settings []json.RawMessage `json:"settings"`
}

// validateResponse is the body of a validate response. It is always sent with
// a 200 status code so the raw editor can show inline feedback.
type validateResponse struct {
	Valid       bool     `json:"valid"`
	Warnings    []string `json:"warnings"`
	Errors      []string `json:"errors"`
	RecordCount int      `json:"recordCount"`
}

// apiGetSettings returns the current settings entries together with the
// metadata the web UI needs to display them.
func (h *handlers) apiGetSettings(w http.ResponseWriter, _ *http.Request) {
	entries, warnings, err := h.settings.Entries()
	if err != nil {
		h.respondManagerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, h.newSettingsResponse(entries, warnings))
}

// apiAddSetting appends the single settings entry found in the request body.
func (h *handlers) apiAddSetting(w http.ResponseWriter, r *http.Request) {
	entry, ok := decodeSettingsEntry(w, r)
	if !ok {
		return
	}

	result, err := h.settings.AddEntry(entry)
	if err != nil {
		h.respondManagerError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, settingsResponseFromResult(result))
}

// apiReplaceSetting overwrites the settings entry at the {index} URL
// parameter with the single settings entry found in the request body.
func (h *handlers) apiReplaceSetting(w http.ResponseWriter, r *http.Request) {
	index, ok := settingIndexFromPath(w, r)
	if !ok {
		return
	}

	entry, ok := decodeSettingsEntry(w, r)
	if !ok {
		return
	}

	result, err := h.settings.ReplaceEntry(index, entry)
	if err != nil {
		h.respondManagerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, settingsResponseFromResult(result))
}

// apiDeleteSetting removes the settings entry at the {index} URL parameter.
func (h *handlers) apiDeleteSetting(w http.ResponseWriter, r *http.Request) {
	index, ok := settingIndexFromPath(w, r)
	if !ok {
		return
	}

	result, err := h.settings.DeleteEntry(index)
	if err != nil {
		h.respondManagerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, settingsResponseFromResult(result))
}

// apiReplaceAllSettings overwrites every settings entry with the ones found
// in the `{"settings":[...]}` request body.
func (h *handlers) apiReplaceAllSettings(w http.ResponseWriter, r *http.Request) {
	request := replaceAllRequest{}
	ok := decodeJSONBody(w, r, &request)
	if !ok {
		return
	}

	entries := request.Settings
	if entries == nil {
		entries = []json.RawMessage{}
	}

	result, err := h.settings.ReplaceAllEntries(entries)
	if err != nil {
		h.respondManagerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, settingsResponseFromResult(result))
}

// apiValidateSettings checks settings entries without saving them. It always
// answers with a 200 status code, carrying the validity in the body.
func (h *handlers) apiValidateSettings(w http.ResponseWriter, r *http.Request) {
	raw := json.RawMessage{}
	ok := decodeJSONBody(w, r, &raw)
	if !ok {
		return
	}

	entries, err := settingsEntriesOf(raw)
	if err != nil {
		writeJSON(w, http.StatusOK, validateResponse{
			Valid:       false,
			Warnings:    []string{},
			Errors:      errorLines(err),
			RecordCount: h.db.Count(),
		})
		return
	}

	warnings, err := h.settings.Validate(entries)
	response := validateResponse{
		Valid:       err == nil,
		Warnings:    orEmptyStrings(warnings),
		Errors:      []string{},
		RecordCount: h.db.Count(),
	}
	if err != nil {
		response.Errors = errorLines(err)
	}

	writeJSON(w, http.StatusOK, response)
}

// settingsResponseFromResult builds the settings API response body from a
// result returned by the settings manager. The settings and warnings are
// never nil, so that they always marshal as JSON arrays.
func settingsResponseFromResult(result manager.Result) settingsResponse {
	return settingsResponse{
		Settings: orEmptyRawMessages(result.Settings),
		Warnings: orEmptyStrings(result.Warnings),
		Source:   result.Source,
		Env:      result.Env,
		FilePath: result.FilePath,
		Records:  result.Records,
	}
}

// newSettingsResponse builds the settings API response body from the entries
// and warnings read from the settings manager, deriving the metadata from the
// settings file and the record database. The settings and warnings are never
// nil, so that they always marshal as JSON arrays.
func (h *handlers) newSettingsResponse(entries []json.RawMessage,
	warnings []string,
) settingsResponse {
	return settingsResponse{
		Settings: orEmptyRawMessages(entries),
		Warnings: orEmptyStrings(warnings),
		Source:   h.settingsSource(len(entries)),
		Env:      settingsEnvState(),
		FilePath: h.settings.Filepath(),
		Records:  h.db.Count(),
	}
}

// settingsSource returns whether the settings currently in use come from the
// settings file, from the CONFIG environment variable, or from an empty
// configuration, for a configuration holding the number of entries given. It
// mirrors the resolution done by the settings package, so that a plain read
// reports the same source as a mutation would.
func (h *handlers) settingsSource(entriesCount int) string {
	return settingsSourceFor(h.settings.Filepath(), os.Getenv("CONFIG"), entriesCount)
}

// settingsSourceFor returns the configuration source for the settings file at
// the path given, given the CONFIG environment variable value and the number
// of settings entries the configuration holds.
func settingsSourceFor(filepath, envValue string, entriesCount int) string {
	if envValue != "" && settingsFileEnvSeed(filepath) != params.EnvSeedOf(envValue) {
		return sourceEnv
	}

	if entriesCount == 0 {
		return sourceEmpty
	}

	return sourceFile
}

// settingsFileEnvSeed returns the environment variable seed stamped in the
// settings file, or an empty string if the file cannot be read or holds none.
func settingsFileEnvSeed(filepath string) string {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return ""
	}

	document := struct {
		EnvSeed string `json:"_env_seed"`
	}{}
	err = json.Unmarshal(data, &document)
	if err != nil {
		return ""
	}

	return document.EnvSeed
}

// settingsEnvState returns whether the CONFIG environment variable holds a
// settings document.
func settingsEnvState() string {
	if params.EnvSeedOf(os.Getenv("CONFIG")) != "" {
		return envSet
	}
	return envUnset
}

// decodeSettingsEntry decodes the request body as a single raw settings
// entry, which must be a JSON object.
func decodeSettingsEntry(w http.ResponseWriter, r *http.Request) (
	entry json.RawMessage, ok bool,
) {
	raw := json.RawMessage{}
	ok = decodeJSONBody(w, r, &raw)
	if !ok {
		return nil, false
	}

	return json.RawMessage(bytes.TrimSpace(raw)), true
}

// settingsEntriesOf interprets a validate request body, which is either a
// `{"settings":[...]}` wrapper or a single settings entry object.
func settingsEntriesOf(raw json.RawMessage) (entries []json.RawMessage, err error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, params.ErrEntryInvalidJSON
	}

	if trimmed[0] == '{' {
		request := validateRequest{}
		err = json.Unmarshal(trimmed, &request)
		if err == nil && request.Settings != nil {
			return request.Settings, nil
		}
	}

	return []json.RawMessage{trimmed}, nil
}

// errorLines splits an error into its non-empty lines, so that an error
// joining several validation errors yields one array entry per error.
func errorLines(err error) []string {
	if err == nil {
		return []string{}
	}

	lines := strings.Split(err.Error(), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return cleaned
}

// orEmptyStrings returns an empty slice instead of nil for an empty input, so
// that the slice marshals as `[]` instead of `null`.
func orEmptyStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// orEmptyRawMessages returns an empty slice instead of nil for an empty input,
// so that the slice marshals as `[]` instead of `null`.
func orEmptyRawMessages(m []json.RawMessage) []json.RawMessage {
	if m == nil {
		return []json.RawMessage{}
	}
	return m
}
