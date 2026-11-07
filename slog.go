package caddyferretdb

import (
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

// FerretDB logs with log/slog; Caddy with zap. Its lines go to Caddy's log.
func zapslogLogger(l *zap.Logger) *slog.Logger { return slog.New(zapslog.NewHandler(l.Core())) }
