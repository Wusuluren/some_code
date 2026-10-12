package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func setup(t *testing.T) string {
	t.Helper()
	resetLedger()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func TestStaleWriteRefused(t *testing.T) {
	dir := setup(t)
	p := filepath.Join(dir, "a.txt")
	w := NewWriteTool()

	// Fresh file: no prior sighting, so the write is allowed and recorded.
	if _, err := w.Call(context.Background(), mustJSON(t, map[string]interface{}{"file_path": p, "content": "v1"})); err != nil {
		t.Fatalf("initial write: %v", err)
	}
	// Same content again is fine (ledger matches disk).
	if _, err := w.Call(context.Background(), mustJSON(t, map[string]interface{}{"file_path": p, "content": "v2"})); err != nil {
		t.Fatalf("second write: %v", err)
	}
	// An external editor changes the file behind our back.
	if err := os.WriteFile(p, []byte("someone-else"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := w.Call(context.Background(), mustJSON(t, map[string]interface{}{"file_path": p, "content": "v3"}))
	if err == nil || !strings.Contains(err.Error(), "re-read") {
		t.Fatalf("expected stale-write refusal, got %v", err)
	}
	// On disk it is still the external content — we refused, never clobbered.
	if data, _ := os.ReadFile(p); string(data) != "someone-else" {
		t.Fatalf("refused write must not touch disk: %q", data)
	}
}

func TestMultiEditAllOrNothing(t *testing.T) {
	dir := setup(t)
	p := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(p, []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// We must have "seen" the file for a clean (non-stale) edit.
	NewReadTool().Call(context.Background(), mustJSON(t, map[string]interface{}{"file_path": p}))

	e := NewEditTool()
	// Second pair won't be found => the whole batch must not commit.
	in := map[string]interface{}{"file_path": p, "edits": []map[string]string{
		{"old_string": "alpha", "new_string": "ALPHA"},
		{"old_string": "does-not-exist", "new_string": "X"},
	}}
	if _, err := e.Call(context.Background(), mustJSON(t, in)); err == nil {
		t.Fatal("expected batch to fail on the missing edit")
	}
	if data, _ := os.ReadFile(p); string(data) != "alpha\nbeta\n" {
		t.Fatalf("all-or-nothing violated, file changed: %q", data)
	}

	// Now a valid batch commits both edits at once.
	in2 := map[string]interface{}{"file_path": p, "edits": []map[string]string{
		{"old_string": "alpha", "new_string": "ALPHA"},
		{"old_string": "beta", "new_string": "BETA"},
	}}
	if _, err := e.Call(context.Background(), mustJSON(t, in2)); err != nil {
		t.Fatalf("valid batch: %v", err)
	}
	if data, _ := os.ReadFile(p); string(data) != "ALPHA\nBETA\n" {
		t.Fatalf("batch not applied: %q", data)
	}
}

func TestAmbiguousEditReportsLineNumbers(t *testing.T) {
	dir := setup(t)
	p := filepath.Join(dir, "c.txt")
	if err := os.WriteFile(p, []byte("dup\nmid\ndup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	NewReadTool().Call(context.Background(), mustJSON(t, map[string]interface{}{"file_path": p}))

	_, err := NewEditTool().Call(context.Background(), mustJSON(t, map[string]interface{}{
		"file_path": p, "old_string": "dup", "new_string": "x",
	}))
	if err == nil || !strings.Contains(err.Error(), "lines 1, 3") {
		t.Fatalf("expected line numbers 1,3 in ambiguity error, got %v", err)
	}
}

func TestAtomicWriteLeavesNoTempFile(t *testing.T) {
	dir := setup(t)
	p := filepath.Join(dir, "d.txt")
	if _, err := NewWriteTool().Call(context.Background(), mustJSON(t, map[string]interface{}{"file_path": p, "content": "hello"})); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("temp file left behind: %v", names)
	}
}
