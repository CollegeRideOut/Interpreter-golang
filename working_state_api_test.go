package main

import "testing"

func TestWorkingStateStartsAtSource(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	state, err := NewWorkingState(working, target, edits)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := state.Snapshot()
	if !sameStructuralTree(snapshot.Root, working) {
		t.Fatal("new working state did not start at source")
	}
	if snapshot.Status[0] != EditUnapplied || snapshot.Status[1] != EditUnapplied || snapshot.Status[2] != EditUnapplied {
		t.Fatalf("initial statuses = %v", snapshot.Status)
	}
}

func TestWorkingStateAppliesWithoutReconciliationAndReconcilesCandidate(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	state, err := NewWorkingState(working, target, edits)
	if err != nil {
		t.Fatal(err)
	}

	if err := state.ApplyWithOptions(1, ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}
	if err := state.ApplyWithOptions(0, ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}

	candidates := state.Candidates(edits[0].NodeID)
	if len(candidates) != 1 || !candidates[0].CanReconcile {
		t.Fatalf("candidates = %+v", candidates)
	}
	if err := state.Reconcile(candidates[0]); err != nil {
		t.Fatal(err)
	}

	snapshot := state.Snapshot()
	identifier := snapshot.Root.find(edits[1].NodeID)
	assignment := snapshot.Root.find(edits[0].NodeID)
	if identifier == nil || assignment == nil || identifier.parent != assignment {
		t.Fatal("API reconciliation did not reattach identifier")
	}
}

func TestWorkingStateRemoveRebuildsRemainingEdits(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	state, err := NewWorkingState(working, target, edits)
	if err != nil {
		t.Fatal(err)
	}

	if err := state.ApplyWithOptions(0, ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}
	if err := state.ApplyWithOptions(1, ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}
	if err := state.Remove(1); err != nil {
		t.Fatal(err)
	}

	snapshot := state.Snapshot()
	assignment := snapshot.Root.find(edits[0].NodeID)
	if assignment == nil || len(assignment.Children) != 0 {
		t.Fatal("removing child did not rebuild the parent-only state")
	}
	if snapshot.Status[0] != EditApplied || snapshot.Status[1] != EditRemoved {
		t.Fatalf("statuses after remove = %v", snapshot.Status)
	}
}

func TestWorkingStateSnapshotIsIndependent(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	state, err := NewWorkingState(working, target, edits)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := state.Snapshot()
	snapshot.Root.Value = "changed outside state"
	if state.Snapshot().Root.Value == "changed outside state" {
		t.Fatal("snapshot exposed mutable working state")
	}
}

func TestWorkingStateRejectsInvalidEditIndex(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	state, err := NewWorkingState(working, target, edits)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ApplyWithOptions(-1, ApplyOptions{}); err == nil {
		t.Fatal("invalid edit index was accepted")
	}
	if err := state.Remove(len(edits)); err == nil {
		t.Fatal("invalid remove index was accepted")
	}
}

func TestWorkingStateReportsInvalidOrphanGoState(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	state, err := NewWorkingState(working, target, edits)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ApplyWithOptions(1, ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}

	report := state.ValidateGo()
	if report.Valid || len(report.Diagnostics) == 0 {
		t.Fatalf("orphan state report = %+v, want invalid with diagnostics", report)
	}
}

func TestWorkingStateReportsValidCompleteGoState(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	state, err := NewWorkingState(working, target, edits)
	if err != nil {
		t.Fatal(err)
	}
	for index := range edits {
		if err := state.Apply(index); err != nil {
			t.Fatal(err)
		}
	}

	report := state.ValidateGo()
	if !report.Valid || len(report.Diagnostics) != 0 {
		t.Fatalf("complete state report = %+v, want valid", report)
	}
}
