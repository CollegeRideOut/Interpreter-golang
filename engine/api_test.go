package engine_test

import (
	"strings"
	"testing"

	"interpreter/engine"
)

func TestPublicEngineAPI(t *testing.T) {
	source := []byte("package main\n\nfunc main() {}\n")
	target := []byte("package main\n\nfunc main() {\n\tx := 10\n}\n")

	root, err := engine.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if root == nil || root.Kind != "*ast.File" {
		t.Fatalf("unexpected parsed root: %#v", root)
	}

	sourceTree, targetTree, edits, err := engine.Diff(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if sourceTree == nil || targetTree == nil || len(edits) == 0 {
		t.Fatalf("expected trees and edits, got %#v %#v %#v", sourceTree, targetTree, edits)
	}

	state, err := engine.NewWorkingStateFromSource(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Apply(0); err != nil {
		t.Fatal(err)
	}
	if state.Snapshot().Root == nil {
		t.Fatal("snapshot has no root")
	}
	if state.Snapshot().RenderedCode == "" {
		t.Fatal("snapshot has no rendered working code")
	}
}

func TestApplyProjectedKeepsReplacementSlotsValid(t *testing.T) {
	state, err := engine.NewWorkingStateFromSource(
		[]byte("package main\n\nfunc main() {\n\tx := 5\n\t_ = x\n}\n"),
		[]byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tmonthlyIncome := 81000\n\tfmt.Println(monthlyIncome)\n}\n"),
	)
	if err != nil {
		t.Fatal(err)
	}

	deleteIndex := -1
	for index, edit := range state.Snapshot().Edits {
		if edit.Kind == "DELETE" && edit.NodeKind == "*ast.Ident" && edit.Value == "x" {
			deleteIndex = index
			break
		}
	}
	if deleteIndex < 0 {
		t.Fatal("expected the old x identifier replacement edit")
	}
	if err := state.ApplyProjected(deleteIndex, engine.ApplyOptions{Reconcile: true}, engine.LiftOptions{}); err != nil {
		t.Fatal(err)
	}
	if report := state.ValidateGo(); !report.Valid {
		t.Fatalf("replacement left invalid Go: %v", report.Diagnostics)
	}
	if err := state.RemoveProjected(deleteIndex, engine.LiftOptions{}); err != nil {
		t.Fatal(err)
	}
	if report := state.ValidateGo(); !report.Valid {
		t.Fatalf("removing replacement left invalid Go: valid=%v code=%q", report.Valid, state.Snapshot().RenderedCode)
	}
}

func TestApplyProjectedReachesMonthlyBudgetTarget(t *testing.T) {
	source := []byte("package main\n\nfunc main() {\n\n\tx := 5\n\t_ = x\n\n}\n")
	target := []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tmonthlyIncome := 81000\n\tphone := 2000\n\tgas := 5000\n\tgym := 2000\n\tentertainment := 2000\n\n\ttotalExpenses := phone + gas + gym + entertainment\n\tremaining := monthlyIncome - totalExpenses\n\n\tfmt.Println(\"remaining:\", remaining)\n}\n")
	state, err := engine.NewWorkingStateFromSource(source, target)
	if err != nil {
		t.Fatal(err)
	}
	for index := range state.Snapshot().Edits {
		if err := state.ApplyProjected(index, engine.ApplyOptions{Reconcile: true}, engine.LiftOptions{}); err != nil {
			t.Fatalf("apply edit %d: %v", index, err)
		}
	}
	if report := state.ValidateGo(); !report.Valid {
		t.Fatalf("target application left invalid Go: %v\n%s", report.Diagnostics, state.Snapshot().RenderedCode)
	}
	if got := state.Snapshot().RenderedCode; strings.Join(strings.Fields(got), " ") != strings.Join(strings.Fields(string(target)), " ") {
		t.Fatalf("applied source differs from target:\n--- got ---\n%s\n--- want ---\n%s", got, target)
	}
}

func TestRenderBestEffortPreservesIntermediateNodes(t *testing.T) {
	root, err := engine.Parse([]byte("package main\n\nfunc main() {}\n"))
	if err != nil {
		t.Fatal(err)
	}

	result := engine.RenderBestEffort(root)
	if result.Code != "package main\n\nfunc main() {}\n" {
		t.Fatalf("rendered source = %q", result.Code)
	}
}

func TestRenderBestEffortRendersFunctionLiteral(t *testing.T) {
	source := []byte("package main\n\nfunc main() {\n\thttp.HandleFunc(\"/\", func(writer http.ResponseWriter, request *http.Request) {})\n}\n")
	root, err := engine.Parse(source)
	if err != nil {
		t.Fatal(err)
	}

	result := engine.RenderBestEffort(root)
	want := "package main\n\nfunc main() {\n\thttp.HandleFunc(\"/\", func(writer http.ResponseWriter, request *http.Request) {})\n}\n"
	if result.Code != want {
		t.Fatalf("rendered function literal = %q, want %q", result.Code, want)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic == "unsupported or incomplete *ast.FuncLit" {
			t.Fatalf("complete function literal produced an unsupported diagnostic")
		}
	}
}

func TestRenderBestEffortRendersTypesAndCompositeLiterals(t *testing.T) {
	source := []byte("package main\n\ntype Config struct { Name string }\n\nfunc main() { _ = Config{Name: \"demo\"} }\n")
	root, err := engine.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	result := engine.RenderBestEffort(root)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic == "unsupported or incomplete *ast.TypeSpec" || diagnostic == "unsupported or incomplete *ast.CompositeLit" {
			t.Fatalf("complete AST node produced an unsupported diagnostic: %s\n%s", diagnostic, result.Code)
		}
	}
}

func TestRenderBestEffortReportsIncompleteFunctionLiteral(t *testing.T) {
	result := engine.RenderBestEffort(&engine.Node{Kind: "*ast.FuncLit"})
	if result.Code != "STRUCTURALERROR.Function(/* params */) { /* body */ }\n" {
		t.Fatalf("incomplete function literal = %q", result.Code)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("incomplete function literal diagnostics = %v, want none for skeleton rendering", result.Diagnostics)
	}
}

func TestRenderBestEffortShowsPartialNodeSkeletons(t *testing.T) {
	tests := []struct {
		name string
		node *engine.Node
		want string
	}{
		{
			name: "call without children",
			node: &engine.Node{Kind: "*ast.CallExpr"},
			want: "STRUCTURALERROR.FunctionCall()\n",
		},
		{
			name: "selector without children",
			node: &engine.Node{Kind: "*ast.SelectorExpr"},
			want: "STRUCTURALERROR.Selector\n",
		},
		{
			name: "function without children",
			node: &engine.Node{Kind: "*ast.FuncLit"},
			want: "STRUCTURALERROR.Function(/* params */) { /* body */ }\n",
		},
		{
			name: "if without children",
			node: &engine.Node{Kind: "*ast.IfStmt"},
			want: "if STRUCTURALERROR.Condition { /* body */ }\n",
		},
		{
			name: "binary expression without operands",
			node: &engine.Node{Kind: "*ast.BinaryExpr", Value: "!="},
			want: "STRUCTURALERROR.BinaryLeft != STRUCTURALERROR.BinaryRight\n",
		},
		{
			name: "if with prepared binary condition",
			node: &engine.Node{Kind: "*ast.IfStmt", Children: []*engine.Node{
				{Kind: "*ast.BinaryExpr", Value: "!=", Field: "Cond"},
			}},
			want: "if STRUCTURALERROR.Condition { /* body */ }\n",
		},
		{
			name: "empty expression statement",
			node: &engine.Node{Kind: "*ast.ExprStmt"},
			want: "STRUCTURALERROR.ExpressionStatement\n",
		},
		{
			name: "call preserves orphan selector member",
			node: &engine.Node{Kind: "*ast.CallExpr", Children: []*engine.Node{
				{Kind: "*ast.Ident", Value: "HandleFunc", Field: "Fun", OriginalField: "Sel"},
			}},
			want: "STRUCTURALERROR.Selector.HandleFunc()\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := engine.RenderBestEffort(test.node).Code; got != test.want {
				t.Fatalf("rendered skeleton = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRenderBestEffortShowsOrphanNodeInSource(t *testing.T) {
	block := &engine.Node{Kind: "*ast.BlockStmt", Field: "Body", Children: []*engine.Node{
		{Kind: "*ast.BasicLit", Value: `"/"`, Field: "child", Index: 0},
	}}
	result := engine.RenderBestEffort(block)
	if result.Code != "{\n\tSTRUCTURALERROR.Lit(\"/\")\n}\n" {
		t.Fatalf("orphan node render = %q", result.Code)
	}
}

func TestApplyFunctionCreatesEmptyTypeAndBodySlots(t *testing.T) {
	state, err := engine.NewWorkingStateFromSource(
		[]byte("package main\n"),
		[]byte("package main\n\nfunc main() {}\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	for index, edit := range state.Snapshot().Edits {
		if edit.NodeKind == "*ast.FuncDecl" {
			if err := state.ApplyWithOptions(index, engine.ApplyOptions{Reconcile: false}); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	snapshot := state.Snapshot()
	prepared := 0
	for index, edit := range snapshot.Edits {
		if edit.ParentID != "" && (edit.NodeKind == "*ast.FuncType" || edit.NodeKind == "*ast.BlockStmt") && snapshot.Status[index] == engine.EditPrepared {
			prepared++
		}
	}
	if prepared != 2 {
		t.Fatalf("prepared function slots = %d, want 2", prepared)
	}
	var function *engine.Node
	var walk func(*engine.Node)
	walk = func(node *engine.Node) {
		if node == nil {
			return
		}
		if node.Kind == "*ast.FuncDecl" {
			function = node
			return
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(snapshot.Root)
	if function == nil {
		t.Fatal("function declaration was not applied")
	}
	for _, field := range []string{"Type", "Body"} {
		found := false
		for _, child := range function.Children {
			if child.Field == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("function is missing empty %s slot", field)
		}
	}
}

func TestApplyingFunctionRestoresRemovedBodySlot(t *testing.T) {
	state, err := engine.NewWorkingStateFromSource(
		[]byte("package main\n"),
		[]byte("package main\n\nfunc main() {}\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	functionIndex := -1
	bodyIndex := -1
	for index, edit := range state.Snapshot().Edits {
		if edit.NodeKind == "*ast.FuncDecl" {
			functionIndex = index
		}
		if edit.NodeKind == "*ast.BlockStmt" {
			bodyIndex = index
		}
	}
	if functionIndex < 0 || bodyIndex < 0 {
		t.Fatal("expected function and body edits")
	}
	if err := state.Apply(functionIndex); err != nil {
		t.Fatal(err)
	}
	if err := state.Remove(bodyIndex); err != nil {
		t.Fatal(err)
	}
	if err := state.Apply(functionIndex); err != nil {
		t.Fatal(err)
	}
	if status := state.Snapshot().Status[bodyIndex]; status != engine.EditPrepared {
		t.Fatalf("restored body status = %q, want prepared", status)
	}
}

func TestRenderBestEffortPreservesOrphanCallSubtree(t *testing.T) {
	call := &engine.Node{Kind: "*ast.CallExpr", Field: "child", Children: []*engine.Node{
		{Kind: "*ast.SelectorExpr", Field: "Fun", Children: []*engine.Node{
			{Kind: "*ast.Ident", Value: "http", Field: "X"},
			{Kind: "*ast.Ident", Value: "ServerFile", Field: "Sel"},
		}},
		{Kind: "*ast.BasicLit", Value: `"index.html"`, Field: "Args", Index: 0},
	}}
	block := &engine.Node{Kind: "*ast.BlockStmt", Children: []*engine.Node{call}}
	result := engine.RenderBestEffort(block)
	want := "{\n\tSTRUCTURALERROR.FunctionCall(http.ServerFile(\"index.html\"))\n}\n"
	if result.Code != want {
		t.Fatalf("orphan call render = %q, want %q", result.Code, want)
	}
}

func TestApplySubtreeAppliesDependentEdits(t *testing.T) {
	source := []byte("package main\n\nfunc main() {}\n")
	target := []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
	state, err := engine.NewWorkingStateFromSource(source, target)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := state.Snapshot()
	parent := -1
	for index, edit := range snapshot.Edits {
		for _, child := range snapshot.Edits {
			if child.ParentID == edit.NodeID {
				parent = index
				break
			}
		}
		if parent >= 0 {
			break
		}
	}
	if parent < 0 {
		t.Fatal("expected a parent edit")
	}
	if err := state.ApplySubtree(parent); err != nil {
		t.Fatal(err)
	}

	applied := 0
	for _, status := range state.Snapshot().Status {
		if status == engine.EditApplied {
			applied++
		}
	}
	if applied < 2 {
		detail := state.Snapshot()
		t.Fatalf("expected dependent edits to apply, got %d of %d", applied, len(detail.Status))
	}
}

func TestApplyWithReconciliationDisabledPreservesOrphanGroup(t *testing.T) {
	current := &engine.Node{
		ID: "root", Kind: "*ast.File", Value: "main",
		Children: []*engine.Node{{ID: "root.Body", Kind: "*ast.BlockStmt", Field: "Body"}},
	}
	assignment := &engine.Node{ID: "assignment", GlobalID: "target:assignment", Kind: "*ast.AssignStmt", Value: ":=", Field: "List", Index: 0}
	identifier := &engine.Node{ID: "identifier", GlobalID: "target:identifier", Kind: "*ast.Ident", Value: "x", Field: "Lhs", Index: 0}
	assignment.Children = []*engine.Node{identifier}
	target := &engine.Node{
		ID: "root", Kind: "*ast.File", Value: "main",
		Children: []*engine.Node{{ID: "root.Body", Kind: "*ast.BlockStmt", Field: "Body", Children: []*engine.Node{assignment}}},
	}
	edits := []engine.Edit{
		{Kind: "INSERT", NodeID: assignment.ID, NodeKind: assignment.Kind, ParentID: "root.Body", Field: "List", Position: 0, Node: assignment},
		{Kind: "INSERT", NodeID: identifier.ID, NodeKind: identifier.Kind, ParentID: assignment.ID, Field: "Lhs", Position: 0, Node: identifier},
	}
	state, err := engine.NewWorkingState(current, target, edits)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ApplyWithOptions(1, engine.ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}
	if err := state.ApplyWithOptions(0, engine.ApplyOptions{Reconcile: false}); err != nil {
		t.Fatal(err)
	}

	root := state.Snapshot().Root
	block := root.Children[0]
	if len(block.Children) != 2 {
		t.Fatalf("reconciliation disabled tree has %d block children, want assignment plus orphan", len(block.Children))
	}
	if len(block.Children[0].Children) != 0 || block.Children[1].Kind != "*ast.Ident" || block.Children[1].Field != "List" {
		t.Fatalf("reconciliation disabled tree unexpectedly moved the orphan: %#v", block.Children)
	}
}

func TestRemoveSubtreeRemovesParentAndDependentChildren(t *testing.T) {
	source := []byte("package main\n\nfunc main() {}\n")
	target := []byte("package main\n\nfunc main() {\n\tx := 1\n}\n")
	state, err := engine.NewWorkingStateFromSource(source, target)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := state.Snapshot()
	parent := -1
	for index, edit := range snapshot.Edits {
		for _, child := range snapshot.Edits {
			if child.ParentID == edit.NodeID {
				parent = index
				break
			}
		}
		if parent >= 0 {
			break
		}
	}
	if parent < 0 {
		t.Fatal("expected a parent edit")
	}
	if err := state.ApplySubtree(parent); err != nil {
		t.Fatal(err)
	}
	if err := state.RemoveSubtree(parent); err != nil {
		t.Fatal(err)
	}
	for index, status := range state.Snapshot().Status {
		if status == engine.EditApplied {
			t.Fatalf("edit %d remained applied after subtree removal", index)
		}
	}
}
