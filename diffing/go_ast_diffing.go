package diffing

import (
	"fmt"
	goast "go/ast"
	gotoken "go/token"

	treeeditdistance "interpreter/treeEditDistance"
)

// GoASTToTree converts a Go AST into the generic tree representation while
// preserving each original go/ast node in the generic node's Value field.
func GoASTToTree(node goast.Node) *treeeditdistance.Node {
	if node == nil {
		return nil
	}

	builder := &goTreeBuilder{}
	goast.Walk(builder, node)
	return builder.root
}

type goTreeBuilder struct {
	root  *treeeditdistance.Node
	stack []*treeeditdistance.Node
}

func (builder *goTreeBuilder) Visit(node goast.Node) goast.Visitor {
	if node == nil {
		builder.stack = builder.stack[:len(builder.stack)-1]
		return nil
	}

	treeNode := treeeditdistance.NewNodeWithValue(goNodeLabel(node), node)
	if len(builder.stack) == 0 {
		builder.root = treeNode
	} else {
		parent := builder.stack[len(builder.stack)-1]
		parent.Children = append(parent.Children, treeNode)
	}

	builder.stack = append(builder.stack, treeNode)
	return builder
}

func goNodeLabel(node goast.Node) string {
	label := fmt.Sprintf("%T", node)

	switch node := node.(type) {
	case *goast.Ident:
		return "Ident:" + node.Name
	case *goast.BasicLit:
		return "BasicLit:" + node.Kind.String() + ":" + node.Value
	case *goast.BinaryExpr:
		return "BinaryExpr:" + node.Op.String()
	case *goast.UnaryExpr:
		return "UnaryExpr:" + node.Op.String()
	case *goast.AssignStmt:
		return "AssignStmt:" + node.Tok.String()
	case *goast.IncDecStmt:
		return "IncDecStmt:" + node.Tok.String()
	case *goast.BranchStmt:
		return "BranchStmt:" + node.Tok.String()
	case *goast.ChanType:
		switch node.Dir {
		case goast.SEND:
			return "ChanType:send"
		case goast.RECV:
			return "ChanType:receive"
		default:
			return "ChanType:bidirectional"
		}
	case *goast.ImportSpec:
		if node.Path != nil {
			return "ImportSpec:" + node.Path.Value
		}
	case *goast.File:
		return "File:" + node.Name.Name
	case *goast.FuncType:
		if node.TypeParams != nil {
			return "FuncType:generic"
		}
	}

	if tokenLabel := goTokenLabel(node); tokenLabel != "" {
		return label + ":" + tokenLabel
	}
	return label
}

func goTokenLabel(node goast.Node) string {
	switch node := node.(type) {
	case *goast.ArrayType:
		return "array"
	case *goast.Ellipsis:
		return "..."
	case *goast.FuncLit:
		return "func"
	case *goast.FuncType:
		return "func"
	case *goast.MapType:
		return "map"
	case *goast.StructType:
		return "struct"
	case *goast.InterfaceType:
		return "interface"
	case *goast.TypeSpec:
		return "type"
	case *goast.ValueSpec:
		return "value"
	case *goast.GenDecl:
		return node.Tok.String()
	case *goast.ReturnStmt:
		return gotoken.RETURN.String()
	case *goast.DeferStmt:
		return gotoken.DEFER.String()
	case *goast.GoStmt:
		return gotoken.GO.String()
	case *goast.IfStmt:
		return gotoken.IF.String()
	case *goast.ForStmt:
		return gotoken.FOR.String()
	case *goast.RangeStmt:
		return gotoken.RANGE.String()
	case *goast.SwitchStmt:
		return gotoken.SWITCH.String()
	case *goast.TypeSwitchStmt:
		return gotoken.SWITCH.String()
	case *goast.SelectStmt:
		return gotoken.SELECT.String()
	case *goast.CaseClause:
		return gotoken.CASE.String()
	case *goast.CommClause:
		return gotoken.CASE.String()
	}

	return ""
}
