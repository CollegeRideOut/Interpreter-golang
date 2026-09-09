package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gumast "github.com/Xanonymous-GitHub/gumtree-go/ast"
	"github.com/Xanonymous-GitHub/gumtree-go/comparator"
)

func TestDirectoryDiffReachesWorkingTree(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.scripts) == 0 {
		t.Fatal("directory diff produced no scripts")
	}
	if len(diff.scripts) == 1 {
		t.Fatal("directory diff collapsed into one file-level edit")
	}
	for index := range diff.scripts {
		diff.selected[index] = true
		applySelectedEdits(diff)
	}

	if string(diff.workingSource) != string(diff.targetSource) {
		t.Fatalf("applied source does not match working tree:\ngot:\n%s\nwant:\n%s", diff.workingSource, diff.targetSource)
	}
}

func TestDirectoryDiffUsesWorkingTreePath(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}

	want, err := filepath.Abs(filepath.Join("TestProgram", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if diff.sourceFile != want {
		t.Fatalf("source file = %q, want %q", diff.sourceFile, want)
	}
	if _, err := os.Stat(diff.sourceFile); err != nil {
		t.Fatalf("source file is not on disk: %v", err)
	}
}

func TestASTRowsKeepEditsIndependent(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	rows := makeASTRows(diff)

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
	foundIf := false
	for _, row := range rows {
		if row.node != nil && row.node.Label == "*ast.IfStmt" {
			foundIf = true
			break
		}
	}
	if !foundIf {
		t.Fatal("AST rows did not expose the inserted if statement")
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

func TestDirectoryDiffUndoRedoRepeatedly(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	original := string(diff.sourceSource)
	target := string(diff.targetSource)

	for index := range diff.scripts {
		diff.selected[index] = true
		applySelectedEdits(diff)
	}
	if got := string(diff.workingSource); got != target {
		t.Fatalf("after applying all edits = %q, want target", got)
	}

	for len(diff.undoStack) > 0 {
		undoEdit(diff)
	}
	if got := string(diff.workingSource); got != original {
		t.Fatalf("after undoing all edits = %q, want source", got)
	}

	for len(diff.redoStack) > 0 {
		redoEdit(diff)
	}
	if got := string(diff.workingSource); got != target {
		t.Fatalf("after redoing all edits = %q, want target", got)
	}
	if !strings.Contains(target, "http.ListenAndServe") {
		t.Fatal("test target does not contain the expected web server")
	}
}

func TestToggleAppliedEdit(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}

	for index := range diff.scripts {
		toggleEdit(diff, index)
	}
	if got := string(diff.workingSource); got != string(diff.targetSource) {
		t.Fatalf("after toggling all edits = %q, want target", got)
	}

	toggleEdit(diff, 0)
	if diff.applied[0] {
		t.Fatal("toggled edit remained applied")
	}
	if string(diff.workingSource) == string(diff.targetSource) {
		t.Fatal("removing an applied edit did not change working source")
	}

	toggleEdit(diff, 0)
	if got := string(diff.workingSource); got != string(diff.targetSource) {
		t.Fatalf("after toggling edit back on = %q, want target", got)
	}
}

func TestInvalidDraftIsReported(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	diff.workingSource = []byte("package main\nfunc main() {")
	updateParseStatus(diff)
	if diff.parseError == "" {
		t.Fatal("invalid draft did not produce a parse diagnostic")
	}
	diff.workingSource = diff.targetSource
	updateParseStatus(diff)
	if diff.parseError != "" {
		t.Fatalf("valid draft retained parse diagnostic: %s", diff.parseError)
	}
}

func TestApplyingParentKeepsDraftChildrenIndependent(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	statement := -1
	for index, script := range diff.scripts {
		if script.kind == editInsert && script.target != nil && script.target.Label == "*ast.ExprStmt" {
			statement = index
			break
		}
	}
	if statement < 0 {
		t.Fatal("missing inserted expression statement")
	}

	toggleEdit(diff, statement)
	inserted := diff.workingDraft.find(diff.scripts[statement].target.Id)
	if inserted == nil {
		t.Fatal("parent edit was not added to the draft")
	}
	if strings.Contains(string(diff.workingSource), "http.HandleFunc") {
		t.Fatalf("parent projection materialized call children:\n%s", diff.workingSource)
	}
	for _, child := range inserted.children {
		if child.label == "*ast.CallExpr" {
			t.Fatal("parent edit materialized its call child")
		}
	}
}

func TestApplyingChildBuildsItsStructuralPath(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	call := -1
	for index, script := range diff.scripts {
		if script.kind == editInsert && script.target != nil && script.target.Label == "*ast.CallExpr" {
			call = index
			break
		}
	}
	if call < 0 {
		t.Fatal("missing inserted call expression")
	}

	toggleEdit(diff, call)
	inserted := diff.workingDraft.find(diff.scripts[call].target.Id)
	if inserted == nil || inserted.parent == nil {
		t.Fatal("child edit was not attached to the draft")
	}
	if inserted.parent.label != "*ast.ExprStmt" {
		t.Fatalf("call parent = %s, want ExprStmt", inserted.parent.label)
	}
	if !strings.Contains(string(diff.workingSource), "()") || strings.Contains(string(diff.workingSource), "\"/\"") {
		t.Fatalf("call projection did not preserve only the selected call structure:\n%s", diff.workingSource)
	}
}

func TestImportParentAndSpecComposeInDraft(t *testing.T) {
	diff, err := prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	genDecl, importSpec := -1, -1
	for index, script := range diff.scripts {
		if script.kind != editInsert || script.target == nil {
			continue
		}
		switch script.target.Label {
		case "*ast.GenDecl":
			genDecl = index
		case "*ast.ImportSpec":
			importSpec = index
		}
	}
	if genDecl < 0 || importSpec < 0 {
		t.Fatalf("missing import edits: declaration=%d spec=%d", genDecl, importSpec)
	}

	toggleEdit(diff, genDecl)
	if !strings.Contains(string(diff.workingSource), "import (") || strings.Contains(string(diff.workingSource), "\"log\"") {
		t.Fatalf("import declaration materialized its spec children:\n%s", diff.workingSource)
	}

	diff, err = prepareDirectoryDiff("TestProgram")
	if err != nil {
		t.Fatal(err)
	}
	toggleEdit(diff, importSpec)
	if !strings.Contains(string(diff.workingSource), "import (") || strings.Contains(string(diff.workingSource), "\"log\"") {
		t.Fatalf("import spec did not compose through its parent:\n%s", diff.workingSource)
	}
}

func TestIfStatementIsOneAtomicEdit(t *testing.T) {
	source := []byte("package main\n\nfunc main() {\n}\n")
	target := []byte("package main\n\nfunc main() {\n\tif true {\n\t\tprintln(\"hello\")\n\t}\n}\n")

	sourceAST, sourceFileSet, err := parseGoAST(source)
	if err != nil {
		t.Fatal(err)
	}
	targetAST, targetFileSet, err := parseGoAST(target)
	if err != nil {
		t.Fatal(err)
	}
	sourceTree, sourceNodes, err := goASTToGumTree(sourceAST)
	if err != nil {
		t.Fatal(err)
	}
	targetTree, targetNodes, err := goASTToGumTree(targetAST)
	if err != nil {
		t.Fatal(err)
	}
	mappings := comparator.NewComparator(&sourceTree, &targetTree, 0, 10000, 0.5, *slog.Default()).Compare()
	scripts := buildEditScripts(sourceTree.Root(), targetTree.Root(), sourceNodes, targetNodes, sourceFileSet, targetFileSet, mappings, source, target)
	if len(scripts) < 2 {
		t.Fatalf("scripts = %d, want the if edit and finer-grained descendants", len(scripts))
	}
	ifScript := -1
	for index, script := range scripts {
		if script.target != nil && script.target.Label == "*ast.IfStmt" {
			ifScript = index
			break
		}
	}
	if ifScript < 0 {
		t.Fatal("missing atomic if edit")
	}
	if string(scripts[ifScript].replacement) != "if {}" {
		t.Fatalf("replacement was not shallow: %q", scripts[ifScript].replacement)
	}

	updated, err := applyEdit(source, scripts[ifScript])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "println") {
		t.Fatalf("if edit unexpectedly included its child body:\n%s", updated)
	}
}

func TestInsertedCallAndIfAreIndependentCandidates(t *testing.T) {
	source := []byte("package main\n\nfunc main() {\n}\n")
	target := []byte("package main\n\nfunc main() {\n\thttp.HandleFunc(\"/\", func(w http.ResponseWriter, r *http.Request) {\n\t\tif r.URL.Path != \"/\" {\n\t\t\thttp.NotFound(w, r)\n\t\t\treturn\n\t\t}\n\t})\n}\n")
	scripts := scriptsForSources(t, source, target)

	statement, call, ifEdit := -1, -1, -1
	for index, script := range scripts {
		if script.target == nil {
			continue
		}
		switch script.target.Label {
		case "*ast.ExprStmt":
			statement = index
		case "*ast.CallExpr":
			call = index
		case "*ast.IfStmt":
			ifEdit = index
		}
	}
	if statement < 0 || call < 0 || ifEdit < 0 {
		t.Fatalf("missing statement, call, or if candidate: statement=%d call=%d if=%d", statement, call, ifEdit)
	}
	if string(scripts[statement].replacement) != ";" {
		t.Fatalf("expression statement included children: %q", scripts[statement].replacement)
	}
	if strings.Contains(string(scripts[call].replacement), "if r.URL.Path") {
		t.Fatalf("call candidate included the if body: %q", scripts[call].replacement)
	}
	if string(scripts[ifEdit].replacement) != "if {}" {
		t.Fatalf("if candidate was not shallow: %q", scripts[ifEdit].replacement)
	}

	updated, err := applyEdit(source, scripts[call])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "if r.URL.Path") {
		t.Fatalf("applying only call inserted child body:\n%s", updated)
	}
}

func scriptsForSources(t *testing.T, source, target []byte) []editScript {
	t.Helper()
	sourceAST, sourceFileSet, err := parseGoAST(source)
	if err != nil {
		t.Fatal(err)
	}
	targetAST, targetFileSet, err := parseGoAST(target)
	if err != nil {
		t.Fatal(err)
	}
	sourceTree, sourceNodes, err := goASTToGumTree(sourceAST)
	if err != nil {
		t.Fatal(err)
	}
	targetTree, targetNodes, err := goASTToGumTree(targetAST)
	if err != nil {
		t.Fatal(err)
	}
	mappings := comparator.NewComparator(&sourceTree, &targetTree, 0, 10000, 0.5, *slog.Default()).Compare()
	return buildEditScripts(sourceTree.Root(), targetTree.Root(), sourceNodes, targetNodes, sourceFileSet, targetFileSet, mappings, source, target)
}
