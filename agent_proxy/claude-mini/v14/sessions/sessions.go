package sessions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A Store is a per-project directory of session files. Each file is just the
// conversation history the engine already serialises; the store adds identity
// (a timestamp id), listing, and "resume newest / by prefix" resolution. This
// is v13's take on jingwei's sessions, without inventing a new on-disk format.

// Session is one stored conversation with a little metadata for listing.
type Session struct {
	ID      string
	Preview string
	Created time.Time
	Path    string
}

// Store locates a project's sessions by directory.
type Store struct {
	Dir string
}

// ProjectKey makes a filesystem-safe, collision-resistant directory name from
// the working directory (basename + a short hash of the full path).
func ProjectKey(cwd string) string {
	base := sanitize(filepath.Base(cwd))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "project"
	}
	sum := sha256.Sum256([]byte(cwd))
	return base + "-" + hex.EncodeToString(sum[:])[:12]
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// NewID formats a timestamp id like jingwei's 20260916-1156.
func NewID(t time.Time) string { return t.Format("20060102-150405") }

// Path is the file for a session id.
func (s Store) Path(id string) string { return filepath.Join(s.Dir, id+".json") }

// NextID returns a fresh, unused timestamp id under the store directory.
func (s Store) NextID() string {
	_ = os.MkdirAll(s.Dir, 0o755)
	base := NewID(time.Now())
	id := base
	for i := 1; ; i++ {
		if _, err := os.Stat(s.Path(id)); os.IsNotExist(err) {
			return id
		}
		id = base + "-" + strconv.Itoa(i)
	}
}

// rawMessage is a minimal decode of the engine's history format, enough to
// preview the first real user prompt without importing the api package.
type rawBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type rawMessage struct {
	Role    string     `json:"role"`
	Content []rawBlock `json:"content"`
}

// List returns every session in the directory, newest id first, each with a
// preview of its first user text message.
func (s Store) List() ([]Session, error) {
	matches, err := filepath.Glob(filepath.Join(s.Dir, "*.json"))
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(matches))
	for _, m := range matches {
		id := strings.TrimSuffix(filepath.Base(m), ".json")
		var msgs []rawMessage
		if data, err := os.ReadFile(m); err == nil {
			_ = json.Unmarshal(data, &msgs)
		}
		st, _ := os.Stat(m)
		out = append(out, Session{ID: id, Preview: firstUserText(msgs), Created: st.ModTime(), Path: m})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func firstUserText(msgs []rawMessage) string {
	for _, m := range msgs {
		if m.Role != "user" {
			continue
		}
		for _, b := range m.Content {
			if b.Type == "text" && b.Text != "" {
				if len(b.Text) > 70 {
					return b.Text[:70] + "…"
				}
				return b.Text
			}
		}
	}
	return ""
}

// Newest returns the most recent session id, if any.
func (s Store) Newest() (string, bool) {
	list, _ := s.List()
	if len(list) == 0 {
		return "", false
	}
	return list[0].ID, true
}

// Resolve maps an id or id-prefix to a full session id. Exact match wins;
// otherwise a single prefix match is used; none or many are errors.
func (s Store) Resolve(prefix string) (string, error) {
	list, _ := s.List()
	var matches []string
	for _, ses := range list {
		if ses.ID == prefix {
			return ses.ID, nil
		}
		if strings.HasPrefix(ses.ID, prefix) {
			matches = append(matches, ses.ID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("no session matches %q", prefix)
	default:
		return "", fmt.Errorf("session prefix %q is ambiguous (%d matches)", prefix, len(matches))
	}
}
