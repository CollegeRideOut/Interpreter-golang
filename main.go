package main

import (
	"fmt"
	goParser "go/parser"
	"go/token"
	"interpreter/ast"
	"interpreter/diffing"
	"interpreter/lexer"
	"interpreter/parser"
	treeeditdistance "interpreter/treeEditDistance"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {

	runTestProgram()

}

func runMonekyTestProgram() {

	sourceProgram := parseMonkeyProgram("let x = 5;")
	targetProgram := parseMonkeyProgram("let x = 10;")
	sourceTree := diffing.MonkeyASTToTree(sourceProgram)
	targetTree := diffing.MonkeyASTToTree(targetProgram)
	edits := diffing.GetTreeDiff(sourceTree, targetTree)

	fmt.Println("Source Monkey AST:")
	printTree(sourceTree, "", true, true)

	fmt.Println("Target Monkey AST:")
	printTree(targetTree, "", true, true)

	fmt.Println("Edit path:")
	for _, edit := range edits {
		switch edit.Type {
		case diffing.EditInsert:
			fmt.Printf("%s %s\n", edit.Type, edit.To.Label)
		case diffing.EditDelete:
			fmt.Printf("%s %s\n", edit.Type, edit.From.Label)
		case diffing.EditUpdate:
			fmt.Printf("%s %s -> %s\n", edit.Type, edit.From.Label, edit.To.Label)
		}
	}

}

func runTestProgram() {
	clonePath, err := clonePreviousTestProgram()
	if err != nil {
		panic(err)
	}
	fmt.Printf("TestProgram clone at HEAD~1: %s\n", clonePath)

	beforeSource, err := readTestProgramAt("HEAD~1")
	if err != nil {
		panic(err)
	}

	currentSource, err := readTestProgramAt("HEAD")
	if err != nil {
		panic(err)
	}

	beforeAST, err := goParser.ParseFile(token.NewFileSet(), "main.go", beforeSource, 0)
	if err != nil {
		panic(err)
	}

	currentAST, err := goParser.ParseFile(token.NewFileSet(), "main.go", currentSource, 0)
	if err != nil {
		panic(err)
	}

	beforeTree := diffing.GoASTToTree(beforeAST)
	currentTree := diffing.GoASTToTree(currentAST)
	edits := diffing.GetTreeDiff(beforeTree, currentTree)

	fmt.Println("Before Go AST:")
	printTree(beforeTree, "", true, true)

	fmt.Println("Current Go AST:")
	printTree(currentTree, "", true, true)

	fmt.Println("Go AST edit path:")
	for _, edit := range edits {
		switch edit.Type {
		case diffing.EditInsert:
			fmt.Printf("%s %s\n", edit.Type, edit.To.Label)
		case diffing.EditDelete:
			fmt.Printf("%s %s\n", edit.Type, edit.From.Label)
		case diffing.EditUpdate:
			fmt.Printf("%s %s -> %s\n", edit.Type, edit.From.Label, edit.To.Label)
		}
	}
}

func clonePreviousTestProgram() (string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cloneParent, err := os.MkdirTemp("", "test-program-clone-")
	if err != nil {
		return "", err
	}

	sourceRepository := filepath.Join(workingDirectory, "TestProgram")
	clonePath := filepath.Join(cloneParent, "TestProgram")
	cloneCommand := exec.Command("git", "clone", "--no-local", sourceRepository, clonePath)
	if output, err := cloneCommand.CombinedOutput(); err != nil {
		return "", fmt.Errorf("clone TestProgram: %w\n%s", err, output)
	}

	checkoutCommand := exec.Command("git", "checkout", "--detach", "HEAD~1")
	checkoutCommand.Dir = clonePath
	if output, err := checkoutCommand.CombinedOutput(); err != nil {
		return "", fmt.Errorf("checkout previous TestProgram commit: %w\n%s", err, output)
	}

	return clonePath, nil
}

func readTestProgramAt(revision string) ([]byte, error) {
	command := exec.Command("git", "show", revision+":main.go")
	command.Dir = "TestProgram"

	return command.Output()

}

func parseMonkeyProgram(input string) *ast.Program {
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		panic(p.Errors())
	}
	return program
}

func printTree(node *treeeditdistance.Node, prefix string, isLast, isRoot bool) {
	if node == nil {
		return
	}

	if isRoot {
		fmt.Println(node.Label)
	} else {
		connector := "|-- "
		if isLast {
			connector = "`-- "
		}
		fmt.Printf("%s%s%s\n", prefix, connector, node.Label)
	}

	for index, child := range node.Children {
		lastChild := index == len(node.Children)-1
		childPrefix := prefix
		if !isRoot {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "|   "
			}
		}
		printTree(child, childPrefix, lastChild, false)
	}
}
