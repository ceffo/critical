package main

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunLogsFatalStartupErrorAtErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	code := run(func() error { return errors.New("unknown migration") })

	assert.Equal(t, 1, code)
	assert.Contains(t, buf.String(), "level=ERROR")
	assert.Contains(t, buf.String(), "unknown migration")
}

func TestRunReturnsZeroOnCleanShutdown(t *testing.T) {
	assert.Equal(t, 0, run(func() error { return nil }))
}
