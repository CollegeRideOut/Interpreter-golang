package main

import "testing"

func TestApplyStructuralEditsParentThenChild(t *testing.T) {
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
	working := structuralASTTree(sourceAST, sourceFileSet)
	targetTree := structuralASTTree(targetAST, targetFileSet)
	edits := simpleASTEditScripts(sourceAST, targetAST, sourceFileSet, targetFileSet)
	bindStructuralEditIdentities(working, targetTree, edits)

	assign, ident := -1, -1
	for index, edit := range edits {
		if edit.NodeKind == "*ast.AssignStmt" {
			assign = index
		}
		if edit.NodeKind == "*ast.Ident" && edit.Value == "x" {
			ident = index
		}
	}
	if assign < 0 || ident < 0 {
		t.Fatalf("missing assignment edits: assign=%d ident=%d", assign, ident)
	}
	if err := applyStructuralEdit(working, targetTree, edits[assign]); err != nil {
		t.Fatal(err)
	}
	assignNode := working.find(edits[assign].NodeID)
	if assignNode == nil || len(assignNode.Children) != 0 {
		t.Fatal("parent edit materialized assignment children")
	}
	if err := applyStructuralEdit(working, targetTree, edits[ident]); err != nil {
		t.Fatal(err)
	}
	identNode := working.find(edits[ident].NodeID)
	if identNode == nil || identNode.parent != assignNode || identNode.Field != "Lhs" {
		t.Fatal("child edit did not attach to AssignStmt.Lhs")
	}
}

func TestApplyStructuralEditChildBeforeParentKeepsOrphan(t *testing.T) {
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
	working := structuralASTTree(sourceAST, sourceFileSet)
	targetTree := structuralASTTree(targetAST, targetFileSet)
	edits := simpleASTEditScripts(sourceAST, targetAST, sourceFileSet, targetFileSet)
	bindStructuralEditIdentities(working, targetTree, edits)

	assign, ident := -1, -1
	for index, edit := range edits {
		if edit.NodeKind == "*ast.AssignStmt" {
			assign = index
		}
		if edit.NodeKind == "*ast.Ident" && edit.Value == "x" {
			ident = index
		}
	}
	if assign < 0 || ident < 0 {
		t.Fatalf("missing assignment or identifier edit: assignment=%d identifier=%d", assign, ident)
	}

	if err := applyStructuralEdit(working, targetTree, edits[ident]); err != nil {
		t.Fatal(err)
	}

	block := working.find("root.Decls[0].Body")
	identifier := working.find(edits[ident].NodeID)
	if block == nil || identifier == nil || identifier.parent != block || identifier.Field != "List" {
		t.Fatalf("child edit was not kept as an orphan in BlockStmt.List")
	}

	if err := applyStructuralEdit(working, targetTree, edits[assign]); err != nil {
		t.Fatal(err)
	}
	assignment := working.find(edits[assign].NodeID)
	if assignment == nil || identifier.parent != assignment || identifier.Field != "Lhs" {
		t.Fatal("parent edit did not reattach the orphan identifier")
	}
}
