package main

import "testing"

func TestStructuralSiblingsConvergeInEveryOrder(t *testing.T) {
	working, target, edits := structuralSiblingFixture()
	orders := [][]int{{0, 1, 2}, {2, 1, 0}, {1, 0, 2}, {1, 2, 0}}

	for _, order := range orders {
		t.Run(editOrderName(order), func(t *testing.T) {
			candidate := working.clone()
			applyHardcodedEdits(t, candidate, target, edits, order...)

			if !sameStructuralTree(candidate, target) {
				t.Fatalf("order %v did not preserve target sibling order", order)
			}
			assertStructuralTreeInvariants(t, candidate)
		})
	}
}

func TestStructuralRepeatedSiblingsKeepTheirIdentity(t *testing.T) {
	working, target, edits := structuralSiblingFixture()

	applyHardcodedEdits(t, working, target, edits, 2, 0, 1)

	if working.find("a") == nil || working.find("b") == nil || working.find("c") == nil {
		t.Fatal("one of the repeated sibling IDs was lost")
	}
	if !sameStructuralTree(working, target) {
		t.Fatal("repeated siblings did not converge to target")
	}
}

func TestStructuralDeleteIsOrderIndependentAndIdempotent(t *testing.T) {
	working, parent, child := structuralDeleteFixture()
	edits := []structuralEdit{
		{Index: 0, Kind: "DELETE", NodeID: parent.ID, NodeKind: parent.Kind},
		{Index: 1, Kind: "DELETE", NodeID: child.ID, NodeKind: child.Kind},
	}

	if err := applyStructuralEdit(working, nil, edits[0]); err != nil {
		t.Fatal(err)
	}
	if err := applyStructuralEdit(working, nil, edits[1]); err != nil {
		t.Fatal(err)
	}
	if err := applyStructuralEdit(working, nil, edits[0]); err != nil {
		t.Fatal(err)
	}
	if len(working.Children) != 1 {
		t.Fatal("deleting a parent should leave the root intact")
	}
}

func TestStructuralUpdateChangesTheSelectedNodeOnly(t *testing.T) {
	working := structuralTestTree(&structuralASTNode{
		ID: "value", Kind: "*ast.BasicLit", Value: "1", Field: "List", Index: 0,
	})
	target := structuralTestTree(&structuralASTNode{
		ID: "value", Kind: "*ast.BasicLit", Value: "2", Field: "List", Index: 0,
	})
	edit := structuralEdit{
		Kind: "UPDATE", NodeID: "value", NodeKind: "*ast.BasicLit",
		ParentID: "root.Body", ParentKind: "*ast.BlockStmt", Field: "List", Position: 0, Value: "2",
	}

	if err := applyStructuralEdit(working, target, edit); err != nil {
		t.Fatal(err)
	}
	if got := working.find("value"); got == nil || got.Value != "2" {
		t.Fatal("update did not change the selected node")
	}
	if len(working.Children) != 1 {
		t.Fatal("update changed unrelated tree structure")
	}
}

func structuralSiblingFixture() (*structuralASTNode, *structuralASTNode, []structuralEdit) {
	working := structuralTestTree()
	target := structuralTestTree(
		&structuralASTNode{ID: "a", Kind: "*ast.Ident", Value: "a", Field: "List", Index: 0},
		&structuralASTNode{ID: "b", Kind: "*ast.Ident", Value: "b", Field: "List", Index: 1},
		&structuralASTNode{ID: "c", Kind: "*ast.Ident", Value: "c", Field: "List", Index: 2},
	)
	edits := []structuralEdit{
		{Index: 0, Kind: "INSERT", NodeID: "a", NodeKind: "*ast.Ident", ParentID: "root.Body", ParentKind: "*ast.BlockStmt", Field: "List", Position: 0, Value: "a"},
		{Index: 1, Kind: "INSERT", NodeID: "b", NodeKind: "*ast.Ident", ParentID: "root.Body", ParentKind: "*ast.BlockStmt", Field: "List", Position: 1, Value: "b"},
		{Index: 2, Kind: "INSERT", NodeID: "c", NodeKind: "*ast.Ident", ParentID: "root.Body", ParentKind: "*ast.BlockStmt", Field: "List", Position: 2, Value: "c"},
	}
	return working, target, edits
}

func structuralDeleteFixture() (*structuralASTNode, *structuralASTNode, *structuralASTNode) {
	child := &structuralASTNode{ID: "child", Kind: "*ast.Ident", Value: "x", Field: "List", Index: 0}
	parent := &structuralASTNode{ID: "parent", Kind: "*ast.AssignStmt", Field: "List", Index: 0, Children: []*structuralASTNode{child}}
	root := structuralTestTree(parent)
	return root, parent, child
}

func structuralTestTree(children ...*structuralASTNode) *structuralASTNode {
	block := &structuralASTNode{ID: "root.Body", Kind: "*ast.BlockStmt", Field: "Body"}
	root := &structuralASTNode{ID: "root", Kind: "*ast.File", Value: "main", Children: []*structuralASTNode{block}}
	block.parent = root
	for _, child := range children {
		insertStructuralNode(block, child, len(block.Children))
	}
	return root
}

func assertStructuralTreeInvariants(t *testing.T, root *structuralASTNode) {
	t.Helper()
	seen := make(map[string]bool)
	var walk func(*structuralASTNode)
	walk = func(node *structuralASTNode) {
		if node == nil {
			return
		}
		if seen[node.ID] {
			t.Fatalf("duplicate node ID %q", node.ID)
		}
		seen[node.ID] = true
		for _, child := range node.Children {
			if child.parent != node {
				t.Fatalf("node %q has incorrect parent", child.ID)
			}
			walk(child)
		}
	}
	walk(root)
}
