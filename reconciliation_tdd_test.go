package main

import "testing"

func TestAssignmentEditsConvergeInEveryOrder(t *testing.T) {
	orders := [][]int{
		{0, 1, 2},
		{0, 2, 1},
		{1, 0, 2},
		{1, 2, 0},
		{2, 0, 1},
		{2, 1, 0},
	}

	for _, order := range orders {
		order := order
		t.Run(editOrderName(order), func(t *testing.T) {
			working, target, edits := hardcodedAssignmentFixture(t)
			applyHardcodedEdits(t, working, target, edits, order...)

			if !sameStructuralTree(working, target) {
				t.Fatalf("order %v did not converge to target tree", order)
			}
		})
	}
}

func TestAssignmentOrphansReconcileIntoParent(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)

	// Apply both children while their AssignStmt parent is absent.
	applyHardcodedEdits(t, working, target, edits, 2, 1)

	block := working.find("root.Decls[0].Body")
	if block == nil || len(block.Children) != 2 {
		t.Fatalf("orphan state has %d block children, want 2", len(block.Children))
	}
	if block.Children[0].Value != "1" || block.Children[1].Value != "x" {
		t.Fatalf("orphan order = %q, %q; want 1, x", block.Children[0].Value, block.Children[1].Value)
	}

	// Applying the parent must reuse and reparent both existing children.
	applyHardcodedEdits(t, working, target, edits, 0)

	if !sameStructuralTree(working, target) {
		t.Fatalf("reconciled tree does not match target tree")
	}
}

func TestAssignmentEditApplicationIsIdempotent(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)

	applyHardcodedEdits(t, working, target, edits, 1, 1, 2, 2, 0, 0)

	if !sameStructuralTree(working, target) {
		t.Fatal("reapplying edits changed the final tree")
	}
}

func TestAssignmentCanApplyWithoutReconciliation(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)

	if err := applyStructuralEditWithOptions(working, target, edits[1], ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}
	if err := applyStructuralEditWithOptions(working, target, edits[0], ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}

	block := working.find("root.Decls[0].Body")
	assignment := working.find(edits[0].NodeID)
	identifier := working.find(edits[1].NodeID)
	if block == nil || assignment == nil || identifier == nil {
		t.Fatal("no-reconciliation apply lost a node")
	}
	if assignment.parent != block || identifier.parent != block {
		t.Fatal("no-reconciliation apply unexpectedly moved the orphan")
	}

	if err := applyStructuralEditWithOptions(working, target, edits[0], ApplyOptions{Reconcile: true}); err != nil {
		t.Fatal(err)
	}
	if identifier.parent != assignment || identifier.CurrentParentGlobalID != assignment.GlobalID {
		t.Fatal("explicit reconciliation did not repair the orphan")
	}
}

func TestAssignmentReportsReconciliationCandidates(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	if err := applyStructuralEditWithOptions(working, target, edits[2], ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}
	if err := applyStructuralEditWithOptions(working, target, edits[0], ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}

	candidates := ReconciliationCandidates(working, target, edits[0].NodeID)
	if len(candidates) != 1 {
		t.Fatalf("found %d reconciliation candidates, want 1", len(candidates))
	}
	candidate := candidates[0]
	if candidate.NodeID != edits[2].NodeID || !candidate.CanReconcile {
		t.Fatalf("candidate = %+v, want the orphan literal", candidate)
	}
	if candidate.CurrentParentID != edits[0].ParentID || candidate.OriginalParentID != edits[0].NodeID {
		t.Fatalf("candidate locations = current %q, original %q", candidate.CurrentParentID, candidate.OriginalParentID)
	}

	if err := ApplyReconciliationCandidate(working, target, candidate); err != nil {
		t.Fatal(err)
	}
	literal := working.find(edits[2].NodeID)
	assignment := working.find(edits[0].NodeID)
	if literal == nil || assignment == nil || literal.parent != assignment || literal.CurrentField != "Rhs" {
		t.Fatal("candidate application did not reattach the literal")
	}
}

func TestAssignmentReportsBlockedReconciliation(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	if err := applyStructuralEditWithOptions(working, target, edits[1], ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}

	candidates := ReconciliationCandidates(working, target, edits[0].NodeID)
	if len(candidates) != 1 {
		t.Fatalf("found %d reconciliation candidates, want 1", len(candidates))
	}
	if candidates[0].CanReconcile || candidates[0].Reason != "intended parent is not present" {
		t.Fatalf("candidate = %+v, want blocked candidate", candidates[0])
	}
}

func editOrderName(order []int) string {
	name := ""
	for _, index := range order {
		if name != "" {
			name += "-"
		}
		name += string(rune('0' + index))
	}
	return name
}
