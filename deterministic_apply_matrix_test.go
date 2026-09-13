package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"interpreter/engine"
)

type applyOrder struct {
	Name    string
	Indices []int
}

func TestDeterministicApplyOrderMatrix(t *testing.T) {
	source, target, edits := testProgramDiff(t)
	orders := []applyOrder{
		{Name: "diff-order", Indices: identityOrder(len(edits))},
		{Name: "reverse-order", Indices: reverseOrder(len(edits))},
		{Name: "parents-first", Indices: dependencyOrder(edits, false)},
		{Name: "children-first", Indices: dependencyOrder(edits, true)},
	}
	for _, seed := range []int64{7, 41, 2026, 9001} {
		indices := identityOrder(len(edits))
		rand.New(rand.NewSource(seed)).Shuffle(len(indices), func(i, j int) {
			indices[i], indices[j] = indices[j], indices[i]
		})
		orders = append(orders, applyOrder{Name: fmt.Sprintf("mixed-seed-%d", seed), Indices: indices})
	}

	for _, order := range orders {
		order := order
		t.Run(order.Name, func(t *testing.T) {
			state, err := engine.NewWorkingState(source, target, edits)
			if err != nil {
				t.Fatal(err)
			}
			for step, editIndex := range order.Indices {
				before := state.Snapshot()
				if err := state.Apply(editIndex); err != nil {
					t.Fatalf("step %d, edit %d: apply failed: %v", step, editIndex, err)
				}
				after := state.Snapshot()
				assertEditIntent(t, step, editIndex, edits, after)
				if after.Status[editIndex] != engine.EditApplied {
					t.Fatalf("step %d, edit %d: status = %q, want applied", step, editIndex, after.Status[editIndex])
				}
				if editChangedShape(before.Root, after.Root) {
					continue
				}
				if !expectedNoOp(editIndex, edits, before) {
					t.Fatalf("step %d, edit %d (%s %s) was an unexpected structural no-op; before and after JSON are identical:\n%s", step, editIndex, edits[editIndex].Kind, edits[editIndex].NodeKind, nodeJSON(t, before.Root))
				}
			}
			final := state.Snapshot()
			if got, want := nodeShape(final.Root), nodeShape(target); got != want {
				t.Fatalf("final shape does not match target for %s, order=%v: %s\ngot %d bytes, want %d bytes; first JSON difference: %s", order.Name, order.Indices, firstShapeDifference(shape(final.Root), shape(target), "root"), len(got), len(want), firstJSONDifference(got, want))
			}
		})
	}
}

func TestSecondEditHasInspectableIntermediateASTChange(t *testing.T) {
	source, target, edits := testProgramDiff(t)
	if len(edits) < 2 {
		t.Fatal("expected at least two edits")
	}
	state, err := engine.NewWorkingState(source, target, edits)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Apply(0); err != nil {
		t.Fatal(err)
	}
	first := state.Snapshot()
	if err := state.Apply(1); err != nil {
		t.Fatal(err)
	}
	second := state.Snapshot()
	if !editChangedShape(first.Root, second.Root) {
		t.Fatalf("edit 1 (%s %s) produced no JSON AST shape change; before:\n%s\nafter:\n%s", edits[1].Kind, edits[1].NodeKind, nodeJSON(t, first.Root), nodeJSON(t, second.Root))
	}
}

func testProgramDiff(t *testing.T) (*engine.Node, *engine.Node, []engine.Edit) {
	t.Helper()
	directory := "TestProgram"
	target, err := os.ReadFile(filepath.Join(directory, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := gitFile(directory, "HEAD~1:main.go")
	if err != nil {
		t.Fatal(err)
	}
	sourceTree, targetTree, edits, err := engine.Diff(source, target)
	if err != nil {
		t.Fatal(err)
	}
	return sourceTree, targetTree, edits
}

func identityOrder(count int) []int {
	indices := make([]int, count)
	for i := range indices {
		indices[i] = i
	}
	return indices
}

func reverseOrder(count int) []int {
	indices := identityOrder(count)
	for left, right := 0, count-1; left < right; left, right = left+1, right-1 {
		indices[left], indices[right] = indices[right], indices[left]
	}
	return indices
}

func dependencyOrder(edits []engine.Edit, childrenFirst bool) []int {
	byNode := make(map[string]int, len(edits))
	for index, edit := range edits {
		byNode[edit.NodeID] = index
	}
	depth := make([]int, len(edits))
	var getDepth func(int) int
	getDepth = func(index int) int {
		if depth[index] > 0 {
			return depth[index]
		}
		parent, ok := byNode[edits[index].ParentID]
		if !ok || parent == index {
			depth[index] = 1
			return depth[index]
		}
		depth[index] = getDepth(parent) + 1
		return depth[index]
	}
	indices := identityOrder(len(edits))
	for _, index := range indices {
		getDepth(index)
	}
	for left := range indices {
		for right := left + 1; right < len(indices); right++ {
			if (childrenFirst && depth[indices[right]] > depth[indices[left]]) || (!childrenFirst && depth[indices[right]] < depth[indices[left]]) {
				indices[left], indices[right] = indices[right], indices[left]
			}
		}
	}
	return indices
}

func assertEditIntent(t *testing.T, step, index int, edits []engine.Edit, snapshot engine.WorkingSnapshot) {
	t.Helper()
	edit := edits[index]
	target := edit.Node
	if edit.Kind == "DELETE" {
		if findNodeByGlobal(snapshot.Root, edit.SourceGlobalID) != nil {
			t.Fatalf("step %d, delete edit %d: source node %s is still present", step, index, edit.SourceGlobalID)
		}
		return
	}
	current := findNodeByGlobal(snapshot.Root, edit.NodeGlobalID)
	if current == nil {
		t.Fatalf("step %d, edit %d: target node %s is absent after apply", step, index, edit.NodeID)
	}
	if current.Kind != target.Kind || current.Value != target.Value {
		t.Fatalf("step %d, edit %d: node %s = (%s, %q), want (%s, %q)", step, index, edit.NodeID, current.Kind, current.Value, target.Kind, target.Value)
	}
	if parent := findNodeByGlobal(snapshot.Root, edit.ParentGlobalID); parent != nil && (current.CurrentParentID != parent.ID || current.Field != target.Field) {
		t.Fatalf("step %d, edit %d: node %s is at parent %s/%s, want %s/%s", step, index, edit.NodeID, current.CurrentParentID, current.Field, parent.ID, target.Field)
	}
}

func expectedNoOp(index int, edits []engine.Edit, before engine.WorkingSnapshot) bool {
	edit := edits[index]
	if edit.Kind == "DELETE" {
		return findNodeByGlobal(before.Root, edit.SourceGlobalID) == nil || hasAppliedAncestor(index, edits, before)
	}
	return hasAppliedAncestor(index, edits, before)
}

func hasAppliedAncestor(index int, edits []engine.Edit, snapshot engine.WorkingSnapshot) bool {
	byNode := make(map[string]int, len(edits))
	for editIndex, edit := range edits {
		byNode[edit.NodeID] = editIndex
	}
	for parentID := edits[index].ParentID; parentID != ""; {
		parentIndex, ok := byNode[parentID]
		if !ok {
			return false
		}
		if snapshot.Status[parentIndex] == engine.EditApplied {
			return true
		}
		parentID = edits[parentIndex].ParentID
	}
	return false
}

func findNodeByGlobal(root *engine.Node, globalID string) *engine.Node {
	if root == nil || globalID == "" {
		return nil
	}
	if root.GlobalID == globalID {
		return root
	}
	for _, child := range root.Children {
		if found := findNodeByGlobal(child, globalID); found != nil {
			return found
		}
	}
	return nil
}

func nodeShape(root *engine.Node) string {
	return mustJSONValue(shape(root))
}

func nodeJSON(t *testing.T, root *engine.Node) string {
	t.Helper()
	return mustJSON(t, root)
}

func mustJSONValue(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func editChangedShape(before, after *engine.Node) bool {
	return nodeShape(before) != nodeShape(after)
}

func firstShapeDifference(got, want structuralShape, path string) string {
	if got.Kind != want.Kind || got.Value != want.Value || got.Field != want.Field || got.Index != want.Index {
		return fmt.Sprintf("at %s got (%s,%q,%s,%d), want (%s,%q,%s,%d)", path, got.Kind, got.Value, got.Field, got.Index, want.Kind, want.Value, want.Field, want.Index)
	}
	if len(got.Children) != len(want.Children) {
		return fmt.Sprintf("at %s got %d children, want %d", path, len(got.Children), len(want.Children))
	}
	for index := range got.Children {
		if difference := firstShapeDifference(got.Children[index], want.Children[index], fmt.Sprintf("%s.children[%d]", path, index)); difference != "" {
			return difference
		}
	}
	return "unknown difference"
}

func firstJSONDifference(got, want string) string {
	limit := len(got)
	if len(want) < limit {
		limit = len(want)
	}
	for index := 0; index < limit; index++ {
		if got[index] != want[index] {
			start := index - 40
			if start < 0 {
				start = 0
			}
			return fmt.Sprintf("byte %d got %q want %q", index, strings.TrimSpace(got[start:index+40]), strings.TrimSpace(want[start:index+40]))
		}
	}
	return fmt.Sprintf("length got %d want %d", len(got), len(want))
}
