package server

import (
	"bytes"
	"net/http"

	"github.com/qdm12/ddns-updater/internal/models"
)

func (h *handlers) index(w http.ResponseWriter, _ *http.Request) {
	var htmlData models.HTMLData
	htmlData.BasePath = scriptSafeJSON(h.basePath)
	all := h.db.SelectAll()
	htmlData.Rows = make([]models.HTMLRow, len(all))
	for i := range all {
		htmlData.Rows[i] = all[i].HTML(h.timeNow())
	}

	// The template is executed in a buffer first, so that a template failure
	// produces a clean error response instead of a partially written 200
	// response.
	buffer := bytes.NewBuffer(nil)
	err := h.indexTemplate.ExecuteTemplate(buffer, "index.html", htmlData)
	if err != nil {
		h.logger.Error("failed generating webpage: " + err.Error())
		httpError(w, http.StatusInternalServerError, "failed generating webpage: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buffer.Bytes())
}
