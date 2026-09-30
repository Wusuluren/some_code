package builtin

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "dir/main.go", false}, // * never crosses a segment
		{"internal/**/*.go", "internal/tool/tool.go", true},
		{"internal/**/*.go", "internal/a/b/c.go", true},
		{"**/*.go", "a.go", true}, // ** swallows zero segments
		{"**/*.go", "x/y/a.go", true},
		{"docs/**", "docs/a/b.md", true},
		{"src/ma?n.go", "src/main.go", true},
		{"src/ma?n.go", "src/multi.go", false},
		{"*.md", "a.md", true},
		{"*.md", "a.go", false},
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.name); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}
