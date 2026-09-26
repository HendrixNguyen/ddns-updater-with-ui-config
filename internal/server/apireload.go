package server

import (
	"net/http"
)

// reloadResponse is the body of a reload response.
type reloadResponse struct {
	Reloaded bool     `json:"reloaded"`
	Records  int      `json:"records"`
	Warnings []string `json:"warnings"`
}

// apiReload re-reads the settings configuration and applies it to the running
// updater without saving any change.
func (h *handlers) apiReload(w http.ResponseWriter, _ *http.Request) {
	warnings, err := h.settings.Reload()
	if err != nil {
		h.respondManagerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, reloadResponse{
		Reloaded: true,
		Records:  h.db.Count(),
		Warnings: orEmptyStrings(warnings),
	})
}
