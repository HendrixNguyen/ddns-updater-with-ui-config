package server

import (
	"context"

	"github.com/qdm12/goservices/httpserver"
)

func New(ctx context.Context, address, rootURL string, db Database,
	settings SettingsManager, logger Logger, runner UpdateForcer,
) (server *httpserver.Server, err error) {
	return httpserver.New(httpserver.Settings{
		Handler: newHandler(ctx, rootURL, db, settings, runner, logger),
		Address: &address,
		Logger:  logger,
	})
}
