package main

import "testing"

func TestStructuralASTApplyGoldenStates(t *testing.T) {
	tests := []struct {
		name  string
		build func() (*structuralASTNode, *structuralASTNode, []structuralEdit)
		order []int
		wants []string
	}{
		{
			name:  "update-only",
			build: goldenUpdateFixture,
			order: []int{0},
			wants: []string{`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.BasicLit","value":"2","field":"List","index":0}]}]}`},
		},
		{
			name:  "insert-parent-then-children",
			build: goldenAssignmentFixture,
			order: []int{0, 1, 2},
			wants: []string{
				`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.AssignStmt","value":":=","field":"List","index":0}]}]}`,
				`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.AssignStmt","value":":=","field":"List","index":0,"children":[{"kind":"*ast.Ident","value":"x","field":"Lhs","index":0}]}]}]}`,
				`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.AssignStmt","value":":=","field":"List","index":0,"children":[{"kind":"*ast.Ident","value":"x","field":"Lhs","index":0},{"kind":"*ast.BasicLit","value":"1","field":"Rhs","index":0}]}]}]}`,
			},
		},
		{
			name:  "children-before-parent",
			build: goldenAssignmentFixture,
			order: []int{1, 2, 0},
			wants: []string{
				`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.Ident","value":"x","field":"List","index":0}]}]}`,
				`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.Ident","value":"x","field":"List","index":0},{"kind":"*ast.BasicLit","value":"1","field":"List","index":1}]}]}`,
				`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.AssignStmt","value":":=","field":"List","index":0,"children":[{"kind":"*ast.Ident","value":"x","field":"Lhs","index":0},{"kind":"*ast.BasicLit","value":"1","field":"Rhs","index":0}]}]}]}`,
			},
		},
		{
			name:  "delete-parent-then-child",
			build: goldenDeleteFixture,
			order: []int{0, 1},
			wants: []string{
				`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0}]}`,
				`{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0}]}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			working, target, edits := test.build()
			for step, editIndex := range test.order {
				if err := applyStructuralEdit(working, target, edits[editIndex]); err != nil {
					t.Fatalf("edit %d: %v", editIndex, err)
				}
				if got := structuralNodeShape(working); got != test.wants[step] {
					t.Fatalf("after edit %d got:\n%s\nwant:\n%s", editIndex, got, test.wants[step])
				}
			}
		})
	}
}

func TestStructuralASTSiblingOrderGoldenStates(t *testing.T) {
	orders := [][]int{{0, 1, 2}, {2, 0, 1}, {1, 2, 0}, {2, 1, 0}}
	want := `{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.Ident","value":"a","field":"List","index":0},{"kind":"*ast.Ident","value":"b","field":"List","index":1},{"kind":"*ast.Ident","value":"c","field":"List","index":2}]}]}`
	for _, order := range orders {
		working, target, edits := goldenSiblingsFixture()
		for _, editIndex := range order {
			if err := applyStructuralEdit(working, target, edits[editIndex]); err != nil {
				t.Fatalf("order %v edit %d: %v", order, editIndex, err)
			}
		}
		if got := structuralNodeShape(working); got != want {
			t.Errorf("order %v got:\n%s\nwant:\n%s", order, got, want)
		}
	}
}

func TestStructuralASTCandidateGoldenState(t *testing.T) {
	working, target, edits := goldenAssignmentFixture()
	if err := applyStructuralEditWithOptions(working, target, edits[1], ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}
	if err := applyStructuralEditWithOptions(working, target, edits[0], ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}
	wantOrphan := `{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.AssignStmt","value":":=","field":"List","index":0},{"kind":"*ast.Ident","value":"x","field":"List","index":1}]}]}`
	if got := structuralNodeShape(working); got != wantOrphan {
		t.Fatalf("orphan state got:\n%s\nwant:\n%s", got, wantOrphan)
	}
	candidates := ReconciliationCandidates(working, target, edits[0].NodeID)
	if len(candidates) != 1 || !candidates[0].CanReconcile || candidates[0].NodeID != edits[1].NodeID {
		t.Fatalf("candidates = %+v", candidates)
	}
	if err := ApplyReconciliationCandidate(working, target, candidates[0]); err != nil {
		t.Fatal(err)
	}
	wantReconciled := `{"kind":"*ast.File","value":"main","index":0,"children":[{"kind":"*ast.BlockStmt","field":"Body","index":0,"children":[{"kind":"*ast.AssignStmt","value":":=","field":"List","index":0,"children":[{"kind":"*ast.Ident","value":"x","field":"Lhs","index":0}]}]}]}`
	if got := structuralNodeShape(working); got != wantReconciled {
		t.Fatalf("reconciled state got:\n%s\nwant:\n%s", got, wantReconciled)
	}
	if got := ReconciliationCandidates(working, target, edits[0].NodeID); len(got) != 0 {
		t.Fatalf("reconciliation left candidates: %+v", got)
	}
}

func goldenUpdateFixture() (*structuralASTNode, *structuralASTNode, []structuralEdit) {
	current := goldenFile(goldenNodeInternal("value", "*ast.BasicLit", "1", "List", 0))
	target := goldenFile(goldenNodeInternal("value", "*ast.BasicLit", "2", "List", 0))
	return current, target, []structuralEdit{{Kind: "UPDATE", NodeID: "value", NodeKind: "*ast.BasicLit", Value: "2"}}
}

func goldenAssignmentFixture() (*structuralASTNode, *structuralASTNode, []structuralEdit) {
	current := goldenFile()
	assignment := goldenNodeInternal("assignment", "*ast.AssignStmt", ":=", "List", 0)
	identifier := goldenNodeInternal("identifier", "*ast.Ident", "x", "Lhs", 0)
	literal := goldenNodeInternal("literal", "*ast.BasicLit", "1", "Rhs", 0)
	identifier.parent = assignment
	literal.parent = assignment
	assignment.Children = []*structuralASTNode{identifier, literal}
	target := goldenFile(assignment)
	edits := []structuralEdit{
		{Kind: "INSERT", NodeID: assignment.ID, NodeKind: assignment.Kind, ParentID: "root.Body", Field: "List", Position: 0},
		{Kind: "INSERT", NodeID: identifier.ID, NodeKind: identifier.Kind, ParentID: assignment.ID, Field: "Lhs", Position: 0},
		{Kind: "INSERT", NodeID: literal.ID, NodeKind: literal.Kind, ParentID: assignment.ID, Field: "Rhs", Position: 0},
	}
	return current, target, edits
}

func goldenDeleteFixture() (*structuralASTNode, *structuralASTNode, []structuralEdit) {
	_, target, _ := goldenAssignmentFixture()
	parent := target.find("assignment")
	child := target.find("identifier")
	return currentWithAssignment(parent, child), targetWithoutAssignment(target), []structuralEdit{
		{Kind: "DELETE", NodeID: parent.ID, NodeKind: parent.Kind, SourceGlobalID: parent.GlobalID},
		{Kind: "DELETE", NodeID: child.ID, NodeKind: child.Kind, SourceGlobalID: child.GlobalID},
	}
}

func goldenSiblingsFixture() (*structuralASTNode, *structuralASTNode, []structuralEdit) {
	current := goldenFile()
	a := goldenNodeInternal("a", "*ast.Ident", "a", "List", 0)
	b := goldenNodeInternal("b", "*ast.Ident", "b", "List", 1)
	c := goldenNodeInternal("c", "*ast.Ident", "c", "List", 2)
	target := goldenFile(a, b, c)
	edits := []structuralEdit{
		{Kind: "INSERT", NodeID: a.ID, NodeKind: a.Kind, ParentID: "root.Body", Field: "List", Position: 0},
		{Kind: "INSERT", NodeID: b.ID, NodeKind: b.Kind, ParentID: "root.Body", Field: "List", Position: 1},
		{Kind: "INSERT", NodeID: c.ID, NodeKind: c.Kind, ParentID: "root.Body", Field: "List", Position: 2},
	}
	return current, target, edits
}

func goldenFile(children ...*structuralASTNode) *structuralASTNode {
	block := &structuralASTNode{ID: "root.Body", GlobalID: "global:root.Body", Kind: "*ast.BlockStmt", Field: "Body", Index: 0}
	root := &structuralASTNode{ID: "root", GlobalID: "global:root", Kind: "*ast.File", Value: "main", Children: []*structuralASTNode{block}}
	block.parent = root
	for _, child := range children {
		insertStructuralNode(block, child, child.Index)
	}
	return root
}

func goldenNodeInternal(id, kind, value, field string, index int) *structuralASTNode {
	return &structuralASTNode{ID: id, GlobalID: "global:" + id, Kind: kind, Value: value, Field: field, Index: index}
}

func currentWithAssignment(parent, child *structuralASTNode) *structuralASTNode {
	root := goldenFile()
	assignment := parent.clone()
	assignment.Children = []*structuralASTNode{child.clone()}
	insertStructuralNode(root.Children[0], assignment, 0)
	return root
}

func targetWithoutAssignment(target *structuralASTNode) *structuralASTNode {
	return goldenFile()
}

func structuralNodeShape(root *structuralASTNode) string {
	return mustJSONValue(structuralNodeShapeValue(root))
}

func structuralNodeShapeValue(node *structuralASTNode) structuralShape {
	if node == nil {
		return structuralShape{}
	}
	result := structuralShape{Kind: node.Kind, Value: node.Value, Field: node.Field, Index: node.Index}
	for _, child := range node.Children {
		result.Children = append(result.Children, structuralNodeShapeValue(child))
	}
	return result
}
