package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func New(level string, filePaths ...string) *slog.Logger {
	return NewFromHandlers(NewHandler(level, filePaths...))
}

func NewHandler(level string, filePaths ...string) slog.Handler {
	slogLevel := parseLevel(level)
	handlers := []slog.Handler{
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}),
	}

	for _, path := range filePaths {
		if path == "" {
			continue
		}
		writer, err := openLogFile(path)
		if err != nil {
			slog.Warn("open log file failed", slog.String("path", path), slog.String("error", err.Error()))
			continue
		}
		handlers = append(handlers, slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slogLevel}))
	}

	return multiHandler{handlers: handlers}
}

func NewFromHandlers(handlers ...slog.Handler) *slog.Logger {
	return slog.New(multiHandler{handlers: handlers})
}

func NewCallbackHandler(level string, fn func(context.Context, slog.Record) error) slog.Handler {
	return callbackHandler{
		level: parseLevel(level),
		fn:    fn,
	}
}

func CleanupOldFile(path string, retentionDays int) {
	if path == "" || retentionDays <= 0 {
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if time.Since(info.ModTime()) <= time.Duration(retentionDays)*24*time.Hour {
		return
	}
	if err := os.Remove(path); err != nil {
		slog.Warn("remove expired log file failed", slog.String("path", path), slog.String("error", err.Error()))
	}
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func openLogFile(path string) (io.Writer, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

type multiHandler struct {
	handlers []slog.Handler
}

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		next = append(next, handler.WithAttrs(attrs))
	}
	return multiHandler{handlers: next}
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		next = append(next, handler.WithGroup(name))
	}
	return multiHandler{handlers: next}
}

type callbackHandler struct {
	level slog.Level
	fn    func(context.Context, slog.Record) error
}

func (h callbackHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h callbackHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.fn(ctx, record.Clone())
}

func (h callbackHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h callbackHandler) WithGroup(_ string) slog.Handler {
	return h
}
