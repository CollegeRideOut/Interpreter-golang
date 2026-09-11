package main

import (
	"testing"
)

func TestSimpleASTDiffPreservesAssignmentFields(t *testing.T) {
	source := []byte("package main\n\nfunc main() {}\n")
	target := []byte("package main\n\nfunc main() {\n\tx := 10\n}\n")

	sourceAST, sourceFileSet, err := parseGoAST(source)
	if err != nil {
		t.Fatal(err)
	}
	targetAST, targetFileSet, err := parseGoAST(target)
	if err != nil {
		t.Fatal(err)
	}
	edits := simpleASTEditScripts(sourceAST, targetAST, sourceFileSet, targetFileSet)

	assign := -1
	ident := -1
	literal := -1
	for index, edit := range edits {
		switch edit.NodeKind {
		case "*ast.AssignStmt":
			assign = index
		case "*ast.Ident":
			if edit.Value == "x" {
				ident = index
			}
		case "*ast.BasicLit":
			if edit.Value == "10" {
				literal = index
			}
		}
	}
	if assign < 0 || ident < 0 || literal < 0 {
		t.Fatalf("missing assignment edits: assign=%d ident=%d literal=%d", assign, ident, literal)
	}
	if edits[assign].Field != "List" || edits[assign].Position != 0 {
		t.Fatalf("assignment location = %s[%d], want BlockStmt.List[0]", edits[assign].Field, edits[assign].Position)
	}
	if edits[ident].ParentID != edits[assign].NodeID || edits[ident].Field != "Lhs" || edits[ident].Position != 0 {
		t.Fatalf("identifier location = %s[%d] parent %s, want AssignStmt.Lhs[0]", edits[ident].Field, edits[ident].Position, edits[ident].ParentID)
	}
	if edits[literal].ParentID != edits[assign].NodeID || edits[literal].Field != "Rhs" || edits[literal].Position != 0 {
		t.Fatalf("literal location = %s[%d] parent %s, want AssignStmt.Rhs[0]", edits[literal].Field, edits[literal].Position, edits[literal].ParentID)
	}
}
