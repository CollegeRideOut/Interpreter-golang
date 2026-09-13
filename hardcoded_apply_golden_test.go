package main

import (
	"testing"

	"interpreter/engine"
)

func TestHardcodedApplyGoldenSnapshots(t *testing.T) {
	current := goldenNode("root", "*ast.File", "", "", 0)
	current.Children = []*engine.Node{goldenNode("root.Name", "*ast.Ident", "main", "Name", 0)}
	desired := goldenNode("root", "*ast.File", "", "", 0)
	desired.Children = []*engine.Node{
		goldenNode("root.Name", "*ast.Ident", "main", "Name", 0),
		goldenNode("root.Decls[0]", "*ast.ExprStmt", "", "Decls", 0),
	}
	call := goldenNode("root.Decls[0].X", "*ast.CallExpr", "", "X", 0)
	name := goldenNode("root.Decls[0].X.Fun", "*ast.Ident", "println", "Fun", 0)
	call.Children = []*engine.Node{name}
	desired.Children[1].Children = []*engine.Node{call}

	edits := []engine.Edit{
		{Kind: "INSERT", NodeID: "root.Decls[0]", NodeKind: "*ast.ExprStmt", ParentID: "root", Field: "Decls", Position: 0, Node: desired.Children[1]},
		{Kind: "INSERT", NodeID: call.ID, NodeKind: call.Kind, ParentID: desired.Children[1].ID, Field: "X", Position: 0, Node: call},
		{Kind: "INSERT", NodeID: name.ID, NodeKind: name.Kind, ParentID: call.ID, Field: "Fun", Position: 0, Node: name},
	}
	state, err := engine.NewWorkingState(current, desired, edits)
	if err != nil {
		t.Fatal(err)
	}
	wants := []string{
		`{"kind":"*ast.File","index":0,"children":[{"kind":"*ast.Ident","value":"main","field":"Name","index":0},{"kind":"*ast.ExprStmt","field":"Decls","index":0}]}`,
		`{"kind":"*ast.File","index":0,"children":[{"kind":"*ast.Ident","value":"main","field":"Name","index":0},{"kind":"*ast.ExprStmt","field":"Decls","index":0,"children":[{"kind":"*ast.CallExpr","field":"X","index":0}]}]}`,
		`{"kind":"*ast.File","index":0,"children":[{"kind":"*ast.Ident","value":"main","field":"Name","index":0},{"kind":"*ast.ExprStmt","field":"Decls","index":0,"children":[{"kind":"*ast.CallExpr","field":"X","index":0,"children":[{"kind":"*ast.Ident","value":"println","field":"Fun","index":0}]}]}]}`,
	}
	for index, want := range wants {
		if err := state.Apply(index); err != nil {
			t.Fatalf("edit %d: %v", index, err)
		}
		got := nodeShape(state.Snapshot().Root)
		if got != want {
			t.Fatalf("edit %d produced unexpected AST shape\ngot:  %s\nwant: %s", index, got, want)
		}
	}
}

func TestHardcodedParentDeleteMakesChildDeleteNoOp(t *testing.T) {
	current := goldenNode("root", "*ast.File", "", "", 0)
	parent := goldenNode("root.Decls[0]", "*ast.ExprStmt", "", "Decls", 0)
	child := goldenNode("root.Decls[0].X", "*ast.Ident", "old", "X", 0)
	parent.Children = []*engine.Node{child}
	current.Children = []*engine.Node{parent}
	desired := goldenNode("root", "*ast.File", "", "", 0)
	edits := []engine.Edit{
		{Kind: "DELETE", NodeID: parent.ID, NodeKind: parent.Kind, SourceGlobalID: parent.GlobalID, ParentID: current.ID, Field: parent.Field, Position: 0, Node: parent},
		{Kind: "DELETE", NodeID: child.ID, NodeKind: child.Kind, SourceGlobalID: child.GlobalID, ParentID: parent.ID, Field: child.Field, Position: 0, Node: child},
	}
	state, err := engine.NewWorkingState(current, desired, edits)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Apply(0); err != nil {
		t.Fatal(err)
	}
	if got := nodeShape(state.Snapshot().Root); got != `{"kind":"*ast.File","index":0}` {
		t.Fatalf("after parent delete = %s", got)
	}
	if err := state.Apply(1); err != nil {
		t.Fatal(err)
	}
	if got := nodeShape(state.Snapshot().Root); got != `{"kind":"*ast.File","index":0}` {
		t.Fatalf("after dependent child delete = %s", got)
	}
}

func goldenNode(id, kind, value, field string, index int) *engine.Node {
	return &engine.Node{ID: id, GlobalID: "global:" + id, Kind: kind, Value: value, Field: field, Index: index}
}
