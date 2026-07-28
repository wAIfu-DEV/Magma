package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const completionMatrixDependency = `mod dependency
pub Thing(value u64)
pub Holder(item Thing)

Thing.touch() Thing:
    ret *this
..

pub make() Thing:
    ret Thing(value=1)
..

pub open() !Thing:
    ret Thing(value=1)
..

pub consume(value Thing) void:
..
`

func TestCompletionContextMatrix(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "module standalone", body: "    dep.|", want: []string{"Thing", "make", "open"}},
		{name: "module prefix", body: "    dep.ma|", want: []string{"make"}},
		{name: "module inferred assignment", body: "    item := dep.|", want: []string{"Thing", "make"}},
		{name: "module destructuring", body: "    item, err := dep.|", want: []string{"make", "open"}},
		{name: "module call argument", body: "    dep.consume(dep.|)", want: []string{"Thing", "make"}},
		{name: "typed local", body: "    item dep.Thing\n    item.|", want: []string{"touch", "value"}},
		{name: "inferred call local", body: "    item := dep.make()\n    item.|", want: []string{"touch", "value"}},
		{name: "destructured call local", body: "    item, err := dep.open()\n    item.|", want: []string{"touch", "value"}},
		{name: "invalid try destructured local", body: "    item, err := try dep.open()\n    item.|", want: []string{"touch", "value"}},
		{name: "field chain", body: "    holder dep.Holder\n    holder.item.|", want: []string{"touch", "value"}},
		{name: "method result", body: "    item dep.Thing\n    item.touch().|", want: []string{"touch", "value"}},
		{name: "module function result", body: "    dep.make().|", want: []string{"touch", "value"}},
		{name: "function result in argument", body: "    dep.consume(dep.make().|)", want: []string{"touch", "value"}},
		{name: "other trailing selector before", body: "    dep.\n    item := dep.make()\n    item.|", want: []string{"touch", "value"}},
		{name: "other trailing selector after", body: "    item := dep.make()\n    item.|\n    dep.", want: []string{"touch", "value"}},
		{name: "other argument selector", body: "    dep.consume(dep.make().)\n    item := dep.make()\n    item.|", want: []string{"touch", "value"}},
		{name: "active selector with other invalid try", body: "    item, err := try dep.open()\n    dep.|", want: []string{"Thing", "make", "open"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			dependency := filepath.Join(directory, "dependency.mg")
			if err := os.WriteFile(dependency, []byte(completionMatrixDependency), 0o600); err != nil {
				t.Fatal(err)
			}
			source := "mod completion\nuse \"./dependency.mg\" dep\nmain() void:\n" + test.body + "\n..\n"
			marker := strings.Index(source, "|")
			if marker < 0 || strings.Count(source, "|") != 1 {
				t.Fatalf("source must contain exactly one cursor marker: %q", source)
			}
			before := source[:marker]
			line := uint32(strings.Count(before, "\n"))
			lastNewline := strings.LastIndex(before, "\n")
			character := uint32(len([]rune(before[lastNewline+1:])))
			source = source[:marker] + source[marker+1:]
			path := filepath.Join(directory, "completion.mg")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			items := complete("file:///"+filepath.ToSlash(path), source, position{Line: line, Character: character}, testStdRoot())
			labels := map[string]bool{}
			for _, item := range items {
				labels[item.Label] = true
			}
			for _, want := range test.want {
				if !labels[want] {
					t.Errorf("completion labels %v do not include %q", labels, want)
				}
			}
		})
	}
}

func TestExpressionCompletionWithOtherIncompleteStatements(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "after while keyword",
			body: "    value, err := try dep.open()\n    local := arg\n    while |\n    if ",
		},
		{
			name: "after if keyword",
			body: "    value, err := try dep.open()\n    local := arg\n    while \n    if |",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			dependency := filepath.Join(directory, "dependency.mg")
			if err := os.WriteFile(dependency, []byte(completionMatrixDependency), 0o600); err != nil {
				t.Fatal(err)
			}
			source := "mod completion\nuse \"./dependency.mg\" dep\nconst GLOBAL u64 = 1\nmain(arg u64) void:\n" + test.body + "\n..\n"
			marker := strings.Index(source, "|")
			before := source[:marker]
			line := uint32(strings.Count(before, "\n"))
			lastNewline := strings.LastIndex(before, "\n")
			character := uint32(len([]rune(before[lastNewline+1:])))
			source = source[:marker] + source[marker+1:]
			path := filepath.Join(directory, "completion.mg")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			items := complete("file:///"+filepath.ToSlash(path), source, position{Line: line, Character: character}, testStdRoot())
			labels := map[string]bool{}
			for _, item := range items {
				labels[item.Label] = true
			}
			for _, want := range []string{"GLOBAL", "arg", "local", "value", "err", "dep"} {
				if !labels[want] {
					t.Errorf("completion labels %v do not include %q", labels, want)
				}
			}
		})
	}
}
