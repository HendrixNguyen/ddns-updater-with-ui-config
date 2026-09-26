package manager

import (
	"errors"

	"github.com/qdm12/ddns-updater/internal/params"
)

// ErrEntryNotFound is returned when an entry index does not exist. It wraps
// params.ErrEntryNotFound so callers can match on either.
var ErrEntryNotFound = params.ErrEntryNotFound

var (
	errNoDatabase      = errors.New("no database attached to the settings manager")
	errReadSettings    = errors.New("cannot read the settings entries")
	errSaveSettings    = errors.New("cannot save the settings entries")
	errSettingsChanged = errors.New("settings change rolled back: reloading the new settings failed")
)

// ValidationError reports user-supplied settings that could not be applied.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }

func (e *ValidationError) Unwrap() error { return e.Err }

// IsValidationError reports whether err was caused by invalid user-supplied settings.
func IsValidationError(err error) bool {
	var validationError *ValidationError
	return errors.As(err, &validationError)
}
