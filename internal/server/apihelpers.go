package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/qdm12/ddns-updater/internal/manager"
	"github.com/qdm12/ddns-updater/internal/params"
)

// maxRequestBodyBytes is the maximum size in bytes of a JSON request body
// accepted by the API. It is 1 MiB, which is orders of magnitude larger than
// any settings file the updater can produce.
const maxRequestBodyBytes = 1 << 20

// contentTypeJSON is the content type set on every JSON response.
const contentTypeJSON = "application/json; charset=utf-8"

// mediaTypeJSON is the only media type accepted in a JSON request.
const mediaTypeJSON = "application/json"

var (
	errUnsupportedMediaType = errors.New("unsupported content type, expected application/json")
	errBodyTooLarge         = errors.New("request body too large")
	errMalformedJSON        = errors.New("malformed JSON request body")
	errEmptyJSONBody        = errors.New("empty JSON request body")
	errInvalidIndex         = errors.New("invalid settings index, expected a non-negative integer")
)

// decodeJSONBody decodes the JSON request body of r into target. It writes an
// error JSON response and returns false if the request is not acceptable, in
// which case the caller must stop. The body is always drained, up to
// maxRequestBodyBytes, and closed before returning.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any) (ok bool) {
	defer drainAndClose(r)

	if !checkJSONContentType(w, r) {
		return false
	}

	// The body is bounded so that a malicious client cannot make the server
	// allocate an unbounded amount of memory.
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(target)
	if err == nil {
		return true
	}

	var maxBytesError *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesError):
		httpErrors(w, http.StatusRequestEntityTooLarge,
			[]error{fmt.Errorf("%w: limit is %d bytes", errBodyTooLarge, maxRequestBodyBytes)})
	case errors.Is(err, io.EOF):
		httpErrors(w, http.StatusBadRequest, []error{errEmptyJSONBody})
	default:
		// io.ErrUnexpectedEOF, *json.SyntaxError, *json.UnmarshalTypeError and
		// any other decoding failure are all client side problems.
		httpErrors(w, http.StatusBadRequest, []error{fmt.Errorf("%w: %w", errMalformedJSON, err)})
	}

	return false
}

// drainAndClose drains up to maxRequestBodyBytes of the remaining request
// body and closes it, so that the underlying connection can be reused.
func drainAndClose(r *http.Request) {
	if r.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, maxRequestBodyBytes))
	_ = r.Body.Close()
}

// checkJSONContentType writes a 415 error response and returns false if the
// request does not declare an application/json content type.
func checkJSONContentType(w http.ResponseWriter, r *http.Request) (ok bool) {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType == mediaTypeJSON {
		return true
	}

	if contentType == "" {
		contentType = "<missing>"
	}
	httpErrors(w, http.StatusUnsupportedMediaType,
		[]error{fmt.Errorf("%w: got %q", errUnsupportedMediaType, contentType)})

	return false
}

// writeJSON writes the data given as a JSON response with the status code
// given. The document is encoded in a buffer first, so that an encoding
// failure produces a clean error response instead of a truncated body.
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	buffer := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(data)
	if err != nil {
		httpErrors(w, http.StatusInternalServerError, []error{err})
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(statusCode)
	_, _ = w.Write(buffer.Bytes())
}

// respondManagerError maps a settings manager error to an HTTP status code
// and writes the error as a `{"errors":[...]}` JSON response. The error is
// logged but never a stack trace.
func (h *handlers) respondManagerError(w http.ResponseWriter, err error) {
	status := managerErrorStatus(err)
	h.logger.Error("settings API request failed: " + err.Error())
	httpErrors(w, status, []error{err})
}

// managerErrorStatus maps a settings manager error to an HTTP status code.
func managerErrorStatus(err error) (status int) {
	switch {
	case errors.Is(err, params.ErrEntryNotFound):
		return http.StatusNotFound
	case manager.IsValidationError(err),
		errors.Is(err, params.ErrEntryInvalidJSON),
		errors.Is(err, params.ErrEntryNotObject),
		errors.Is(err, params.ErrEntryNoProvider):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// settingIndexFromPath parses the {index} URL parameter as a non-negative
// integer. It writes a 400 error response and returns false if the parameter
// is missing, not a number, out of range or negative.
func settingIndexFromPath(w http.ResponseWriter, r *http.Request) (index int, ok bool) {
	raw := chi.URLParam(r, "index")
	parsed, err := strconv.Atoi(raw)
	if err == nil && parsed >= 0 {
		return parsed, true
	}

	reason := "not a number"
	switch {
	case err == nil:
		reason = "must not be negative"
	case errors.Is(err, strconv.ErrRange):
		reason = "out of range"
	case raw == "":
		reason = "missing"
	}
	httpErrors(w, http.StatusBadRequest,
		[]error{fmt.Errorf("%w: %q %s", errInvalidIndex, raw, reason)})

	return 0, false
}
