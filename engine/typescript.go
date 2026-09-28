package engine

import (
	"fmt"
	"unsafe"

	"github.com/google/uuid"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// ParseTypeScript converts TypeScript source into the same editable structural
// tree used by the Go adapter. Tree-sitter keeps this adapter syntax-focused;
// type resolution can be added independently later.
func ParseTypeScript(source []byte) (*Node, error) {
	return parseTreeSitter(source, "typescript", typescript.LanguageTypescript())
}

func parseTreeSitter(source []byte, language string, languagePointer unsafe.Pointer) (*Node, error) {
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(languagePointer)); err != nil {
		return nil, err
	}
	tree := parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("TypeScript parser returned no tree")
	}
	root := structuralASTNodeFromTypeScript(tree.RootNode(), source, "root", "", 0, language)
	if tree.RootNode().HasError() {
		return nil, fmt.Errorf("invalid %s source", language)
	}
	return root, nil
}

// DiffTypeScript returns the TypeScript trees and structural edit script.
func DiffTypeScript(source, target []byte) (sourceTree, targetTree *Node, edits []Edit, err error) {
	sourceTree, err = ParseTypeScript(source)
	if err != nil {
		return nil, nil, nil, err
	}
	targetTree, err = ParseTypeScript(target)
	if err != nil {
		return nil, nil, nil, err
	}
	edits = simpleStructuralEditScripts(sourceTree, targetTree)
	bindStructuralEditIdentities(sourceTree, targetTree, edits)
	return sourceTree, targetTree, edits, nil
}

// NewWorkingStateFromTypeScript creates an editable TypeScript session.
func NewWorkingStateFromTypeScript(source, target []byte) (*WorkingState, error) {
	sourceTree, targetTree, edits, err := DiffTypeScript(source, target)
	if err != nil {
		return nil, err
	}
	return NewWorkingState(sourceTree, targetTree, edits)
}

func structuralASTNodeFromTypeScript(node *tree_sitter.Node, source []byte, id, field string, index int, language string) *structuralASTNode {
	if node == nil {
		return nil
	}
	result := &structuralASTNode{
		ID:            id,
		GlobalID:      uuid.NewString(),
		OriginalPath:  id,
		CurrentPath:   id,
		Kind:          language + ":" + node.Kind(),
		Field:         field,
		Index:         index,
		OriginalField: field,
		CurrentField:  field,
		OriginalIndex: index,
		CurrentIndex:  index,
		StartLine:     int(node.StartPosition().Row) + 1,
		EndLine:       int(node.EndPosition().Row) + 1,
		StartByte:     uint(node.StartByte()),
		EndByte:       uint(node.EndByte()),
		Language:      language,
		source:        source,
	}
	if node.ChildCount() == 0 && result.StartByte <= result.EndByte && result.EndByte <= uint(len(source)) {
		result.Value = string(source[result.StartByte:result.EndByte])
	}
	for childIndex := uint(0); childIndex < node.ChildCount(); childIndex++ {
		child := node.Child(childIndex)
		childField := node.FieldNameForChild(uint32(childIndex))
		childNode := structuralASTNodeFromTypeScript(child, source, fmt.Sprintf("%s.child[%d]", id, childIndex), childField, int(childIndex), language)
		if childNode == nil {
			continue
		}
		childNode.parent = result
		setStructuralOriginalOwnership(childNode, result)
		setStructuralOwnership(childNode, result)
		result.Children = append(result.Children, childNode)
	}
	return result
}
