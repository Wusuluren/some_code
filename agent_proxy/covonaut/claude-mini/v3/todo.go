package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/covoyage/covonaut/agentcore"
	"github.com/covoyage/covonaut/tui/chat"
	"github.com/covoyage/covonaut/tui/component"
	"github.com/covoyage/covonaut/tui/core"
)

// --- todo state ---

// todoState is the single source of truth for the task list. The model mutates
// it only through the "todo" tool (full-replace semantics); the REPL listener
// and the TUI TodoPanel are the two read-side consumers, wired via OnChange.
var todoState struct {
	mu        sync.RWMutex
	items     []component.TodoItem
	listeners []func()
}

func todoSetItems(newItems []component.TodoItem) {
	todoState.mu.Lock()
	todoState.items = newItems
	notify := make([]func(), len(todoState.listeners))
	copy(notify, todoState.listeners)
	todoState.mu.Unlock()
	for _, fn := range notify {
		fn()
	}
}

func todoItems() []component.TodoItem {
	todoState.mu.RLock()
	defer todoState.mu.RUnlock()
	return append([]component.TodoItem(nil), todoState.items...)
}

func todoHasItems() bool {
	todoState.mu.RLock()
	defer todoState.mu.RUnlock()
	return len(todoState.items) > 0
}

func todoOnChange(fn func()) {
	todoState.mu.Lock()
	todoState.listeners = append(todoState.listeners, fn)
	todoState.mu.Unlock()
}

var todoIcons = map[string]string{
	"pending":     "○",
	"in_progress": "◐",
	"completed":   "●",
	"cancelled":   "✗",
}

// todoSummary renders the list as plain text for the REPL front-end.
func todoSummary() string {
	items := todoItems()
	var b strings.Builder
	done := 0
	fmt.Fprintf(&b, "\n\033[1;36m📋 todo (%d)\033[0m\n", len(items))
	for _, it := range items {
		icon := todoIcons[it.Status]
		if icon == "" {
			icon = "○"
		}
		if it.Status == "completed" {
			done++
		}
		fmt.Fprintf(&b, "  %s %s [%s]\n", icon, it.Content, it.Status)
	}
	fmt.Fprintf(&b, "  \033[2m── %d/%d completed ──\033[0m\n", done, len(items))
	return b.String()
}

// --- the todo tool (model-facing) ---

// newTodoTool builds the model-facing task-list tool. Each call replaces the
// whole list, which keeps the model's bookkeeping trivial and convergence fast.
func newTodoTool() *agentcore.Tool {
	return &agentcore.Tool{
		Name: "todo",
		Description: "Create or replace the task list for the current job. Use it for any task that needs " +
			"more than 3 steps: post the full plan first, then re-send the whole list whenever progress " +
			"changes (keep exactly one item in_progress). Pass an empty items array to clear the list.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type":        "array",
					"description": "The complete new list, replacing any previous items",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":      map[string]any{"type": "string", "description": "Stable id (auto-assigned when omitted)"},
							"content": map[string]any{"type": "string", "description": "The task, one short imperative sentence"},
							"status":  map[string]any{"type": "string", "enum": []any{"pending", "in_progress", "completed", "cancelled"}, "description": "Task status (default pending)"},
						},
						"required": []any{"content"},
					},
				},
			},
			"required": []any{"items"},
		},
		Func: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in struct {
				Items []struct {
					ID      string `json:"id"`
					Content string `json:"content"`
					Status  string `json:"status"`
				} `json:"items"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return resultErrf("invalid arguments: %w", err)
			}
			items := make([]component.TodoItem, 0, len(in.Items))
			for i, raw := range in.Items {
				content := strings.TrimSpace(raw.Content)
				if content == "" {
					return resultErrf("items[%d].content must not be empty", i)
				}
				status := strings.ToLower(strings.TrimSpace(raw.Status))
				if status == "" {
					status = "pending"
				}
				if _, ok := todoIcons[status]; !ok {
					return resultErrf("items[%d].status %q not in pending|in_progress|completed|cancelled", i, raw.Status)
				}
				id := raw.ID
				if strings.TrimSpace(id) == "" {
					id = strconv.Itoa(i + 1)
				}
				items = append(items, component.TodoItem{ID: id, Content: content, Status: status})
			}
			todoSetItems(items)
			if len(items) == 0 {
				return map[string]any{"ok": true, "detail": "todo list cleared"}, nil
			}
			return map[string]any{"ok": true, "detail": fmt.Sprintf("todo list updated: %d items", len(items))}, nil
		},
	}
}

func resultErrf(format string, a ...any) (any, error) {
	return nil, fmt.Errorf(format, a...)
}

// --- TUI footer wiring (stage 2) ---

// todoFooter is a drop-in core.Component for ChatApp.SetFooter: while the model
// has a task list it renders the exported component.TodoPanel (the repo's own
// panel, previously unwired); otherwise it defers to the app's original footer.
// No covonaut source changes — SetFooter is a public extension point.
type todoFooter struct {
	base   core.Component
	panel  *component.TodoPanel
	active func() bool
}

func (f *todoFooter) Render(width int64) []string {
	if f.active() {
		return f.panel.Render(width)
	}
	if f.base != nil {
		return f.base.Render(width)
	}
	return nil
}

func (f *todoFooter) Invalidate() {
	if f.base != nil {
		f.base.Invalidate()
	}
}

// Update lets panel keybindings (↑/↓, toggle) work while the panel is showing;
// otherwise keys fall through to the original footer.
func (f *todoFooter) Update(msg core.Msg) core.Cmd {
	if f.active() {
		if cmd := f.panel.Update(msg); cmd != nil {
			return cmd
		}
	}
	if u, ok := f.base.(core.Updatable); ok {
		return u.Update(msg)
	}
	return nil
}

// bindTodoPanel attaches the TodoPanel to the chat app footer (REPL listener is
// wired separately in main). Reads are pull-based via dataProvider, refreshes
// push panel.Reload through OnChange.
func bindTodoPanel(app *chat.ChatApp) {
	panel := component.NewTodoPanel()
	panel.SetTitle("TODO")
	panel.SetDataProvider(todoItems)
	todoOnChange(panel.Reload)
	app.SetFooter(&todoFooter{base: app.Footer(), panel: panel, active: todoHasItems})
}
