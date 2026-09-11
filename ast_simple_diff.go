package main

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"reflect"
)

type structuralASTNode struct {
	ID       string               `json:"id"`
	Kind     string               `json:"kind"`
	Value    string               `json:"value,omitempty"`
	Field    string               `json:"field,omitempty"`
	Index    int                  `json:"index,omitempty"`
	Children []*structuralASTNode `json:"children,omitempty"`
	parent   *structuralASTNode
}

type structuralEdit struct {
	Index      int                `json:"index"`
	Kind       string             `json:"kind"`
	NodeID     string             `json:"nodeId"`
	NodeKind   string             `json:"nodeKind"`
	ParentID   string             `json:"parentId,omitempty"`
	ParentKind string             `json:"parentKind,omitempty"`
	Field      string             `json:"field,omitempty"`
	Position   int                `json:"position"`
	Value      string             `json:"value,omitempty"`
	Node       *structuralASTNode `json:"node"`
}

func structuralASTTree(node goast.Node, fileSet *token.FileSet) *structuralASTNode {
	return structuralASTNodeFromGo(node, fileSet, "root", "", 0)
}

func structuralASTNodeFromGo(node goast.Node, fileSet *token.FileSet, id, field string, index int) *structuralASTNode {
	if node == nil {
		return nil
	}
	result := &structuralASTNode{
		ID:    id,
		Kind:  fmt.Sprintf("%T", node),
		Value: goNodeValue(node),
		Field: field,
		Index: index,
	}
	value := reflect.ValueOf(node).Elem()
	astNodeType := reflect.TypeOf((*goast.Node)(nil)).Elem()
	for fieldIndex := 0; fieldIndex < value.NumField(); fieldIndex++ {
		fieldInfo := value.Type().Field(fieldIndex)
		fieldValue := value.Field(fieldIndex)
		if !fieldInfo.IsExported() || fieldInfo.Name == "Obj" || fieldInfo.Name == "Scope" || fieldInfo.Name == "Unresolved" || fieldInfo.Name == "Imports" {
			continue
		}
		if fieldValue.Type().Implements(astNodeType) {
			if fieldValue.IsNil() {
				continue
			}
			child := fieldValue.Interface().(goast.Node)
			childNode := structuralASTNodeFromGo(child, fileSet, id+"."+fieldInfo.Name, fieldInfo.Name, 0)
			childNode.parent = result
			result.Children = append(result.Children, childNode)
			continue
		}
		if fieldValue.Kind() != reflect.Slice || !fieldValue.Type().Elem().Implements(astNodeType) {
			continue
		}
		for childIndex := 0; childIndex < fieldValue.Len(); childIndex++ {
			child := fieldValue.Index(childIndex).Interface().(goast.Node)
			childNode := structuralASTNodeFromGo(child, fileSet, fmt.Sprintf("%s.%s[%d]", id, fieldInfo.Name, childIndex), fieldInfo.Name, childIndex)
			childNode.parent = result
			result.Children = append(result.Children, childNode)
		}
	}
	return result
}

func shallowStructuralNode(node *structuralASTNode) *structuralASTNode {
	if node == nil {
		return nil
	}
	clone := *node
	clone.Children = nil
	clone.parent = nil
	return &clone
}

func (node *structuralASTNode) find(id string) *structuralASTNode {
	if node == nil {
		return nil
	}
	if node.ID == id {
		return node
	}
	for _, child := range node.Children {
		if found := child.find(id); found != nil {
			return found
		}
	}
	return nil
}

func (node *structuralASTNode) clone() *structuralASTNode {
	if node == nil {
		return nil
	}
	clone := shallowStructuralNode(node)
	for _, child := range node.Children {
		childClone := child.clone()
		childClone.parent = clone
		clone.Children = append(clone.Children, childClone)
	}
	return clone
}

func simpleASTEditScripts(source, target goast.Node, sourceFileSet, targetFileSet *token.FileSet) []structuralEdit {
	sourceTree := structuralASTTree(source, sourceFileSet)
	targetTree := structuralASTTree(target, targetFileSet)
	edits := make([]structuralEdit, 0)
	compareStructuralNodes(sourceTree, targetTree, nil, &edits)
	for index := range edits {
		edits[index].Index = index
	}
	return edits
}

func compareStructuralNodes(source, target *structuralASTNode, parent *structuralASTNode, edits *[]structuralEdit) {
	if source == nil && target == nil {
		return
	}
	if source == nil {
		emitStructuralSubtree("INSERT", target, parent, edits)
		return
	}
	if target == nil {
		emitStructuralSubtree("DELETE", source, parent, edits)
		return
	}
	if source.Kind != target.Kind {
		emitStructuralSubtree("DELETE", source, parent, edits)
		emitStructuralSubtree("INSERT", target, parent, edits)
		return
	}
	if source.Value != target.Value {
		*edits = append(*edits, structuralEdit{
			Kind:       "UPDATE",
			NodeID:     target.ID,
			NodeKind:   target.Kind,
			ParentID:   parentID(parent),
			ParentKind: parentKind(parent),
			Field:      target.Field,
			Position:   target.Index,
			Value:      target.Value,
			Node:       shallowStructuralNode(target),
		})
	}

	compareStructuralChildren(source, target, edits)
}

func compareStructuralChildren(source, target *structuralASTNode, edits *[]structuralEdit) {
	sourceByField := make(map[string][]*structuralASTNode)
	targetByField := make(map[string][]*structuralASTNode)
	fieldOrder := make([]string, 0)
	seenFields := make(map[string]bool)
	for _, child := range source.Children {
		sourceByField[child.Field] = append(sourceByField[child.Field], child)
	}
	for _, child := range target.Children {
		targetByField[child.Field] = append(targetByField[child.Field], child)
		if !seenFields[child.Field] {
			fieldOrder = append(fieldOrder, child.Field)
			seenFields[child.Field] = true
		}
	}
	for _, child := range source.Children {
		if !seenFields[child.Field] {
			fieldOrder = append(fieldOrder, child.Field)
			seenFields[child.Field] = true
		}
	}

	for _, field := range fieldOrder {
		sourceChildren := sourceByField[field]
		targetChildren := targetByField[field]
		usedSource := make([]bool, len(sourceChildren))
		for _, targetChild := range targetChildren {
			match := -1
			for sourceIndex, sourceChild := range sourceChildren {
				if !usedSource[sourceIndex] && sourceChild.Kind == targetChild.Kind {
					match = sourceIndex
					break
				}
			}
			if match < 0 {
				emitStructuralSubtree("INSERT", targetChild, target, edits)
				continue
			}
			usedSource[match] = true
			compareStructuralNodes(sourceChildren[match], targetChild, target, edits)
		}
		for sourceIndex, sourceChild := range sourceChildren {
			if !usedSource[sourceIndex] {
				compareStructuralNodes(sourceChild, nil, source, edits)
			}
		}
	}
}

func emitStructuralSubtree(kind string, node *structuralASTNode, parent *structuralASTNode, edits *[]structuralEdit) {
	if node == nil {
		return
	}
	*edits = append(*edits, structuralEdit{
		Kind:       kind,
		NodeID:     node.ID,
		NodeKind:   node.Kind,
		ParentID:   parentID(parent),
		ParentKind: parentKind(parent),
		Field:      node.Field,
		Position:   node.Index,
		Value:      node.Value,
		Node:       shallowStructuralNode(node),
	})
	for _, child := range node.Children {
		emitStructuralSubtree(kind, child, node, edits)
	}
}

func parentID(node *structuralASTNode) string {
	if node == nil {
		return ""
	}
	return node.ID
}

func parentKind(node *structuralASTNode) string {
	if node == nil {
		return ""
	}
	return node.Kind
}
