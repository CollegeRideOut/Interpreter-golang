package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func runInteractive(directory string) error {
	diff, err := prepareDirectoryDiff(directory)
	if err != nil {
		return err
	}
	sourceAST, sourceFileSet, err := parseGoAST(diff.sourceSource)
	if err != nil {
		return err
	}
	targetAST, targetFileSet, err := parseGoAST(diff.targetSource)
	if err != nil {
		return err
	}
	edits := simpleASTEditScripts(sourceAST, targetAST, sourceFileSet, targetFileSet)
	working := structuralASTTree(sourceAST, sourceFileSet)
	target := structuralASTTree(targetAST, targetFileSet)
	applied := make([]bool, len(edits))

	printInteractiveEdits(edits, applied)
	printInteractiveAST(working)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("edit number, q to quit: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "q" || input == "quit" {
			break
		}
		index, err := strconv.Atoi(input)
		if err != nil || index < 1 || index > len(edits) {
			fmt.Println("enter a listed edit number or q")
			continue
		}
		editIndex := index - 1
		if applied[editIndex] {
			fmt.Println("edit already applied")
			continue
		}
		if err := applyStructuralEdit(working, target, edits[editIndex]); err != nil {
			fmt.Printf("cannot apply edit %d: %v\n", index, err)
			continue
		}
		applied[editIndex] = true
		if err := writeJSON("intermediateAst.json", working); err != nil {
			return err
		}
		fmt.Printf("applied edit %d\n", index)
		printInteractiveAST(working)
	}
	return scanner.Err()
}

func printInteractiveEdits(edits []structuralEdit, applied []bool) {
	children := make(map[string][]int)
	knownNodes := make(map[string]bool)
	for index, edit := range edits {
		children[edit.ParentID] = append(children[edit.ParentID], index)
		knownNodes[edit.NodeID] = true
	}
	var printEdit func(int, int)
	printEdit = func(index, depth int) {
		edit := edits[index]
		status := " "
		if applied[index] {
			status = "x"
		}
		fmt.Printf("%s[%s] %d %s at %s[%d]\n", strings.Repeat("  ", depth), status, index+1, edit.Kind+" "+edit.NodeKind, edit.Field, edit.Position)
		for _, childIndex := range children[edit.NodeID] {
			printEdit(childIndex, depth+1)
		}
	}
	fmt.Println("AST edits:")
	for index, edit := range edits {
		if edit.ParentID == "" || !knownNodes[edit.ParentID] {
			printEdit(index, 0)
		}
	}
}

func printInteractiveAST(root *structuralASTNode) {
	if err := writeJSON("intermediateAst.json", root); err != nil {
		fmt.Printf("cannot write intermediateAst.json: %v\n", err)
		return
	}
	fmt.Println("current intermediate AST written to intermediateAst.json")
}

func applyStructuralEdit(working, target *structuralASTNode, edit structuralEdit) error {
	if edit.Kind == "DELETE" {
		node := working.find(edit.NodeID)
		if node == nil {
			return fmt.Errorf("node %s is not present", edit.NodeID)
		}
		removeStructuralNode(node)
		return nil
	}
	targetNode := target.find(edit.NodeID)
	if targetNode == nil {
		return fmt.Errorf("target node %s is missing", edit.NodeID)
	}
	if edit.Kind == "UPDATE" {
		if node := resolveStructuralNode(working, targetNode); node != nil {
			node.Value = targetNode.Value
			return nil
		}
		return fmt.Errorf("node %s is not present for update", edit.NodeID)
	}
	if resolveStructuralNode(working, targetNode) != nil {
		return nil
	}
	parent := ensureStructuralParent(working, target, targetNode.parent)
	if parent == nil {
		return fmt.Errorf("parent for %s is unavailable", edit.NodeID)
	}
	child := shallowStructuralNode(targetNode)
	insertStructuralNode(parent, child, targetNode.Index)
	return nil
}

func resolveStructuralNode(working, target *structuralASTNode) *structuralASTNode {
	if target == nil {
		return nil
	}
	if target.ID == "root" {
		return working
	}
	parent := resolveStructuralNode(working, target.parent)
	if parent == nil {
		return nil
	}
	for _, child := range parent.Children {
		if child.Kind == target.Kind && child.Field == target.Field {
			return child
		}
	}
	return nil
}

func ensureStructuralParent(working, target, parent *structuralASTNode) *structuralASTNode {
	if parent == nil {
		return working
	}
	if resolved := resolveStructuralNode(working, parent); resolved != nil {
		return resolved
	}
	grandparent := ensureStructuralParent(working, target, parent.parent)
	if grandparent == nil {
		return nil
	}
	created := shallowStructuralNode(parent)
	insertStructuralNode(grandparent, created, parent.Index)
	return created
}

func insertStructuralNode(parent, child *structuralASTNode, index int) {
	child.parent = parent
	position := len(parent.Children)
	if index >= 0 && index < len(parent.Children) {
		position = index
	}
	parent.Children = append(parent.Children, nil)
	copy(parent.Children[position+1:], parent.Children[position:])
	parent.Children[position] = child
}

func removeStructuralNode(node *structuralASTNode) {
	if node == nil || node.parent == nil {
		return
	}
	parent := node.parent
	for index, child := range parent.Children {
		if child == node {
			parent.Children = append(parent.Children[:index], parent.Children[index+1:]...)
			node.parent = nil
			return
		}
	}
}
