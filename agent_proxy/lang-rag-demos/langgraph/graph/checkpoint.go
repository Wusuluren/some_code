package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Interrupt 是节点发出的"停下来等人"信号。节点把它当 error 返回，
// 运行时保存断点并把状态原样交给调用方，等 Resume 唤醒。
type Interrupt struct {
	Reason string // 给人看的说明
	Ask    any    // 要人确认的内容，例如待执行的工具参数

	Node  string // 由运行时填写
	Step  int
	State State
}

func (e *Interrupt) Error() string {
	return fmt.Sprintf("graph: 等待人工处理：%s", e.Reason)
}

// AsInterrupt 判断错误是否为中断信号。
func AsInterrupt(err error) (*Interrupt, bool) {
	var i *Interrupt
	if errors.As(err, &i) {
		return i, true
	}
	return nil, false
}

// Checkpoint 是一次快照：记录当时在第几个 super-step、执行了哪个节点、状态是什么。
type Checkpoint struct {
	Thread string `json:"thread"`
	Node   string `json:"node"`
	Step   int    `json:"step"`
	State  State  `json:"state"`
}

// Checkpointer 负责快照的存取，是断点续跑和时间旅行回放的基础。
type Checkpointer interface {
	// Save 写入一个普通快照。
	Save(cp Checkpoint) error
	// SavePending 写入中断点，Resume 从这里读。
	SavePending(cp Checkpoint) error
	// LoadPending 返回该会话最近的中断点。
	LoadPending(thread string) (Checkpoint, bool, error)
	// History 返回该会话的全部快照，按 Step 升序。
	History(thread string) ([]Checkpoint, error)
}

// store 是两种实现共用的容器。
type store struct {
	mu      sync.Mutex
	history map[string][]Checkpoint
	pending map[string]Checkpoint
}

func newStore() *store {
	return &store{history: map[string][]Checkpoint{}, pending: map[string]Checkpoint{}}
}

func (s *store) save(cp Checkpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[cp.Thread] = append(s.history[cp.Thread], cp)
}

func (s *store) savePending(cp Checkpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[cp.Thread] = cp
}

func (s *store) loadPending(thread string) (Checkpoint, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, ok := s.pending[thread]
	if ok {
		delete(s.pending, thread)
	}
	return cp, ok
}

// peekPending 读出但不清除中断点，供落盘时使用。
func (s *store) peekPending(thread string) (Checkpoint, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, ok := s.pending[thread]
	return cp, ok
}

func (s *store) list(thread string) []Checkpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Checkpoint(nil), s.history[thread]...)
	sort.Slice(out, func(i, j int) bool { return out[i].Step < out[j].Step })
	return out
}

// MemoryCheckpointer 把快照放在进程内存里，够单测和一次进程内续跑使用。
type MemoryCheckpointer struct{ *store }

// NewMemoryCheckpointer 返回内存版检查点存储。
func NewMemoryCheckpointer() *MemoryCheckpointer {
	return &MemoryCheckpointer{store: newStore()}
}

func (m *MemoryCheckpointer) Save(cp Checkpoint) error        { m.store.save(cp); return nil }
func (m *MemoryCheckpointer) SavePending(cp Checkpoint) error { m.store.savePending(cp); return nil }
func (m *MemoryCheckpointer) LoadPending(t string) (Checkpoint, bool, error) {
	cp, ok := m.store.loadPending(t)
	return cp, ok, nil
}
func (m *MemoryCheckpointer) History(t string) ([]Checkpoint, error) {
	return m.store.list(t), nil
}

// FileCheckpointer 把快照写成 JSON 文件，进程重启后仍能续跑。
type FileCheckpointer struct {
	dir   string
	store *store
}

// NewFileCheckpointer 返回以 dir 为根的检查点存储；dir 会自动创建。
func NewFileCheckpointer(dir string) (*FileCheckpointer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("graph: 创建检查点目录: %w", err)
	}
	f := &FileCheckpointer{dir: dir, store: newStore()}
	if err := f.loadAll(); err != nil {
		return nil, err
	}
	return f, nil
}

// Path 返回某个会话的检查点文件位置。
func (f *FileCheckpointer) Path(thread string) string {
	return filepath.Join(f.dir, sanitize(thread)+".json")
}

type fileDoc struct {
	History []Checkpoint `json:"history"`
	Pending *Checkpoint  `json:"pending,omitempty"`
}

func (f *FileCheckpointer) loadAll() error {
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return fmt.Errorf("graph: 读取检查点目录: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(f.dir, e.Name()))
		if err != nil {
			return fmt.Errorf("graph: 读取 %s: %w", e.Name(), err)
		}
		var doc fileDoc
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("graph: 解析 %s: %w", e.Name(), err)
		}
		for _, cp := range doc.History {
			f.store.save(cp)
		}
		if doc.Pending != nil {
			f.store.savePending(*doc.Pending)
		}
	}
	return nil
}

func (f *FileCheckpointer) flush(thread string) error {
	pending, hadPending := f.store.peekPending(thread)
	doc := fileDoc{History: f.store.list(thread)}
	if hadPending {
		pendingCopy := pending
		doc.Pending = &pendingCopy
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("graph: 序列化检查点: %w", err)
	}
	tmp := f.Path(thread) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("graph: 写临时文件: %w", err)
	}
	if err := os.Rename(tmp, f.Path(thread)); err != nil {
		return fmt.Errorf("graph: 替换检查点文件: %w", err)
	}
	return nil
}

// Save 写入快照并落盘。
func (f *FileCheckpointer) Save(cp Checkpoint) error {
	f.store.save(cp)
	return f.flush(cp.Thread)
}

// SavePending 写入中断点并落盘。
func (f *FileCheckpointer) SavePending(cp Checkpoint) error {
	f.store.savePending(cp)
	return f.flush(cp.Thread)
}

// LoadPending 取出并清除中断点，改完同样落盘。
func (f *FileCheckpointer) LoadPending(thread string) (Checkpoint, bool, error) {
	cp, ok := f.store.loadPending(thread)
	if !ok {
		return Checkpoint{}, false, nil
	}
	if err := f.flush(thread); err != nil {
		return cp, true, err
	}
	return cp, true, nil
}

// History 返回该会话已落盘的快照。
func (f *FileCheckpointer) History(thread string) ([]Checkpoint, error) {
	return f.store.list(thread), nil
}

func sanitize(thread string) string {
	out := make([]rune, 0, len(thread))
	for _, r := range strings.ToLower(thread) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "thread"
	}
	// 上面已经把非安全字符全部换掉，不可能出现 .. 或路径分隔符。
	return string(out)
}
