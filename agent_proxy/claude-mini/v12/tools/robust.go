package tools
package tools

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// v12 robust file writes, ported from jingwei's two ideas that "pay for
// themselves":
//   - an atomic write (temp file + rename) so a crash never leaves a
//     truncated file behind; and
//   - a ledger of the bytes we last saw at each path, so if the file on disk
//     is no longer those bytes (the user's editor, a formatter, another
//     agent changed it) the write is refused and the model is told to re-read
//     first. It is deliberately NOT a "read-before-edit" gate: a file we have
//     never seen is written as before.

var (
	ledgerMu sync.Mutex
	ledger   = map[string][]byte{}
)

// noteRead records the full bytes last seen at path (a whole-file Read).
func noteRead(path string, data []byte) {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	ledger[path] = append([]byte(nil), data...)
}

// checkStale refuses a write when we previously saw path but the current disk
// bytes differ from what we recorded. An unseen path is always allowed.
func checkStale(path string, current []byte) error {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	seen, ok := ledger[path]
	if !ok {
		return nil
	}
	if !bytes.Equal(seen, current) {
		return fmt.Errorf("file changed since you last read it: %s — re-read it before writing", path)
	}
	return nil
}

// commitWrite records that the bytes we last saw at path are now content.
func commitWrite(path string, content []byte) {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	ledger[path] = append([]byte(nil), content...)
}

// resetLedger clears the ledger (test helper).
func resetLedger() {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	ledger = map[string][]byte{}
}

// atomicWrite writes data to path via a temp file in the same directory plus
// a rename, so readers never observe a half-written file.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".atomic-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename below succeeds
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
