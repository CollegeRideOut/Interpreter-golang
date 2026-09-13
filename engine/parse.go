package engine

import (
	"fmt"
	goast "go/ast"
	goParser "go/parser"
	"go/token"
)

func parseGoAST(source []byte) (*goast.File, *token.FileSet, error) {
	fileSet := token.NewFileSet()
	file, err := goParser.ParseFile(fileSet, "example.go", source, 0)
	return file, fileSet, err
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

func goNodeLabel(node goast.Node) string {
	return fmt.Sprintf("%T", node)
}
