package server

import (
	"bytes"
	"net/http"
)

// settingsPageData is the view model of the settings page. It is exported
// field by field so that the HTML template engine can render it.
type settingsPageData struct {
	// BasePath is the root URL the server is mounted under, without a
	// trailing slash, as a JSON string literal escaped so that it is safe to
	// embed in the text content of a script element. It is `""` (the two quote
	// characters) when the server is mounted at the root.
	BasePath string
	// RecordCount is the number of records currently updated by the updater.
	RecordCount int
	// APIPath is the path of the settings API, relative to the root URL.
	APIPath string
}

// settingsPage renders the shell of the redesigned settings page. The page
// itself loads all of its data from the JSON API, so the template only gets
// the base path and the record count. The template is executed in a buffer
// first, so that a template failure produces a clean error response instead
// of a partially written 200 response.
func (h *handlers) settingsPage(w http.ResponseWriter, _ *http.Request) {
	data := settingsPageData{
		BasePath:    scriptSafeJSON(h.basePath),
		RecordCount: h.db.Count(),
		APIPath:     h.basePath + "/api",
	}

	buffer := bytes.NewBuffer(nil)
	err := h.settingsTemplate.ExecuteTemplate(buffer, "settings.html", data)
	if err != nil {
		h.logger.Error("failed generating settings webpage: " + err.Error())
		httpError(w, http.StatusInternalServerError, "failed generating webpage: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buffer.Bytes())
}
