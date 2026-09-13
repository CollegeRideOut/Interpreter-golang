package main

import (
	goast "go/ast"
	"go/token"
	"testing"
)

func TestHardcodedSimpleAssignmentEdits(t *testing.T) {
	fileSet := token.NewFileSet()
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

	working := structuralASTTree(source, fileSet)
	targetTree := structuralASTTree(target, fileSet)

	// This is the hardcoded version of:
	// edits := simpleASTEditScripts(source, target, fileSet, fileSet)
	//
	// Pick one entry below to experiment with one edit at a time.
	edits := []structuralEdit{
		{
			Index:      0,
			Kind:       "INSERT",
			NodeID:     "root.Decls[0].Body.List[0]",
			NodeKind:   "*ast.AssignStmt",
			ParentID:   "root.Decls[0].Body",
			ParentKind: "*ast.BlockStmt",
			Field:      "List",
			Position:   0,
			Value:      ":=",
		},
		{
			Index:      1,
			Kind:       "INSERT",
			NodeID:     "root.Decls[0].Body.List[0].Lhs[0]",
			NodeKind:   "*ast.Ident",
			ParentID:   "root.Decls[0].Body.List[0]",
			ParentKind: "*ast.AssignStmt",
			Field:      "Lhs",
			Position:   0,
			Value:      "x",
		},
		{
			Index:      2,
			Kind:       "INSERT",
			NodeID:     "root.Decls[0].Body.List[0].Rhs[0]",
			NodeKind:   "*ast.BasicLit",
			ParentID:   "root.Decls[0].Body.List[0]",
			ParentKind: "*ast.AssignStmt",
			Field:      "Rhs",
			Position:   0,
			Value:      "1",
		},
	}

	generated := simpleASTEditScripts(source, target, fileSet, fileSet)
	if len(generated) != len(edits) {
		t.Fatalf("generated %d edits, want %d hardcoded edits", len(generated), len(edits))
	}
	for index := range edits {
		if generated[index].Kind != edits[index].Kind ||
			generated[index].NodeID != edits[index].NodeID ||
			generated[index].NodeKind != edits[index].NodeKind ||
			generated[index].ParentID != edits[index].ParentID ||
			generated[index].Field != edits[index].Field ||
			generated[index].Position != edits[index].Position ||
			generated[index].Value != edits[index].Value {
			t.Fatalf("generated edit %d = %+v, want hardcoded %+v", index, generated[index], edits[index])
		}
	}

	for _, edit := range edits {
		t.Run(edit.NodeKind, func(t *testing.T) {
			oneEdit := working.clone()
			if err := applyStructuralEdit(oneEdit, targetTree, edit); err != nil {
				t.Fatal(err)
			}
			inserted := oneEdit.find(edit.NodeID)
			if inserted == nil {
				t.Fatalf("hardcoded edit did not add %s", edit.NodeID)
			}
			if edit.NodeKind == "*ast.AssignStmt" && len(inserted.Children) != 0 {
				t.Fatalf("parent edit unexpectedly added %d children", len(inserted.Children))
			}
			if (edit.NodeKind == "*ast.Ident" || edit.NodeKind == "*ast.BasicLit") &&
				(inserted.parent == nil || inserted.parent.Kind != "*ast.BlockStmt" || inserted.Field != "List") {
				t.Fatalf("%s child edit was not kept directly in BlockStmt.List", edit.NodeKind)
			}
		})
	}
}

func TestHardcodedSimpleAssignmentEdits1(t *testing.T) {
	fileSet := token.NewFileSet()
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

	intermeditate := &goast.File{
		Name: goast.NewIdent("main"),
		Decls: []goast.Decl{
			&goast.FuncDecl{
				Name: goast.NewIdent("main"),
				Type: &goast.FuncType{Params: &goast.FieldList{}},
				Body: &goast.BlockStmt{
					List: []goast.Stmt{
						&goast.AssignStmt{
							Tok: token.DEFINE,
							Lhs: []goast.Expr{},
							Rhs: []goast.Expr{},
						},
					},
				},
			},
		},
	}

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

	working := structuralASTTree(source, fileSet)
	intermeditateAst := structuralASTTree(intermeditate, fileSet)
	targetTree := structuralASTTree(target, fileSet)

	// This is the hardcoded version of:
	// edits := simpleASTEditScripts(source, target, fileSet, fileSet)
	//
	// Pick one entry below to experiment with one edit at a time.
	edits := []structuralEdit{
		{
			Index:      0,
			Kind:       "INSERT",
			NodeID:     "root.Decls[0].Body.List[0]",
			NodeKind:   "*ast.AssignStmt",
			ParentID:   "root.Decls[0].Body",
			ParentKind: "*ast.BlockStmt",
			Field:      "List",
			Position:   0,
			Value:      ":=",
		},
		{
			Index:      1,
			Kind:       "INSERT",
			NodeID:     "root.Decls[0].Body.List[0].Lhs[0]",
			NodeKind:   "*ast.Ident",
			ParentID:   "root.Decls[0].Body.List[0]",
			ParentKind: "*ast.AssignStmt",
			Field:      "Lhs",
			Position:   0,
			Value:      "x",
		},
		{
			Index:      2,
			Kind:       "INSERT",
			NodeID:     "root.Decls[0].Body.List[0].Rhs[0]",
			NodeKind:   "*ast.BasicLit",
			ParentID:   "root.Decls[0].Body.List[0]",
			ParentKind: "*ast.AssignStmt",
			Field:      "Rhs",
			Position:   0,
			Value:      "1",
		},
	}

	generated := simpleASTEditScripts(source, target, fileSet, fileSet)
	if len(generated) != len(edits) {
		t.Fatalf("generated %d edits, want %d hardcoded edits", len(generated), len(edits))
	}
	for index := range edits {
		if generated[index].Kind != edits[index].Kind ||
			generated[index].NodeID != edits[index].NodeID ||
			generated[index].NodeKind != edits[index].NodeKind ||
			generated[index].ParentID != edits[index].ParentID ||
			generated[index].Field != edits[index].Field ||
			generated[index].Position != edits[index].Position ||
			generated[index].Value != edits[index].Value {
			t.Fatalf("generated edit %d = %+v, want hardcoded %+v", index, generated[index], edits[index])
		}
	}
	edit := edits[0]
	oneEdit := working.clone()
	if err := applyStructuralEdit(oneEdit, targetTree, edit); err != nil {
		t.Fatal(err)
	}
	if oneEdit.find(edit.NodeID) == nil {
		t.Fatalf("hardcoded edit did not add %s", edit.NodeID)
	}

	if !sameStructuralTree(oneEdit, intermeditateAst) {
		t.Fatalf("not the same tree ast")
	}
}
func sameStructuralTree(left, right *structuralASTNode) bool {
	if left == nil || right == nil {
		return left == right
	}

	if left.ID != right.ID ||
		left.Kind != right.Kind ||
		left.Value != right.Value ||
		left.Field != right.Field ||
		left.Index != right.Index {
		return false
	}

	if len(left.Children) != len(right.Children) {
		return false
	}

	for index := range left.Children {
		if !sameStructuralTree(left.Children[index], right.Children[index]) {
			return false
		}
	}

	return true
}

func TestHardcodedSimpleAssignmentIdentifierOnly(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	applyHardcodedEdits(t, working, target, edits, 1)

	expected := &structuralASTNode{
		ID: "root", Kind: "*ast.File", Value: "main",
		Children: []*structuralASTNode{
			{ID: "root.Name", Kind: "*ast.Ident", Value: "main", Field: "Name"},
			{ID: "root.Decls[0]", Kind: "*ast.FuncDecl", Field: "Decls", Children: []*structuralASTNode{
				{ID: "root.Decls[0].Name", Kind: "*ast.Ident", Value: "main", Field: "Name"},
				{ID: "root.Decls[0].Type", Kind: "*ast.FuncType", Field: "Type", Children: []*structuralASTNode{
					{ID: "root.Decls[0].Type.Params", Kind: "*ast.FieldList", Field: "Params"},
				}},
				{ID: "root.Decls[0].Body", Kind: "*ast.BlockStmt", Field: "Body", Children: []*structuralASTNode{
					{ID: edits[1].NodeID, Kind: "*ast.Ident", Value: "x", Field: "List", Index: 0},
				}},
			}},
		},
	}
	if !sameStructuralTree(working, expected) {
		t.Fatal("identifier-only intermediate AST is wrong")
	}
}

func TestHardcodedSimpleAssignmentLiteralOnly(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	applyHardcodedEdits(t, working, target, edits, 2)

	expected := &structuralASTNode{
		ID: "root", Kind: "*ast.File", Value: "main",
		Children: []*structuralASTNode{
			{ID: "root.Name", Kind: "*ast.Ident", Value: "main", Field: "Name"},
			{ID: "root.Decls[0]", Kind: "*ast.FuncDecl", Field: "Decls", Children: []*structuralASTNode{
				{ID: "root.Decls[0].Name", Kind: "*ast.Ident", Value: "main", Field: "Name"},
				{ID: "root.Decls[0].Type", Kind: "*ast.FuncType", Field: "Type", Children: []*structuralASTNode{
					{ID: "root.Decls[0].Type.Params", Kind: "*ast.FieldList", Field: "Params"},
				}},
				{ID: "root.Decls[0].Body", Kind: "*ast.BlockStmt", Field: "Body", Children: []*structuralASTNode{
					{ID: edits[2].NodeID, Kind: "*ast.BasicLit", Value: "1", Field: "List", Index: 0},
				}},
			}},
		},
	}
	if !sameStructuralTree(working, expected) {
		t.Fatal("literal-only intermediate AST is wrong")
	}
}

func TestHardcodedSimpleAssignmentIdentifierThenLiteral(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	applyHardcodedEdits(t, working, target, edits, 1, 2)

	expected := &structuralASTNode{
		ID: "root", Kind: "*ast.File", Value: "main",
		Children: []*structuralASTNode{
			{ID: "root.Name", Kind: "*ast.Ident", Value: "main", Field: "Name"},
			{ID: "root.Decls[0]", Kind: "*ast.FuncDecl", Field: "Decls", Children: []*structuralASTNode{
				{ID: "root.Decls[0].Name", Kind: "*ast.Ident", Value: "main", Field: "Name"},
				{ID: "root.Decls[0].Type", Kind: "*ast.FuncType", Field: "Type", Children: []*structuralASTNode{
					{ID: "root.Decls[0].Type.Params", Kind: "*ast.FieldList", Field: "Params"},
				}},
				{ID: "root.Decls[0].Body", Kind: "*ast.BlockStmt", Field: "Body", Children: []*structuralASTNode{
					{ID: edits[1].NodeID, Kind: "*ast.Ident", Value: "x", Field: "List", Index: 0},
					{ID: edits[2].NodeID, Kind: "*ast.BasicLit", Value: "1", Field: "List", Index: 1},
				}},
			}},
		},
	}
	if !sameStructuralTree(working, expected) {
		t.Fatal("identifier-then-literal intermediate AST is wrong")
	}
}

func TestHardcodedSimpleAssignmentIdentifierThenParent(t *testing.T) {
	working, target, edits := hardcodedAssignmentFixture(t)
	applyHardcodedEdits(t, working, target, edits, 1, 0)

	expected := &structuralASTNode{
		ID: "root", Kind: "*ast.File", Value: "main",
		Children: []*structuralASTNode{
			{ID: "root.Name", Kind: "*ast.Ident", Value: "main", Field: "Name"},
			{ID: "root.Decls[0]", Kind: "*ast.FuncDecl", Field: "Decls", Children: []*structuralASTNode{
				{ID: "root.Decls[0].Name", Kind: "*ast.Ident", Value: "main", Field: "Name"},
				{ID: "root.Decls[0].Type", Kind: "*ast.FuncType", Field: "Type", Children: []*structuralASTNode{
					{ID: "root.Decls[0].Type.Params", Kind: "*ast.FieldList", Field: "Params"},
				}},
				{ID: "root.Decls[0].Body", Kind: "*ast.BlockStmt", Field: "Body", Children: []*structuralASTNode{
					{ID: edits[0].NodeID, Kind: "*ast.AssignStmt", Value: ":=", Field: "List", Index: 0, Children: []*structuralASTNode{
						{ID: edits[1].NodeID, Kind: "*ast.Ident", Value: "x", Field: "Lhs", Index: 0},
					}},
				}},
			}},
		},
	}
	if !sameStructuralTree(working, expected) {
		t.Fatal("identifier-then-parent intermediate AST is wrong")
	}
}

func applyHardcodedEdits(t *testing.T, working, target *structuralASTNode, edits []structuralEdit, indices ...int) {
	t.Helper()
	for _, index := range indices {
		if err := applyStructuralEdit(working, target, edits[index]); err != nil {
			t.Fatalf("apply edit %d: %v", index, err)
		}
	}
}

func hardcodedAssignmentFixture(t *testing.T) (*structuralASTNode, *structuralASTNode, []structuralEdit) {
	t.Helper()
	fileSet := token.NewFileSet()
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
	target := &goast.File{
		Name: goast.NewIdent("main"),
		Decls: []goast.Decl{
			&goast.FuncDecl{
				Name: goast.NewIdent("main"),
				Type: &goast.FuncType{Params: &goast.FieldList{}},
				Body: &goast.BlockStmt{List: []goast.Stmt{
					&goast.AssignStmt{
						Tok: token.DEFINE,
						Lhs: []goast.Expr{goast.NewIdent("x")},
						Rhs: []goast.Expr{&goast.BasicLit{Kind: token.INT, Value: "1"}},
					},
				}},
			},
		},
	}

	working := structuralASTTree(source, fileSet)
	targetTree := structuralASTTree(target, fileSet)
	edits := []structuralEdit{
		{Index: 0, Kind: "INSERT", NodeID: "root.Decls[0].Body.List[0]", NodeKind: "*ast.AssignStmt", ParentID: "root.Decls[0].Body", ParentKind: "*ast.BlockStmt", Field: "List", Position: 0, Value: ":="},
		{Index: 1, Kind: "INSERT", NodeID: "root.Decls[0].Body.List[0].Lhs[0]", NodeKind: "*ast.Ident", ParentID: "root.Decls[0].Body.List[0]", ParentKind: "*ast.AssignStmt", Field: "Lhs", Position: 0, Value: "x"},
		{Index: 2, Kind: "INSERT", NodeID: "root.Decls[0].Body.List[0].Rhs[0]", NodeKind: "*ast.BasicLit", ParentID: "root.Decls[0].Body.List[0]", ParentKind: "*ast.AssignStmt", Field: "Rhs", Position: 0, Value: "1"},
	}
	return working, targetTree, edits
}
