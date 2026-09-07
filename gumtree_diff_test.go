package main

import (
	"strings"
	"testing"

	gumast "github.com/Xanonymous-GitHub/gumtree-go/ast"
)

func TestCalculatorExampleDiff(t *testing.T) {
	diff, err := prepareInMemoryDiff()
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.scripts) != 4 {
		t.Fatalf("scripts = %d, want 4", len(diff.scripts))
	}
	if string(diff.scripts[0].source.Value) != "-" || string(diff.scripts[0].target.Value) != "+" {
		t.Fatalf("script 1 = %s, want - -> +", diff.scripts[0])
	}
	for index := 1; index < len(diff.scripts); index++ {
		if diff.scripts[index].kind != editInsert {
			t.Fatalf("script %d kind = %s, want INSERT", index+1, diff.scripts[index].kind)
		}
	}
	diff.selected[0] = true
	applySelectedEdits(diff)
	if !strings.Contains(string(diff.workingSource), "return left + right") {
		t.Fatalf("applied source did not contain corrected addition")
	}
	for index := 1; index < len(diff.scripts); index++ {
		diff.selected[index] = true
		applySelectedEdits(diff)
	}
	for _, operator := range []string{"case '*':", "case '/':", "case '%':"} {
		if !strings.Contains(string(diff.workingSource), operator) {
			t.Fatalf("applied source did not contain %s", operator)
		}
	}
	if string(diff.workingSource) != string(diff.targetSource) {
		t.Fatalf("applied source does not match target:\ngot:\n%s\nwant:\n%s", diff.workingSource, diff.targetSource)
	}
}

func TestASTRowsKeepEditsIndependent(t *testing.T) {
	diff, err := prepareInMemoryDiff()
	if err != nil {
		t.Fatal(err)
	}
	rows := makeASTRows(diff.focusedRoot, diff.scriptsByNode, diff.scriptIndicesByNode, diff.scripts)

	editRows := 0
	for _, row := range rows {
		if len(row.editIndices) > 1 {
			t.Fatalf("AST row grouped edits %v", row.editIndices)
		}
		if len(row.editIndices) == 1 {
			editRows++
		}
	}
	if editRows != len(diff.scripts) {
		t.Fatalf("edit rows = %d, want %d", editRows, len(diff.scripts))
	}
}

func TestEditUndoRedo(t *testing.T) {
	source := gumast.Node{Label: "*ast.BasicLit", Value: "10"}
	target := gumast.Node{Label: "*ast.BasicLit", Value: "20"}
	diff := &inMemoryDiff{
		workingSource: []byte("x := 10"),
		scripts:       []editScript{{source: &source, target: &target, start: 5, end: 7, original: []byte("10"), replacement: []byte("20")}},
		selected:      []bool{true},
		applied:       []bool{false},
	}

	applySelectedEdits(diff)
	if got := string(diff.workingSource); got != "x := 20" {
		t.Fatalf("after apply = %q, want %q", got, "x := 20")
	}

	undoEdit(diff)
	if got := string(diff.workingSource); got != "x := 10" {
		t.Fatalf("after undo = %q, want %q", got, "x := 10")
	}

	redoEdit(diff)
	if got := string(diff.workingSource); got != "x := 20" {
		t.Fatalf("after redo = %q, want %q", got, "x := 20")
	}
}

func TestCalculatorInsertionsUndoRedoRepeatedly(t *testing.T) {
	diff, err := prepareInMemoryDiff()
	if err != nil {
		t.Fatal(err)
	}

	for index := range diff.scripts {
		diff.selected[index] = true
		applySelectedEdits(diff)
	}
	if got := string(diff.workingSource); got != string(diff.targetSource) {
		t.Fatalf("after applying all edits = %q, want target", got)
	}

	for range diff.scripts {
		undoEdit(diff)
	}
	if got := string(diff.workingSource); got != string(diff.sourceSource) {
		t.Fatalf("after undoing all edits = %q, want source", got)
	}

	for range diff.scripts {
		redoEdit(diff)
	}
	if got := string(diff.workingSource); got != string(diff.targetSource) {
		t.Fatalf("after redoing all edits = %q, want target", got)
	}

	for cycle := 0; cycle < 3; cycle++ {
		for range diff.scripts {
			undoEdit(diff)
		}
		if got := string(diff.workingSource); got != string(diff.sourceSource) {
			t.Fatalf("cycle %d undo = %q, want source", cycle, got)
		}
		for range diff.scripts {
			redoEdit(diff)
		}
		if got := string(diff.workingSource); got != string(diff.targetSource) {
			t.Fatalf("cycle %d redo = %q, want target", cycle, got)
		}
	}
}
