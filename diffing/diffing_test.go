package diffing

import (
	goast "go/ast"
	goParser "go/parser"
	"go/token"
	"strings"
	"testing"

	"interpreter/ast"
	"interpreter/lexer"
	"interpreter/parser"
	treeeditdistance "interpreter/treeEditDistance"
)

func TestGetTreeDiffIdenticalTrees(t *testing.T) {
	tree := treeeditdistance.NewNode("1", treeeditdistance.NewNode("2"))

	if edits := GetTreeDiff(tree, tree); len(edits) != 0 {
		t.Fatalf("edits = %v, want no edits", edits)
	}
}

func TestGetTreeDiffUpdate(t *testing.T) {
	source := treeeditdistance.NewNode("1")
	target := treeeditdistance.NewNode("2")

	edits := GetTreeDiff(source, target)
	if len(edits) != 1 {
		t.Fatalf("edits = %v, want one edit", edits)
	}
	if edits[0].Type != EditUpdate || edits[0].From.Label != "1" || edits[0].To.Label != "2" {
		t.Fatalf("edit = %+v, want update 1 -> 2", edits[0])
	}
}

func TestGetTreeDiffInsertAndDelete(t *testing.T) {
	source := treeeditdistance.NewNode("1")
	target := treeeditdistance.NewNode("1", treeeditdistance.NewNode("2"))

	inserted := GetTreeDiff(source, target)
	if len(inserted) != 1 || inserted[0].Type != EditInsert || inserted[0].To.Label != "2" {
		t.Fatalf("inserted edits = %+v, want insert 2", inserted)
	}

	deleted := GetTreeDiff(target, source)
	if len(deleted) != 1 || deleted[0].Type != EditDelete || deleted[0].From.Label != "2" {
		t.Fatalf("deleted edits = %+v, want delete 2", deleted)
	}
}

func TestGetTreeDiffNestedUpdate(t *testing.T) {
	source := treeeditdistance.NewNode("1", treeeditdistance.NewNode("2", treeeditdistance.NewNode("3")))
	target := treeeditdistance.NewNode("1", treeeditdistance.NewNode("2", treeeditdistance.NewNode("4")))

	edits := GetTreeDiff(source, target)
	if len(edits) != 1 {
		t.Fatalf("edits = %v, want one edit", edits)
	}
	if edits[0].Type != EditUpdate || edits[0].From.Label != "3" || edits[0].To.Label != "4" {
		t.Fatalf("edit = %+v, want update 3 -> 4", edits[0])
	}
}

func TestGetTreeDiffWholeTreeInsertAndDelete(t *testing.T) {
	tree := treeeditdistance.NewNode("1", treeeditdistance.NewNode("2", treeeditdistance.NewNode("3")))

	if edits := GetTreeDiff(nil, tree); len(edits) != 3 {
		t.Fatalf("insert edits = %d, want 3", len(edits))
	}
	if edits := GetTreeDiff(tree, nil); len(edits) != 3 {
		t.Fatalf("delete edits = %d, want 3", len(edits))
	}
}

func TestApplyEditPath(t *testing.T) {
	sourceTree, targetTree := treeeditdistance.CreateSimilarTrees()
	forest := []*treeeditdistance.Node{sourceTree.Root}

	for _, edit := range GetTreeDiff(sourceTree.Root, targetTree.Root) {
		forest = ApplyEdit(forest, edit)
	}

	if len(forest) != 1 || !sameShape(forest[0], targetTree.Root) {
		t.Fatalf("applied forest does not match target")
	}
}

func TestApplyEditsProducesTargetTree(t *testing.T) {
	source := treeeditdistance.NewNode(
		"program",
		treeeditdistance.NewNode(
			"assignment",
			treeeditdistance.NewNode("variable:x"),
			treeeditdistance.NewNode("value:5"),
		),
	)
	target := treeeditdistance.NewNode(
		"program",
		treeeditdistance.NewNode(
			"assignment",
			treeeditdistance.NewNode("variable:x"),
			treeeditdistance.NewNode("value:10"),
		),
		treeeditdistance.NewNode("print:remaining"),
	)

	forest := []*treeeditdistance.Node{source}
	for _, edit := range GetTreeDiff(source, target) {
		forest = ApplyEdit(forest, edit)
	}

	got := renderForest(forest)
	want := renderForest([]*treeeditdistance.Node{target})
	if got != want {
		t.Fatalf("replayed tree:\n%s\nwant:\n%s", got, want)
	}
}

func TestApplyGoASTEditsProducesTargetTree(t *testing.T) {
	source := parseGoProgram(t, `package main

func main() {
	x := 5
	_ = x
}`)
	target := parseGoProgram(t, `package main

func main() {
	x := 10
	_ = x
}`)

	sourceTree := GoASTToTree(source)
	targetTree := GoASTToTree(target)
	forest := []*treeeditdistance.Node{sourceTree}
	for _, edit := range GetTreeDiff(sourceTree, targetTree) {
		forest = ApplyEdit(forest, edit)
	}

	got := renderForest(forest)
	want := renderForest([]*treeeditdistance.Node{targetTree})
	if got != want {
		t.Fatalf("replayed Go AST:\n%s\nwant:\n%s", got, want)
	}
}

func parseGoProgram(t *testing.T, source string) *goast.File {
	t.Helper()

	program, err := goParser.ParseFile(token.NewFileSet(), "main.go", source, 0)
	if err != nil {
		t.Fatalf("parse Go program: %v", err)
	}

	return program
}

func renderForest(forest []*treeeditdistance.Node) string {
	var output strings.Builder
	for _, root := range forest {
		renderNode(&output, root, "")
	}
	return output.String()
}

func renderNode(output *strings.Builder, node *treeeditdistance.Node, indent string) {
	if node == nil {
		return
	}

	output.WriteString(indent)
	output.WriteString(node.Label)
	output.WriteByte('\n')
	for _, child := range node.Children {
		renderNode(output, child, indent+"  ")
	}
}

func TestMonkeyASTDiff(t *testing.T) {
	source := parseMonkeyProgram(t, "let x = 5;")
	target := parseMonkeyProgram(t, "let x = 10;")

	edits := GetTreeDiff(MonkeyASTToTree(source), MonkeyASTToTree(target))
	if len(edits) != 1 {
		t.Fatalf("edits = %v, want one edit", edits)
	}

	edit := edits[0]
	if edit.Type != EditUpdate || edit.From.Label != "IntegerLiteral:5" || edit.To.Label != "IntegerLiteral:10" {
		t.Fatalf("edit = %+v, want IntegerLiteral:5 -> IntegerLiteral:10", edit)
	}

	if _, ok := edit.From.Value.(*ast.IntegerLiteral); !ok {
		t.Fatalf("source value = %T, want *ast.IntegerLiteral", edit.From.Value)
	}
	if _, ok := edit.To.Value.(*ast.IntegerLiteral); !ok {
		t.Fatalf("target value = %T, want *ast.IntegerLiteral", edit.To.Value)
	}
}

func parseMonkeyProgram(t *testing.T, input string) *ast.Program {
	t.Helper()

	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	return program
}

func sameShape(source, target *treeeditdistance.Node) bool {
	if source == nil || target == nil {
		return source == target
	}
	if source.Label != target.Label || len(source.Children) != len(target.Children) {
		return false
	}
	for index := range source.Children {
		if !sameShape(source.Children[index], target.Children[index]) {
			return false
		}
	}
	return true
}
