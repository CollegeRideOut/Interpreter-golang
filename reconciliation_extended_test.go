package main

import (
	"encoding/json"
	"testing"
)

func TestStructuralIdenticalSiblingsKeepIndependentIDs(t *testing.T) {
	working := structuralTestTree()
	target := structuralTestTree(
		&structuralASTNode{ID: "same-a", Kind: "*ast.Ident", Value: "x", Field: "List", Index: 0},
		&structuralASTNode{ID: "same-b", Kind: "*ast.Ident", Value: "x", Field: "List", Index: 1},
	)
	edits := []structuralEdit{
		{Kind: "INSERT", NodeID: "same-a", NodeKind: "*ast.Ident", ParentID: "root.Body", Field: "List", Position: 0},
		{Kind: "INSERT", NodeID: "same-b", NodeKind: "*ast.Ident", ParentID: "root.Body", Field: "List", Position: 1},
	}

	applyHardcodedEdits(t, working, target, edits, 1, 0)

	if working.find("same-a") == nil || working.find("same-b") == nil {
		t.Fatal("identical siblings did not keep independent IDs")
	}
	if !sameStructuralTree(working, target) {
		t.Fatal("identical siblings did not converge to target")
	}
}

func TestStructuralInsertionsPreserveExistingMiddleSibling(t *testing.T) {
	working := structuralTestTree(&structuralASTNode{
		ID: "old", Kind: "*ast.Ident", Value: "old", Field: "List", Index: 0,
	})
	target := structuralTestTree(
		&structuralASTNode{ID: "before", Kind: "*ast.Ident", Value: "before", Field: "List", Index: 0},
		&structuralASTNode{ID: "old", Kind: "*ast.Ident", Value: "old", Field: "List", Index: 1},
		&structuralASTNode{ID: "after", Kind: "*ast.Ident", Value: "after", Field: "List", Index: 2},
	)
	edits := []structuralEdit{
		{Kind: "INSERT", NodeID: "before", NodeKind: "*ast.Ident", ParentID: "root.Body", Field: "List", Position: 0},
		{Kind: "INSERT", NodeID: "after", NodeKind: "*ast.Ident", ParentID: "root.Body", Field: "List", Position: 2},
	}

	applyHardcodedEdits(t, working, target, edits, 1, 0)

	if !sameStructuralTree(working, target) {
		t.Fatal("insertions around existing sibling did not converge")
	}
}

func TestStructuralUpdateAfterSiblingInsertion(t *testing.T) {
	working := structuralTestTree(&structuralASTNode{
		ID: "value", Kind: "*ast.BasicLit", Value: "1", Field: "List", Index: 0,
	})
	target := structuralTestTree(
		&structuralASTNode{ID: "before", Kind: "*ast.Ident", Value: "before", Field: "List", Index: 0},
		&structuralASTNode{ID: "value", Kind: "*ast.BasicLit", Value: "2", Field: "List", Index: 1},
	)
	edits := []structuralEdit{
		{Kind: "INSERT", NodeID: "before", NodeKind: "*ast.Ident", ParentID: "root.Body", Field: "List", Position: 0},
		{Kind: "UPDATE", NodeID: "value", NodeKind: "*ast.BasicLit", ParentID: "root.Body", Field: "List", Position: 1, Value: "2"},
	}

	applyHardcodedEdits(t, working, target, edits, 0, 1)

	if !sameStructuralTree(working, target) {
		t.Fatal("update after sibling insertion did not converge")
	}
}

func TestStructuralDeleteReindexesRemainingSiblings(t *testing.T) {
	working := structuralTestTree(
		&structuralASTNode{ID: "a", Kind: "*ast.Ident", Value: "a", Field: "List", Index: 0},
		&structuralASTNode{ID: "b", Kind: "*ast.Ident", Value: "b", Field: "List", Index: 1},
		&structuralASTNode{ID: "c", Kind: "*ast.Ident", Value: "c", Field: "List", Index: 2},
	)
	target := structuralTestTree(
		&structuralASTNode{ID: "a", Kind: "*ast.Ident", Value: "a", Field: "List", Index: 0},
		&structuralASTNode{ID: "c", Kind: "*ast.Ident", Value: "c", Field: "List", Index: 1},
	)
	edit := structuralEdit{Kind: "DELETE", NodeID: "b", NodeKind: "*ast.Ident"}

	if err := applyStructuralEdit(working, nil, edit); err != nil {
		t.Fatal(err)
	}
	if !sameStructuralTree(working, target) {
		t.Fatal("remaining siblings were not reindexed after deletion")
	}
}

func TestStructuralJSONRoundTripRestoresNavigableTree(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	applyHardcodedEdits(t, working, target, edits, 1)

	data, err := json.Marshal(working)
	if err != nil {
		t.Fatal(err)
	}
	var restored structuralASTNode
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	restoreStructuralParents(&restored, nil)

	if !sameStructuralTree(working, &restored) {
		t.Fatal("JSON round trip changed intermediate tree")
	}
	if err := applyStructuralEdit(&restored, target, edits[0]); err != nil {
		t.Fatal(err)
	}
	if restored.find(edits[1].NodeID) == nil {
		t.Fatal("reloaded tree lost the orphan identifier")
	}
}

func TestStructuralOwnershipTracksDesiredAndCurrentParents(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	assignment := target.find(edits[0].NodeID)
	identifier := target.find(edits[1].NodeID)
	block := working.find("root.Decls[0].Body")
	if assignment == nil || identifier == nil || block == nil {
		t.Fatal("fixture is missing assignment, identifier, or block")
	}

	applyHardcodedEdits(t, working, target, edits, 1)
	current := working.find(edits[1].NodeID)
	if current == nil {
		t.Fatal("identifier was not materialized")
	}
	if current.GlobalID != identifier.GlobalID {
		t.Fatal("orphan changed its global identity")
	}
	if current.OriginalParentGlobalID != assignment.GlobalID {
		t.Fatal("orphan lost its desired parent identity")
	}
	if current.CurrentParentGlobalID != block.GlobalID || current.CurrentField != "List" {
		t.Fatal("orphan did not record its current parent")
	}

	applyHardcodedEdits(t, working, target, edits, 0)
	current = working.find(edits[1].NodeID)
	if current.CurrentParentGlobalID != assignment.GlobalID || current.CurrentField != "Lhs" {
		t.Fatal("reconciliation did not update current ownership")
	}
	if current.OriginalParentGlobalID != assignment.GlobalID {
		t.Fatal("reconciliation changed original ownership")
	}
}

func TestStructuralGlobalIDLookupSurvivesPathChanges(t *testing.T) {
	working := structuralTestTree(&structuralASTNode{
		ID: "root.Body.List[0]", GlobalID: "node-value", Kind: "*ast.BasicLit", Value: "1", Field: "List", Index: 0,
	})
	target := structuralTestTree(&structuralASTNode{
		ID: "root.Body.List[1]", GlobalID: "node-value", Kind: "*ast.BasicLit", Value: "2", Field: "List", Index: 1,
	})
	edit := structuralEdit{
		Kind: "UPDATE", NodeID: "root.Body.List[1]", NodeGlobalID: "node-value", SourceGlobalID: "node-value",
		NodeKind: "*ast.BasicLit", ParentID: "root.Body", Field: "List", Position: 1, Value: "2",
	}

	if err := applyStructuralEdit(working, target, edit); err != nil {
		t.Fatal(err)
	}
	node := working.findGlobal("node-value")
	if node == nil || node.Value != "2" || node.ID != "root.Body.List[0]" {
		t.Fatalf("global lookup did not update the original node: %+v", node)
	}
}
