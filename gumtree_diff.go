package main

import (
	"bytes"
	"fmt"
	goast "go/ast"
	goParser "go/parser"
	"go/token"
	"log/slog"

	gumast "github.com/Xanonymous-GitHub/gumtree-go/ast"
	"github.com/Xanonymous-GitHub/gumtree-go/comparator"
)

type inMemoryDiff struct {
	sourceTree          gumast.AST
	targetTree          gumast.AST
	focusedRoot         *gumast.Node
	sourceFile          string
	sourceSource        []byte
	targetSource        []byte
	workingSource       []byte
	scripts             []editScript
	scriptsByNode       map[gumast.NodeIdType]editScript
	scriptIndicesByNode map[gumast.NodeIdType][]int
	selected            []bool
	applied             []bool
	selectedRow         int
	undoStack           []appliedEdit
	redoStack           []appliedEdit
}

func prepareInMemoryDiff() (*inMemoryDiff, error) {
	source := []byte(`package main

import "fmt"

func calculate(expression string) int {
	left := int(expression[0] - '0')
	operator := expression[1]
	right := int(expression[2] - '0')

	switch operator {
	case '+':
		return left - right
	case '-':
		return left - right
	}

	return 0
}

func main() {
	fmt.Println(calculate("2+2"))
}`)
	target := []byte(`package main

import "fmt"

func calculate(expression string) int {
	left := int(expression[0] - '0')
	operator := expression[1]
	right := int(expression[2] - '0')

	switch operator {
	case '+':
		return left + right
	case '-':
		return left - right
	case '*':
		return left * right
	case '/':
		return left / right
	case '%':
		return left % right
	}

	return 0
}

func main() {
	fmt.Println(calculate("2+2"))
}`)

	sourceAST, sourceFileSet, err := parseGoAST(source)
	if err != nil {
		return nil, fmt.Errorf("parse source example: %w", err)
	}
	targetAST, targetFileSet, err := parseGoAST(target)
	if err != nil {
		return nil, fmt.Errorf("parse target example: %w", err)
	}

	sourceTree, sourceNodes, err := goASTToGumTree(sourceAST)
	if err != nil {
		return nil, fmt.Errorf("build source GumTree AST: %w", err)
	}
	targetTree, targetNodes, err := goASTToGumTree(targetAST)
	if err != nil {
		return nil, fmt.Errorf("build target GumTree AST: %w", err)
	}

	comparison := comparator.NewComparator(&sourceTree, &targetTree, 0, 10000, 0.5, *slog.Default())
	scripts := buildEditScripts(sourceTree.Root(), targetTree.Root(), sourceNodes, targetNodes, sourceFileSet, targetFileSet, source, target)
	_ = comparison.Compare()

	scriptsByNode := make(map[gumast.NodeIdType]editScript, len(scripts))
	scriptIndicesByNode := make(map[gumast.NodeIdType][]int)
	for index, script := range scripts {
		scriptsByNode[script.source.Id] = script
		scriptIndicesByNode[script.source.Id] = append(scriptIndicesByNode[script.source.Id], index)
	}

	return &inMemoryDiff{
		sourceTree:          sourceTree,
		targetTree:          targetTree,
		focusedRoot:         focusedASTRoot(scripts),
		sourceFile:          "main.go",
		sourceSource:        source,
		targetSource:        target,
		workingSource:       append([]byte(nil), source...),
		scripts:             scripts,
		scriptsByNode:       scriptsByNode,
		scriptIndicesByNode: scriptIndicesByNode,
		selected:            make([]bool, len(scripts)),
		applied:             make([]bool, len(scripts)),
	}, nil
}

func focusedASTRoot(scripts []editScript) *gumast.Node {
	if len(scripts) == 0 {
		return nil
	}

	focused := scripts[0].source
	for _, script := range scripts[1:] {
		focused = commonAncestor(focused, script.source)
	}
	if focused.Parent != nil && focused == scripts[0].source {
		focused = focused.Parent
	}
	return focused
}

func commonAncestor(left, right *gumast.Node) *gumast.Node {
	ancestors := make(map[*gumast.Node]bool)
	for node := left; node != nil; node = node.Parent {
		ancestors[node] = true
	}
	for node := right; node != nil; node = node.Parent {
		if ancestors[node] {
			return node
		}
	}
	return nil
}

type editScript struct {
	kind        editKind
	source      *gumast.Node
	target      *gumast.Node
	start       int
	end         int
	original    []byte
	replacement []byte
	description string
}

type editKind string

const (
	editUpdate editKind = "UPDATE"
	editInsert editKind = "INSERT"
)

type appliedEdit struct {
	index int
}

func (script editScript) reverse() editScript {
	reversed := editScript{
		kind:        editKindReverse(script.kind),
		source:      script.target,
		target:      script.source,
		start:       script.start,
		end:         script.start + len(script.replacement),
		original:    append([]byte(nil), script.replacement...),
		replacement: append([]byte(nil), script.original...),
	}
	return reversed
}

func (script editScript) String() string {
	if script.kind == editInsert {
		return "INSERT " + script.description
	}
	return fmt.Sprintf("UPDATE %s:%s -> %s:%s", script.source.Label, script.source.Value, script.target.Label, script.target.Value)
}

func editKindReverse(kind editKind) editKind {
	if kind == editInsert {
		return editDelete
	}
	return editUpdate
}

const editDelete editKind = "DELETE"

func parseGoAST(source []byte) (*goast.File, *token.FileSet, error) {
	fileSet := token.NewFileSet()
	file, err := goParser.ParseFile(fileSet, "example.go", source, 0)
	return file, fileSet, err
}

type gumTreeBuilder struct {
	tree  gumast.AST
	stack []*gumast.Node
	nodes map[gumast.NodeIdType]goast.Node
	err   error
}

func goASTToGumTree(root goast.Node) (gumast.AST, map[gumast.NodeIdType]goast.Node, error) {
	builder := &gumTreeBuilder{
		tree:  gumast.NewAST(*slog.Default()),
		nodes: make(map[gumast.NodeIdType]goast.Node),
	}
	goast.Walk(builder, root)
	return builder.tree, builder.nodes, builder.err
}

func (builder *gumTreeBuilder) Visit(node goast.Node) goast.Visitor {
	if builder.err != nil {
		return nil
	}

	if node == nil {
		if len(builder.stack) > 0 {
			builder.stack = builder.stack[:len(builder.stack)-1]
		}
		return nil
	}

	parent := (*gumast.Node)(nil)
	index := -1
	if len(builder.stack) > 0 {
		parent = builder.stack[len(builder.stack)-1]
		index = len(parent.Children)
	}

	created, err := builder.tree.Add(parent, index, gumast.NodeLabelType(goNodeLabel(node)), gumast.NodeValueType(goNodeValue(node)))
	if err != nil {
		builder.err = err
		return nil
	}

	builder.stack = append(builder.stack, created)
	builder.nodes[created.Id] = node
	return builder
}

func goNodeLabel(node goast.Node) string {
	return fmt.Sprintf("%T", node)
}

func goNodeValue(node goast.Node) string {
	switch node := node.(type) {
	case *goast.Ident:
		return node.Name
	case *goast.BasicLit:
		return node.Value
	case *goast.BinaryExpr:
		return node.Op.String()
	case *goast.UnaryExpr:
		return node.Op.String()
	case *goast.AssignStmt:
		return node.Tok.String()
	case *goast.GenDecl:
		return node.Tok.String()
	case *goast.File:
		return node.Name.Name
	default:
		return ""
	}
}

func printGumTree(node *gumast.Node, prefix string) {
	if node == nil {
		return
	}

	value := string(node.Value)
	if value != "" {
		fmt.Printf("%s%s [%s]\n", prefix, node.Label, value)
	} else {
		fmt.Printf("%s%s\n", prefix, node.Label)
	}
	for _, child := range node.OrderedChildren() {
		printGumTree(child, prefix+"  ")
	}
}

func buildEditScripts(source, target *gumast.Node, sourceNodes, targetNodes map[gumast.NodeIdType]goast.Node, sourceFileSet, targetFileSet *token.FileSet, sourceBytes, targetBytes []byte) []editScript {
	if source == nil || target == nil {
		return nil
	}

	var scripts []editScript
	var walk func(*gumast.Node, *gumast.Node)
	walk = func(sourceNode, targetNode *gumast.Node) {
		if sourceNode == nil || targetNode == nil {
			return
		}

		if sourceNode.Label == targetNode.Label && sourceNode.Value != targetNode.Value {
			if sourceValue, sourceOK := sourceNodes[sourceNode.Id]; sourceOK {
				if targetValue, targetOK := targetNodes[targetNode.Id]; targetOK {
					start := sourceFileSet.Position(sourceValue.Pos()).Offset
					end := sourceFileSet.Position(sourceValue.End()).Offset
					targetStart := targetFileSet.Position(targetValue.Pos()).Offset
					targetEnd := targetFileSet.Position(targetValue.End()).Offset
					scripts = append(scripts, editScript{
						kind:        editUpdate,
						source:      sourceNode,
						target:      targetNode,
						start:       start,
						end:         end,
						original:    append([]byte(nil), sourceBytes[start:end]...),
						replacement: append([]byte(nil), targetBytes[targetStart:targetEnd]...),
					})
				}
			}
		}

		sourceChildren := sourceNode.OrderedChildren()
		targetChildren := targetNode.OrderedChildren()
		limit := len(sourceChildren)
		if len(targetChildren) < limit {
			limit = len(targetChildren)
		}
		for index := 0; index < limit; index++ {
			walk(sourceChildren[index], targetChildren[index])
		}

		if len(targetChildren) > len(sourceChildren) {
			inserted := targetChildren[len(sourceChildren):]
			start := sourceFileSet.Position(sourceNodes[sourceNode.Id].End()).Offset - 1
			for _, child := range inserted {
				targetValue, ok := targetNodes[child.Id]
				if !ok {
					continue
				}
				targetStart := targetFileSet.Position(targetValue.Pos()).Offset
				targetEnd := targetFileSet.Position(targetValue.End()).Offset
				replacement := append([]byte(nil), targetBytes[targetStart:targetEnd]...)
				replacement = append(replacement, '\n', '\t')
				scripts = append(scripts, editScript{
					kind:        editInsert,
					source:      sourceNode,
					target:      child,
					start:       start,
					end:         start,
					original:    nil,
					replacement: replacement,
					description: "operator case " + caseOperator(child),
				})
			}
		}
	}

	walk(source, target)
	return scripts
}

func caseOperator(node *gumast.Node) string {
	if node == nil {
		return "?"
	}
	if node.Label == "*ast.BasicLit" {
		return string(node.Value)
	}
	for _, child := range node.OrderedChildren() {
		if operator := caseOperator(child); operator != "?" {
			return operator
		}
	}
	return "?"
}

func applyEdit(source []byte, script editScript) ([]byte, error) {
	if script.start < 0 || script.end < script.start || script.end > len(source) {
		return nil, fmt.Errorf("invalid edit range %d:%d", script.start, script.end)
	}
	if !bytes.Equal(source[script.start:script.end], script.original) {
		return nil, fmt.Errorf("edit precondition failed at %d:%d", script.start, script.end)
	}

	replacement := script.replacement
	updated := make([]byte, 0, len(source)+len(replacement)-(script.end-script.start))
	updated = append(updated, source[:script.start]...)
	updated = append(updated, replacement...)
	updated = append(updated, source[script.end:]...)
	return updated, nil
}
