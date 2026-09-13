package engine

import "strings"

// RenderResult is the best-effort source representation of a structural tree.
// Code may contain placeholders when the tree is intentionally incomplete.
type RenderResult struct {
	Code        string   `json:"code"`
	Diagnostics []string `json:"diagnostics,omitempty"`
}

// RenderBestEffort renders an intermediate tree without parsing it back into
// Go. This keeps invalid, partially applied trees inspectable.
func RenderBestEffort(root *Node) RenderResult {
	result := RenderResult{}
	if root == nil {
		result.Diagnostics = []string{"working tree is empty"}
		return result
	}
	result.Code = strings.TrimSpace(renderNode(root, 0, &result.Diagnostics)) + "\n"
	return result
}

func renderNode(node *structuralASTNode, indent int, diagnostics *[]string) string {
	if node == nil {
		return ""
	}

	switch node.Kind {
	case "*ast.File":
		name := renderField(node, "Name", indent, diagnostics)
		declarations := renderFieldLines(node, "Decls", indent, diagnostics)
		if name == "" {
			return placeholder(node, diagnostics)
		}
		if declarations == "" {
			return "package " + name
		}
		return "package " + name + "\n\n" + declarations
	case "*ast.FuncDecl":
		name := renderField(node, "Name", indent, diagnostics)
		typeNode := childForField(node, "Type")
		body := renderField(node, "Body", indent, diagnostics)
		if name == "" {
			return placeholder(node, diagnostics)
		}
		if body == "" {
			body = "{ /* body */ }"
		}
		return "func " + name + renderFuncType(typeNode, indent, diagnostics) + " " + body
	case "*ast.FuncLit":
		typeNode := childForField(node, "Type")
		bodyNode := childForField(node, "Body")
		functionType := "(/* params */)"
		if typeNode != nil {
			functionType = renderFuncType(typeNode, indent, diagnostics)
		}
		body := "{ /* body */ }"
		if bodyNode != nil {
			body = renderNode(bodyNode, indent, diagnostics)
		}
		prefix := "func"
		if typeNode == nil || bodyNode == nil {
			prefix = "STRUCTURALERROR.Function"
		}
		return prefix + functionType + " " + body
	case "*ast.BlockStmt":
		statements := renderFieldLines(node, "List", indent+1, diagnostics)
		orphaned := renderOrphanLines(node, indent+1, diagnostics)
		if statements != "" && orphaned != "" {
			statements += "\n" + orphaned
		} else if statements == "" {
			statements = orphaned
		}
		if statements == "" {
			return "{}"
		}
		return "{\n" + statements + "\n" + tabs(indent) + "}"
	case "*ast.ExprStmt":
		if len(node.Children) == 0 {
			return "STRUCTURALERROR.ExpressionStatement"
		}
		return renderFirstChild(node, indent, diagnostics)
	case "*ast.AssignStmt":
		left := renderFieldList(node, "Lhs", indent, diagnostics)
		right := renderFieldList(node, "Rhs", indent, diagnostics)
		if left == "" {
			left = "STRUCTURALERROR.Lhs"
		}
		if right == "" {
			right = "STRUCTURALERROR.Rhs"
		}
		op := node.Value
		if op == "" {
			op = "="
		}
		return left + " " + op + " " + right
	case "*ast.CallExpr":
		functionNode := childForField(node, "Fun")
		function := renderField(node, "Fun", indent, diagnostics)
		args := renderFieldList(node, "Args", indent, diagnostics)
		if functionNode == nil || function == "" {
			return "STRUCTURALERROR.FunctionCall(" + args + ")"
		}
		if functionNode.OriginalField == "Sel" {
			function = "STRUCTURALERROR.Selector." + function
		} else if functionNode.OriginalField == "X" {
			function = function + ".STRUCTURALERROR.Selector"
		}
		return function + "(" + args + ")"
	case "*ast.SelectorExpr":
		receiver := renderField(node, "X", indent, diagnostics)
		selector := renderField(node, "Sel", indent, diagnostics)
		if receiver == "" {
			return "STRUCTURALERROR.Selector"
		}
		if selector == "" {
			selector = "STRUCTURALERROR.Selector"
		}
		return receiver + "." + selector
	case "*ast.BinaryExpr":
		left := renderField(node, "X", indent, diagnostics)
		right := renderField(node, "Y", indent, diagnostics)
		if left == "" {
			left = "STRUCTURALERROR.BinaryLeft"
		}
		if right == "" {
			right = "STRUCTURALERROR.BinaryRight"
		}
		return left + " " + node.Value + " " + right
	case "*ast.UnaryExpr":
		operand := renderField(node, "X", indent, diagnostics)
		if operand == "" {
			operand = "STRUCTURALERROR.Operand"
		}
		return node.Value + operand
	case "*ast.ReturnStmt":
		values := renderFieldList(node, "Results", indent, diagnostics)
		if values == "" {
			return "return"
		}
		return "return " + values
	case "*ast.IfStmt":
		conditionNode := childForField(node, "Cond")
		condition := ""
		if hasConditionContent(conditionNode) {
			condition = renderNode(conditionNode, indent, diagnostics)
		}
		body := renderField(node, "Body", indent, diagnostics)
		if condition == "" {
			condition = "STRUCTURALERROR.Condition"
		}
		if body == "" {
			body = "{ /* body */ }"
		}
		return "if " + condition + " " + body
	case "*ast.GenDecl":
		specs := renderFieldLines(node, "Specs", indent, diagnostics)
		if specs == "" {
			return node.Value
		}
		if node.Value == "import" || node.Value == "const" || node.Value == "var" || node.Value == "type" {
			return node.Value + " (\n" + indentLines(specs, indent+1) + "\n" + tabs(indent) + ")"
		}
		return node.Value + " " + specs
	case "*ast.ImportSpec":
		return renderField(node, "Path", indent, diagnostics)
	case "*ast.FieldList":
		return "(" + renderFieldList(node, "List", indent, diagnostics) + ")"
	case "*ast.FuncType":
		return renderFuncType(node, indent, diagnostics)
	case "*ast.Field":
		names := renderFieldList(node, "Names", indent, diagnostics)
		typeName := renderField(node, "Type", indent, diagnostics)
		if names == "" {
			return typeName
		}
		return names + " " + typeName
	case "*ast.BasicLit", "*ast.Ident":
		return node.Value
	case "*ast.ParenExpr":
		return "(" + renderField(node, "X", indent, diagnostics) + ")"
	case "*ast.StarExpr":
		return "*" + renderField(node, "X", indent, diagnostics)
	case "*ast.IncDecStmt":
		return renderField(node, "X", indent, diagnostics) + node.Value
	case "*ast.DeclStmt":
		return renderFirstChild(node, indent, diagnostics)
	case "*ast.EmptyStmt":
		return ";"
	default:
		return placeholder(node, diagnostics)
	}
}

func hasConditionContent(node *structuralASTNode) bool {
	if node == nil {
		return false
	}
	if len(node.Children) > 0 {
		return true
	}
	switch node.Kind {
	case "*ast.Ident", "*ast.BasicLit":
		return node.Value != ""
	default:
		return false
	}
}

func renderFuncType(node *structuralASTNode, indent int, diagnostics *[]string) string {
	if node == nil {
		return "()"
	}
	params := renderField(node, "Params", indent, diagnostics)
	if params == "" {
		params = "()"
	}
	results := renderField(node, "Results", indent, diagnostics)
	if results == "" {
		return params
	}
	return params + " " + results
}

func renderField(node *structuralASTNode, field string, indent int, diagnostics *[]string) string {
	child := childForField(node, field)
	if child == nil {
		return ""
	}
	return renderNode(child, indent, diagnostics)
}

func renderFieldList(node *structuralASTNode, field string, indent int, diagnostics *[]string) string {
	values := make([]string, 0)
	for _, child := range childrenForField(node, field) {
		if value := renderNode(child, indent, diagnostics); value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, ", ")
}

func renderFieldLines(node *structuralASTNode, field string, indent int, diagnostics *[]string) string {
	values := make([]string, 0)
	for _, child := range childrenForField(node, field) {
		if value := renderNode(child, indent, diagnostics); value != "" {
			values = append(values, tabs(indent)+value)
		}
	}
	return strings.Join(values, "\n")
}

func renderFirstChild(node *structuralASTNode, indent int, diagnostics *[]string) string {
	if len(node.Children) == 0 {
		return placeholder(node, diagnostics)
	}
	return renderNode(node.Children[0], indent, diagnostics)
}

func childForField(node *structuralASTNode, field string) *structuralASTNode {
	for _, child := range node.Children {
		if child.Field == field {
			return child
		}
	}
	return nil
}

func placeholder(node *structuralASTNode, diagnostics *[]string) string {
	message := "unsupported or incomplete " + node.Kind
	*diagnostics = append(*diagnostics, message)
	return structuralErrorToken(node)
}

func renderOrphanLines(node *structuralASTNode, indent int, diagnostics *[]string) string {
	lines := make([]string, 0)
	for _, child := range node.Children {
		if child.Field == "List" || (child.Field == "child" && isGoStatementKind(child.Kind)) {
			if child.Field == "child" && isGoStatementKind(child.Kind) {
				lines = append(lines, tabs(indent)+renderNode(child, indent, diagnostics))
			}
			continue
		}
		*diagnostics = append(*diagnostics, "orphaned "+child.Kind)
		content := renderNode(child, indent, diagnostics)
		lines = append(lines, tabs(indent)+structuralErrorWrapper(child, content))
	}
	return strings.Join(lines, "\n")
}

func structuralErrorWrapper(node *structuralASTNode, content string) string {
	return "STRUCTURALERROR." + structuralErrorName(node) + "(" + content + ")"
}

func structuralErrorName(node *structuralASTNode) string {
	if node == nil {
		return "Node"
	}
	switch node.Kind {
	case "*ast.BasicLit":
		return "Lit"
	case "*ast.Ident":
		return "Identifier"
	case "*ast.FuncLit":
		return "Function"
	case "*ast.FuncType":
		return "FunctionType"
	case "*ast.CallExpr":
		return "FunctionCall"
	case "*ast.SelectorExpr":
		return "Selector"
	case "*ast.IfStmt":
		return "If"
	case "*ast.BinaryExpr":
		return "BinaryExpr"
	default:
		return "Node"
	}
}

func structuralErrorToken(node *structuralASTNode) string {
	return "STRUCTURALERROR." + structuralErrorName(node)
}

func indentLines(value string, indent int) string {
	prefix := tabs(indent)
	return prefix + strings.ReplaceAll(value, "\n", "\n"+prefix)
}

func tabs(indent int) string {
	return strings.Repeat("\t", indent)
}
