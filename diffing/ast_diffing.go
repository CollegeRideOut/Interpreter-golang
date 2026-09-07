package diffing

import (
	"fmt"
	"interpreter/ast"

	treeeditdistance "interpreter/treeEditDistance"
)

// MonkeyASTToTree converts a Monkey AST into the generic tree representation
// used by the tree differ. It preserves the AST structure without rewriting it.
func MonkeyASTToTree(node ast.Node) *treeeditdistance.Node {
	if node == nil {
		return nil
	}

	switch node := node.(type) {
	case *ast.Program:
		return newASTTreeNode("Program", node, statements(node.Statements)...)
	case *ast.LetStatement:
		return newASTTreeNode("LetStatement", node, nodeChildren(node.Name, node.Value)...)
	case *ast.ReturnStatement:
		return newASTTreeNode("ReturnStatement", node, nodeChildren(node.ReturnValue)...)
	case *ast.ExpressionStatement:
		return newASTTreeNode("ExpressionStatement", node, nodeChildren(node.Expression)...)
	case *ast.Identifier:
		return treeeditdistance.NewNodeWithValue("Identifier:"+node.Value, node)
	case *ast.IntegerLiteral:
		return treeeditdistance.NewNodeWithValue(fmt.Sprintf("IntegerLiteral:%d", node.Value), node)
	case *ast.Boolean:
		return treeeditdistance.NewNodeWithValue(fmt.Sprintf("Boolean:%t", node.Value), node)
	case *ast.PrefixExpression:
		return newASTTreeNode("PrefixExpression:"+node.Operator, node, nodeChildren(node.Right)...)
	case *ast.InfixExpression:
		return newASTTreeNode("InfixExpression:"+node.Operator, node, nodeChildren(node.Left, node.Right)...)
	case *ast.IfExpression:
		return newASTTreeNode(
			"IfExpression",
			node,
			nodeChildren(node.Condition, node.Consequence, node.Alternative)...,
		)
	case *ast.BlockStatement:
		return newASTTreeNode("BlockStatement", node, statements(node.Statements)...)
	case *ast.FunctionLiteral:
		children := make([]*treeeditdistance.Node, 0, len(node.Parameters)+1)
		for _, parameter := range node.Parameters {
			children = append(children, MonkeyASTToTree(parameter))
		}
		children = append(children, nodeChildren(node.Body)...)
		return newASTTreeNode("FunctionLiteral", node, children...)
	case *ast.CallExpression:
		children := nodeChildren(node.Function)
		children = append(children, expressions(node.Arguments)...)
		return newASTTreeNode("CallExpression", node, children...)
	default:
		return treeeditdistance.NewNodeWithValue(fmt.Sprintf("Unknown:%T", node), node)
	}
}

func newASTTreeNode(label string, value any, children ...*treeeditdistance.Node) *treeeditdistance.Node {
	return treeeditdistance.NewNodeWithValue(label, value, children...)
}

func nodeChildren(nodes ...ast.Node) []*treeeditdistance.Node {
	children := make([]*treeeditdistance.Node, 0, len(nodes))
	for _, node := range nodes {
		if node != nil {
			children = append(children, MonkeyASTToTree(node))
		}
	}
	return children
}

func statements(nodes []ast.Statement) []*treeeditdistance.Node {
	children := make([]*treeeditdistance.Node, 0, len(nodes))
	for _, node := range nodes {
		children = append(children, MonkeyASTToTree(node))
	}
	return children
}

func expressions(nodes []ast.Expression) []*treeeditdistance.Node {
	children := make([]*treeeditdistance.Node, 0, len(nodes))
	for _, node := range nodes {
		children = append(children, MonkeyASTToTree(node))
	}
	return children
}
