package main

import (
	"bytes"
	goast "go/ast"
	"go/token"
	"strings"

	gumast "github.com/Xanonymous-GitHub/gumtree-go/ast"
)

// draftNode is the mutable AST representation used while a user explores a
// change. It deliberately permits missing children and invalid combinations.
type draftNode struct {
	id       gumast.NodeIdType
	label    gumast.NodeLabelType
	value    gumast.NodeValueType
	parent   *draftNode
	children []*draftNode
	role     string
	text     []byte
	dirty    bool
}

func newDraftTree(root *gumast.Node) *draftNode {
	if root == nil {
		return nil
	}
	return cloneDraftNode(root, nil)
}

func cloneDraftNode(node *gumast.Node, parent *draftNode) *draftNode {
	if node == nil {
		return nil
	}
	clone := &draftNode{
		id:     node.Id,
		label:  node.Label,
		value:  node.Value,
		parent: parent,
	}
	for _, child := range node.OrderedChildren() {
		clone.children = append(clone.children, cloneDraftNode(child, clone))
	}
	return clone
}

func newDraftTreeFromAST(root *gumast.Node, nodes map[gumast.NodeIdType]goast.Node, fileSet *token.FileSet, source []byte) *draftNode {
	return cloneDraftNodeFromAST(root, nil, nodes, fileSet, source)
}

func cloneDraftNodeFromAST(node *gumast.Node, parent *draftNode, nodes map[gumast.NodeIdType]goast.Node, fileSet *token.FileSet, source []byte) *draftNode {
	if node == nil {
		return nil
	}
	clone := &draftNode{
		id:     node.Id,
		label:  node.Label,
		value:  node.Value,
		parent: parent,
	}
	if goNode := nodes[node.Id]; goNode != nil {
		start := fileSet.File(goNode.Pos()).Offset(goNode.Pos())
		end := fileSet.File(goNode.End()).Offset(goNode.End())
		if start >= 0 && end >= start && end <= len(source) {
			clone.text = append([]byte(nil), source[start:end]...)
		}
	}
	for index, child := range node.OrderedChildren() {
		childDraft := cloneDraftNodeFromAST(child, clone, nodes, fileSet, source)
		childDraft.role = draftChildRole(nodes[node.Id], nodes[child.Id], index)
		clone.children = append(clone.children, childDraft)
	}
	return clone
}

func (node *draftNode) clone(parent *draftNode) *draftNode {
	if node == nil {
		return nil
	}
	clone := &draftNode{
		id:     node.id,
		label:  node.label,
		value:  node.value,
		parent: parent,
		role:   node.role,
		text:   append([]byte(nil), node.text...),
		dirty:  node.dirty,
	}
	for _, child := range node.children {
		clone.children = append(clone.children, child.clone(clone))
	}
	return clone
}

func (node *draftNode) find(id gumast.NodeIdType) *draftNode {
	if node == nil {
		return nil
	}
	if node.id == id {
		return node
	}
	for _, child := range node.children {
		if found := child.find(id); found != nil {
			return found
		}
	}
	return nil
}

func (node *draftNode) insertChild(index int, child *draftNode) {
	if node == nil || child == nil {
		return
	}
	if child.parent != nil {
		child.parent.removeChild(child)
	}
	if index < 0 || index > len(node.children) {
		index = len(node.children)
	}
	child.parent = node
	node.markDirty()
	node.children = append(node.children, nil)
	copy(node.children[index+1:], node.children[index:])
	node.children[index] = child
}

func (node *draftNode) appendChild(child *draftNode) {
	node.insertChild(len(node.children), child)
}

func (node *draftNode) removeChild(child *draftNode) bool {
	if node == nil || child == nil {
		return false
	}
	for index, candidate := range node.children {
		if candidate != child {
			continue
		}
		copy(node.children[index:], node.children[index+1:])
		node.children = node.children[:len(node.children)-1]
		child.parent = nil
		node.markDirty()
		return true
	}
	return false
}

func (node *draftNode) markDirty() {
	for current := node; current != nil; current = current.parent {
		current.dirty = true
	}
}

func (node *draftNode) detach() {
	if node == nil || node.parent == nil {
		return
	}
	node.parent.removeChild(node)
}

func draftPath(node *draftNode) []*draftNode {
	path := make([]*draftNode, 0)
	for current := node; current != nil; current = current.parent {
		path = append(path, current)
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}

func renderDraft(root *draftNode) []byte {
	if root == nil {
		return nil
	}
	return []byte(renderDraftNode(root))
}

func renderDraftNode(node *draftNode) string {
	if node == nil {
		return ""
	}
	if !node.dirty && len(node.text) > 0 {
		return string(node.text)
	}

	children := func(role string) []string {
		values := make([]string, 0)
		for _, child := range node.children {
			if role == "" || child.role == role {
				values = append(values, renderDraftNode(child))
			}
		}
		return values
	}
	join := func(role, separator string) string {
		return strings.Join(children(role), separator)
	}

	switch node.label {
	case "*ast.File":
		packageName := children("package")
		declarations := children("decl")
		if len(packageName) == 0 {
			return strings.Join(declarations, "\n\n")
		}
		return "package " + packageName[0] + "\n\n" + strings.Join(declarations, "\n\n")
	case "*ast.GenDecl":
		if node.value == "import" {
			return "import (\n" + indent(join("spec", "\n")) + "\n)"
		}
		return string(node.value) + " " + join("spec", "\n")
	case "*ast.ImportSpec":
		return join("name", " ") + join("path", "")
	case "*ast.FuncDecl":
		name := join("name", "")
		functionType := join("type", "")
		if strings.HasPrefix(functionType, "func ") {
			return functionType + " " + join("body", "")
		}
		return "func " + name + functionType + " " + join("body", "")
	case "*ast.BlockStmt":
		body := join("stmt", "\n")
		if body == "" {
			return "{}"
		}
		return "{\n" + indent(body) + "\n}"
	case "*ast.ExprStmt":
		return join("x", "")
	case "*ast.CallExpr":
		return join("fun", "") + "(" + join("arg", ", ") + ")"
	case "*ast.SelectorExpr":
		return join("x", "") + "." + join("sel", "")
	case "*ast.FuncLit":
		return "func" + join("type", "") + " " + join("body", "")
	case "*ast.IfStmt":
		return "if " + join("cond", "") + " " + join("body", "")
	case "*ast.AssignStmt":
		return join("lhs", ", ") + " " + string(node.value) + " " + join("rhs", ", ")
	case "*ast.ReturnStmt":
		return "return " + join("result", ", ")
	case "*ast.BasicLit", "*ast.Ident":
		if len(node.text) > 0 {
			return string(node.text)
		}
		return string(node.value)
	}

	if len(node.text) > 0 {
		return string(node.text)
	}
	return string(node.value)
}

func indent(value string) string {
	lines := bytes.Split([]byte(value), []byte("\n"))
	for index := range lines {
		lines[index] = append([]byte("\t"), lines[index]...)
	}
	return string(bytes.Join(lines, []byte("\n")))
}

func draftChildRole(parent, child goast.Node, index int) string {
	switch parent := parent.(type) {
	case *goast.File:
		if child == parent.Name {
			return "package"
		}
		return "decl"
	case *goast.GenDecl:
		return "spec"
	case *goast.ImportSpec:
		if child == parent.Name {
			return "name"
		}
		return "path"
	case *goast.FuncDecl:
		if child == parent.Recv {
			return "recv"
		}
		if child == parent.Name {
			return "name"
		}
		if child == parent.Type {
			return "type"
		}
		return "body"
	case *goast.BlockStmt:
		return "stmt"
	case *goast.ExprStmt:
		return "x"
	case *goast.CallExpr:
		if child == parent.Fun {
			return "fun"
		}
		return "arg"
	case *goast.SelectorExpr:
		if child == parent.X {
			return "x"
		}
		return "sel"
	case *goast.FuncLit:
		if child == parent.Type {
			return "type"
		}
		return "body"
	case *goast.IfStmt:
		if child == parent.Init {
			return "init"
		}
		if child == parent.Cond {
			return "cond"
		}
		if child == parent.Body {
			return "body"
		}
		return "else"
	case *goast.AssignStmt:
		if index < len(parent.Lhs) {
			return "lhs"
		}
		return "rhs"
	case *goast.ReturnStmt:
		return "result"
	case *goast.BasicLit, *goast.Ident:
		return "value"
	}
	return "child"
}
