package logging

import (
	"io"
	"log/slog"
	"os"
)

// Init configures the global slog default with the given level and format.
// If w is nil, os.Stderr is used. Format must be "text" or "json".
func Init(level slog.Level, format string, w ...io.Writer) {
	var writer io.Writer = os.Stderr
	if len(w) > 0 && w[0] != nil {
		writer = w[0]
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// New returns a logger with a "component" attribute for module-scoped logging.
func New(component string) *slog.Logger {
	return slog.Default().With(slog.String("component", component))
}
