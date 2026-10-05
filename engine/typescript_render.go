package engine

import "strings"

const typeScriptStatementBoundary = "\x00statement-boundary"

func collectTypeScriptTokens(node *structuralASTNode, tokens *[]string) {
	if node == nil {
		return
	}
	if len(node.Children) == 0 {
		if node.Value != "" {
			*tokens = append(*tokens, node.Value)
		}
		return
	}
	if strings.HasSuffix(node.Kind, ":string") || strings.HasSuffix(node.Kind, ":template_string") {
		*tokens = append(*tokens, rawTypeScriptTokens(node))
		return
	}
	for _, child := range node.Children {
		collectTypeScriptTokens(child, tokens)
	}
	if isTypeScriptStatementNode(node.Kind) {
		*tokens = append(*tokens, typeScriptStatementBoundary)
	}
}

func rawTypeScriptTokens(node *structuralASTNode) string {
	if node == nil {
		return ""
	}
	if len(node.Children) == 0 {
		return node.Value
	}
	var builder strings.Builder
	for _, child := range node.Children {
		builder.WriteString(rawTypeScriptTokens(child))
	}
	return builder.String()
}

func formatTypeScriptTokens(tokens []string) string {
	lines := make([]string, 0)
	line := ""
	indent := 0
	parenDepth := 0
	inlineBraces := make([]bool, 0)
	flush := func() {
		if value := strings.TrimSpace(line); value != "" {
			lines = append(lines, strings.Repeat("  ", indent)+value)
		}
		line = ""
	}
	for index, token := range tokens {
		next := ""
		if index+1 < len(tokens) {
			next = tokens[index+1]
		}
		switch token {
		case typeScriptStatementBoundary:
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if !strings.HasSuffix(trimmed, ";") {
					line = trimmed + ";"
				}
				flush()
			}
		case ";":
			line = strings.TrimRight(line, " ") + ";"
			if parenDepth == 0 {
				flush()
			} else {
				line += " "
			}
		case "{":
			trimmedLine := strings.TrimSpace(line)
			inline := lastTypeScriptWord(line) == "import" || strings.HasPrefix(trimmedLine, "export {") || strings.HasSuffix(trimmedLine, "<") || strings.HasSuffix(trimmedLine, "(") || strings.HasSuffix(trimmedLine, "=") || isJSXExpressionStart(line)
			inlineBraces = append(inlineBraces, inline)
			if inline {
				if !strings.HasSuffix(line, "<") && !strings.HasSuffix(line, "(") {
					ensureTypeScriptSpace(&line)
				}
				line += "{ "
			} else {
				ensureTypeScriptSpace(&line)
				line += "{"
				flush()
				indent++
			}
		case "}":
			inline := len(inlineBraces) > 0 && inlineBraces[len(inlineBraces)-1]
			if len(inlineBraces) > 0 {
				inlineBraces = inlineBraces[:len(inlineBraces)-1]
			}
			if inline {
				line = strings.TrimRight(line, " ") + " }"
			} else {
				if strings.TrimSpace(line) != "" && next != "/" {
					flush()
				}
				if indent > 0 {
					indent--
				}
				line = "}"
				if next != ";" && next != ")" && next != "," && next != "else" && next != "catch" && next != "finally" {
					flush()
				}
			}
		case "(":
			if lastTypeScriptWord(line) == "if" || lastTypeScriptWord(line) == "for" || lastTypeScriptWord(line) == "while" || lastTypeScriptWord(line) == "switch" || lastTypeScriptWord(line) == "catch" || lastTypeScriptWord(line) == "async" {
				ensureTypeScriptSpace(&line)
			}
			line += "("
			parenDepth++
		case ")":
			line = strings.TrimRight(line, " ") + ")"
			if parenDepth > 0 {
				parenDepth--
			}
		case ",":
			line = strings.TrimRight(line, " ") + ", "
		case ":":
			line = strings.TrimRight(line, " ") + ": "
		case ".":
			line = strings.TrimRight(line, " ") + "."
		default:
			if token == "import" && strings.TrimSpace(line) != "" && parenDepth == 0 {
				flush()
			}
			if (token == "function" || token == "const" || token == "let" || token == "class" || token == "interface" || token == "export") && strings.TrimSpace(line) != "" && lastTypeScriptWord(line) != "export" && parenDepth == 0 && indent == 0 {
				flush()
			}
			if token == "<" && (isLikelyJSXOpen(line, next) || next == "/") {
				if next == "/" {
					line = strings.TrimRight(line, " ")
				} else if strings.TrimSpace(line) != "" {
					ensureTypeScriptSpace(&line)
				}
				line += "<"
			} else if token == "/" && strings.HasSuffix(line, "<") {
				line += "/"
			} else if token == "<" {
				line = strings.TrimRight(line, " ") + "<"
			} else if token == ">" && isJSXTagEnd(line) {
				line = strings.TrimRight(line, " ") + ">"
			} else if token == ">" {
				line = strings.TrimRight(line, " ") + ">"
			} else if isTypeScriptOperator(token) {
				ensureTypeScriptSpace(&line)
				line += token + " "
			} else {
				if needsTypeScriptSpace(line, token) {
					ensureTypeScriptSpace(&line)
				}
				line += token
			}
		}
	}
	flush()
	return strings.ReplaceAll(strings.Join(lines, "\n"), " </", "</")
}

func ensureTypeScriptSpace(line *string) {
	if *line != "" && !strings.HasSuffix(*line, " ") {
		*line += " "
	}
}

func lastTypeScriptWord(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[len(fields)-1], "({")
}

func needsTypeScriptSpace(line, token string) bool {
	if strings.TrimSpace(line) == "" || strings.HasSuffix(line, " ") || strings.HasSuffix(line, "(") || strings.HasSuffix(line, "[") || strings.HasSuffix(line, ".") || strings.HasSuffix(line, "<") {
		return false
	}
	if token == ")" || token == "]" || token == ">" || token == "?" {
		return false
	}
	last := line[len(line)-1]
	return (last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z') || (last >= '0' && last <= '9') || last == '\'' || last == '"' || last == '`' || last == ')' || last == ']' || last == '}'
}

func isTypeScriptOperator(token string) bool {
	switch token {
	case "=", "=>", "+", "-", "*", "/", "==", "===", "!=", "!==", "&&", "||", "|", "&":
		return true
	default:
		return false
	}
}

func isTypeScriptStatementNode(kind string) bool {
	for _, suffix := range []string{
		":expression_statement",
		":return_statement",
		":throw_statement",
		":break_statement",
		":continue_statement",
		":debugger_statement",
		":lexical_declaration",
		":import_statement",
		":export_statement",
	} {
		if strings.HasSuffix(kind, suffix) {
			return true
		}
	}
	return false
}

func isJSXTagStart(token string) bool {
	return token != "" && ((token[0] >= 'A' && token[0] <= 'Z') || (token[0] >= 'a' && token[0] <= 'z'))
}

func isLikelyJSXOpen(line, next string) bool {
	if !isJSXTagStart(next) {
		return false
	}
	trimmed := strings.TrimSpace(line)
	return strings.HasSuffix(trimmed, "return") || strings.HasSuffix(trimmed, "=") || strings.HasSuffix(trimmed, "=>") || strings.HasSuffix(trimmed, "(") || strings.HasSuffix(trimmed, ",")
}

func isJSXExpressionStart(line string) bool {
	return strings.LastIndex(line, "<") > strings.LastIndex(line, ">")
}

func isJSXTagEnd(line string) bool {
	return strings.Contains(line, "<") && !strings.Contains(line, " ")
}
