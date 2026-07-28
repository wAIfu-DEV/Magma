package lsp

import "testing"

type walkFixture struct {
	Name   string
	Child  *walkFixture
	Parent *walkFixture
}

func TestWalkASTAvoidsBacklinksAndCycles(t *testing.T) {
	root := &walkFixture{Name: "root"}
	orphanedParent := &walkFixture{Name: "skipped-parent"}
	root.Child = &walkFixture{Name: "child", Parent: orphanedParent}
	root.Child.Child = root
	visited := []string{}
	walkAST(root, func(value any) bool {
		if node, ok := value.(*walkFixture); ok {
			visited = append(visited, node.Name)
		}
		return true
	})
	if len(visited) != 2 || visited[0] != "root" || visited[1] != "child" {
		t.Fatalf("visited = %#v, want [root child]", visited)
	}
}

func TestWalkASTStopsWhenVisitorReturnsFalse(t *testing.T) {
	root := &walkFixture{Name: "root", Child: &walkFixture{Name: "child"}}
	visited := 0
	walkAST(root, func(value any) bool {
		if _, ok := value.(*walkFixture); ok {
			visited++
			return false
		}
		return true
	})
	if visited != 1 {
		t.Fatalf("visited %d nodes after stop, want 1", visited)
	}
}
