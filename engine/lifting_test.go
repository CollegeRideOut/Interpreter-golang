package engine_test

import (
	"testing"

	"interpreter/engine"
)

func TestProjectEditsHidesShellsWithoutChangingCanonicalEdits(t *testing.T) {
	edits := []engine.Edit{
		{NodeID: "statement", NodeKind: "*ast.ExprStmt"},
		{NodeID: "call", NodeKind: "*ast.CallExpr", ParentID: "statement"},
		{NodeID: "name", NodeKind: "*ast.Ident", ParentID: "call"},
	}
	views := engine.ProjectEdits(edits, engine.LiftOptions{HiddenKinds: map[string]bool{"*ast.ExprStmt": true}})
	if len(views) != 2 || views[0].EditIndex != 1 || views[0].Depth != 0 || views[1].EditIndex != 2 || views[1].Depth != 1 {
		t.Fatalf("projected views = %+v", views)
	}
	if edits[0].NodeID != "statement" || edits[1].ParentID != "statement" {
		t.Fatal("projection mutated canonical edits")
	}
}

func TestRemoveProjectedRemovesEmptyHiddenAncestor(t *testing.T) {
	state, err := engine.NewWorkingState(
		&engine.Node{ID: "root", Kind: "*ast.File"},
		&engine.Node{ID: "root", Kind: "*ast.File", Children: []*engine.Node{
			{ID: "shell", Kind: "*ast.ExprStmt", Field: "Decls", Children: []*engine.Node{
				{ID: "child", Kind: "*ast.Ident", Value: "x", Field: "X"},
			}},
		}},
		[]engine.Edit{
			{Index: 0, Kind: "INSERT", NodeID: "shell", NodeKind: "*ast.ExprStmt", ParentID: "root", Field: "Decls", Node: &engine.Node{ID: "shell", Kind: "*ast.ExprStmt", Field: "Decls", Children: []*engine.Node{{ID: "child", Kind: "*ast.Ident", Value: "x", Field: "X"}}}},
			{Index: 1, Kind: "INSERT", NodeID: "child", NodeKind: "*ast.Ident", ParentID: "shell", Field: "X", Node: &engine.Node{ID: "child", Kind: "*ast.Ident", Value: "x", Field: "X"}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ApplySubtree(0); err != nil {
		t.Fatal(err)
	}
	if err := state.RemoveProjected(1, engine.LiftOptions{HiddenKinds: map[string]bool{"*ast.ExprStmt": true}}); err != nil {
		t.Fatal(err)
	}
	if len(state.Snapshot().Root.Children) != 0 {
		t.Fatal("empty hidden ancestor remained after projected removal")
	}
}
