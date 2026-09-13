package main

import (
	goast "go/ast"
	"go/token"
	"testing"
)

func TestSimpleAssignmentEdit(t *testing.T) {
	// This is the source program: func main() {}
	source := &goast.File{
		Name: goast.NewIdent("main"),
		Decls: []goast.Decl{
			&goast.FuncDecl{
				Name: goast.NewIdent("main"),
				Type: &goast.FuncType{Params: &goast.FieldList{}},
				Body: &goast.BlockStmt{},
			},
		},
	}

	// This is the target program: func main() { x := 1 }
	target := &goast.File{
		Name: goast.NewIdent("main"),
		Decls: []goast.Decl{
			&goast.FuncDecl{
				Name: goast.NewIdent("main"),
				Type: &goast.FuncType{Params: &goast.FieldList{}},
				Body: &goast.BlockStmt{
					List: []goast.Stmt{
						&goast.AssignStmt{
							Tok: token.DEFINE,
							Lhs: []goast.Expr{goast.NewIdent("x")},
							Rhs: []goast.Expr{&goast.BasicLit{
								Kind:  token.INT,
								Value: "1",
							}},
						},
					},
				},
			},
		},
	}

	fileSet := token.NewFileSet()
	sourceTree := structuralASTTree(source, fileSet)
	targetTree := structuralASTTree(target, fileSet)
	edits := simpleASTEditScripts(source, target, fileSet, fileSet)
	bindStructuralEditIdentities(sourceTree, targetTree, edits)

	if len(edits) != 3 {
		t.Fatalf("generated %d edits, want assignment, x, and 1", len(edits))
	}

	// Applying nothing leaves the source AST unchanged.
	if sourceTree.find("root.Decls[0].Body.List[0]") != nil {
		t.Fatal("source AST already contains the assignment")
	}

	for _, edit := range edits {
		t.Run(edit.Kind+" "+edit.NodeKind, func(t *testing.T) {
			working := sourceTree.clone()
			if err := applyStructuralEdit(working, targetTree, edit); err != nil {
				t.Fatal(err)
			}

			if working.find(edit.NodeID) == nil {
				t.Fatalf("applying %s did not add %s", edit.Kind, edit.NodeKind)
			}
		})
	}
}
