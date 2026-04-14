package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter("warn", &buf)
	l.Info("ignored")
	l.Warn("kept")
	out := buf.String()
	if strings.Contains(out, "ignored") {
		t.Errorf("info message leaked at warn level: %s", out)
	}
	if !strings.Contains(out, "kept") {
		t.Errorf("warn message missing: %s", out)
	}
}

func TestNew_InvalidLevelDefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter("nonsense", &buf)
	l.Info("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected info to be written, got: %s", buf.String())
	}
}
