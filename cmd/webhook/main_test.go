package main

import (
	"log/slog"
	"testing"
)

func TestCreateLogger_KnownFormats(t *testing.T) {
	for _, format := range []string{"text", "TEXT", "json", "JSON"} {
		t.Run(format, func(t *testing.T) {
			l := createLogger(Options{LogFormat: format})
			if l == nil {
				t.Fatal("expected logger, got nil")
			}
			l.Info("hello")
		})
	}
}

func TestCreateLogger_UnknownFormatDoesNotPanic(t *testing.T) {
	for _, format := range []string{"", "garbage", "yaml"} {
		t.Run(format, func(t *testing.T) {
			l := createLogger(Options{LogFormat: format})
			if l == nil {
				t.Fatal("expected logger for unknown format, got nil")
			}
			l.Info("hello")
		})
	}
}

func TestDetermineLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"garbage", slog.LevelInfo},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := determineLogLevel(Options{LogLevel: tc.in})
			if got.Level() != tc.want {
				t.Errorf("got %v want %v", got.Level(), tc.want)
			}
		})
	}
}
