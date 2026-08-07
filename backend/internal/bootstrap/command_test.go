package bootstrap

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/platform/buildinfo"
)

func TestExecuteVersion(t *testing.T) {
	t.Parallel()

	want := buildinfo.New("1.2.3", "abc123", "2026-08-06T00:00:00Z")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := Execute([]string{"version"}, &stdout, &stderr, want); code != 0 {
		t.Fatalf("Execute() code = %d, stderr = %q", code, stderr.String())
	}

	var got buildinfo.Info
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got != want {
		t.Fatalf("version output = %#v, want %#v", got, want)
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	info := buildinfo.New("dev", "unknown", "unknown")

	if code := Execute([]string{"unknown"}, &stdout, &stderr, info); code != 2 {
		t.Fatalf("Execute() code = %d, want 2", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("Execute() did not explain the invalid command")
	}
}
