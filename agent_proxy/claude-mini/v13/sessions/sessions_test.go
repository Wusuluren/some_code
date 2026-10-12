package sessions
package sessions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const hist1 = `[{"role":"user","content":[{"type":"text","text":"hello world"}]},{"role":"assistant","content":[{"type":"text","text":"hi"}]}]`
const hist2 = `[{"role":"user","content":[{"type":"text","text":"second session prompt"}]}]`

func writeSession(t *testing.T, s Store, id, body string) {
	t.Helper()
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path(id), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectKey(t *testing.T) {
	k := ProjectKey("/home/dev/My Project")
	if !strings.HasPrefix(k, "My-Project-") {
		t.Fatalf("bad project key: %q", k)
	}
	// Deterministic, and different paths give different keys.
	if ProjectKey("/home/dev/My Project") != k {
		t.Fatal("project key not deterministic")
	}
	if ProjectKey("/tmp/other") == k {
		t.Fatal("distinct cwd must yield distinct key")
	}
}

func TestNewIDFormat(t *testing.T) {
	id := NewID(time.Date(2026, 9, 16, 11, 56, 5, 0, time.UTC))
	if id != "20260916-115605" {
		t.Fatalf("id format: %s", id)
	}
}

func TestListNewestAndResolve(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	writeSession(t, s, "20260101-101010", hist1)
	writeSession(t, s, "20260102-121212", hist2)

	list, err := s.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if list[0].ID != "20260102-121212" {
		t.Fatalf("expected newest first, got %s", list[0].ID)
	}
	if list[0].Preview != "second session prompt" {
		t.Fatalf("preview: %q", list[0].Preview)
	}
	if id, ok := s.Newest(); !ok || id != "20260102-121212" {
		t.Fatalf("newest: %s %v", id, ok)
	}

	// Exact id, unique prefix, none, and ambiguity.
	if id, err := s.Resolve("20260101-101010"); err != nil || id != "20260101-101010" {
		t.Fatalf("exact resolve: %v %s", err, id)
	}
	if id, err := s.Resolve("20260101"); err != nil || id != "20260101-101010" {
		t.Fatalf("prefix resolve: %v %s", err, id)
	}
	if _, err := s.Resolve("19990101"); err == nil {
		t.Fatal("expected no-match error")
	}
	writeSession(t, s, "20260103-000001", hist1)
	writeSession(t, s, "20260103-000002", hist1)
	if _, err := s.Resolve("20260103-0000"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestNextIDAvoidsExistingFile(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	occupied := NewID(time.Now())
	writeSession(t, s, occupied, hist1)
	got := s.NextID()
	if got == occupied {
		t.Fatalf("NextID reused an existing id: %s", got)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, got+".json")); err == nil {
		t.Fatalf("NextID returned a path that already exists: %s", got)
	}
}
