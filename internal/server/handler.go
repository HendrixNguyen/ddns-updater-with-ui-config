package server

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type handlers struct {
	ctx context.Context //nolint:containedctx
	// Objects
	db               Database
	settings         SettingsManager
	logger           Logger
	runner           UpdateForcer
	indexTemplate    *template.Template
	settingsTemplate *template.Template
	basePath         string
	// Mockable functions
	timeNow func() time.Time
}

//go:embed ui/*
var uiFS embed.FS //nolint:gochecknoglobals

// discardLogger is used when no logger is given to newHandler, so that
// logging from a handler never panics on a nil interface value.
type discardLogger struct{}

func (discardLogger) Info(string)  {}
func (discardLogger) Warn(string)  {}
func (discardLogger) Error(string) {}

// newHandler builds the HTTP handler serving the index page, the settings
// page, the JSON API and the static files, all mounted under rootURL. The
// logger is optional and defaults to a no-op logger.
func newHandler(ctx context.Context, rootURL string,
	db Database, settings SettingsManager, runner UpdateForcer,
	logger ...Logger,
) http.Handler {
	indexTemplate := template.Must(template.ParseFS(uiFS, "ui/index.html"))
	settingsTemplate := template.Must(template.ParseFS(uiFS, "ui/settings.html"))

	staticFolder, err := fs.Sub(uiFS, "ui/static")
	if err != nil {
		panic(err)
	}

	log := Logger(discardLogger{})
	if len(logger) > 0 && logger[0] != nil {
		log = logger[0]
	}

	handlers := &handlers{
		ctx:              ctx,
		db:               db,
		settings:         settings,
		logger:           log,
		indexTemplate:    indexTemplate,
		settingsTemplate: settingsTemplate,
		// TODO build information
		timeNow: time.Now,
		runner:  runner,
	}

	router := chi.NewRouter()

	router.Use(middleware.ClientIPFromRemoteAddr)
	router.Use(middleware.Logger)
	rootURL = strings.TrimSuffix(rootURL, "/")
	handlers.basePath = rootURL

	if rootURL != "" {
		router.Handle(rootURL, http.RedirectHandler(rootURL+"/", http.StatusPermanentRedirect))
	}
	router.Get(rootURL+"/", handlers.index)

	router.Get(rootURL+"/update", handlers.update)

	router.Get(rootURL+"/settings", handlers.settingsPage)

	router.Route(rootURL+"/api", func(r chi.Router) {
		r.Get("/providers", handlers.apiProviders)
		r.Get("/settings", handlers.apiGetSettings)
		r.Post("/settings", handlers.apiAddSetting)
		r.Put("/settings", handlers.apiReplaceAllSettings)
		r.Post("/settings/validate", handlers.apiValidateSettings)
		r.Put("/settings/{index}", handlers.apiReplaceSetting)
		r.Delete("/settings/{index}", handlers.apiDeleteSetting)
		r.Post("/reload", handlers.apiReload)
		r.Get("/records", handlers.apiRecords)
	})

	router.Handle(rootURL+"/static/*", http.StripPrefix(rootURL+"/static/", http.FileServerFS(staticFolder)))

	return router
}

// scriptSafeJSON returns the value given as a JSON string literal which is
// safe to embed in the text content of a <script> element. The templates are
// parsed with a text/template, which escapes nothing, and ROOT_URL is not
// validated yet, so the characters which could close the <script> element
// early must be escaped. json.Marshal escapes "<", ">" and "&" as unicode
// escapes by default, and JSON.parse restores them.
func scriptSafeJSON(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil { // cannot happen when marshaling a string
		return `""`
	}

	return string(encoded)
}
