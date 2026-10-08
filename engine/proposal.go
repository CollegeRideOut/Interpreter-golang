package engine

import (
	"fmt"
	"strings"
)

// ProposalOperation is a portable structural operation derived from one
// proposal's base-to-target diff. It contains the target subtree needed to
// materialize the operation in the human build.
type ProposalOperation struct {
	Kind           string `json:"kind"`
	NodeID         string `json:"nodeId"`
	NodeGlobalID   string `json:"nodeGlobalId,omitempty"`
	NodeKind       string `json:"nodeKind"`
	ParentID       string `json:"parentId,omitempty"`
	ParentGlobalID string `json:"parentGlobalId,omitempty"`
	ParentKind     string `json:"parentKind,omitempty"`
	BeforeID       string `json:"beforeId,omitempty"`
	BeforeGlobalID string `json:"beforeGlobalId,omitempty"`
	BeforeKind     string `json:"beforeKind,omitempty"`
	BeforeValue    string `json:"beforeValue,omitempty"`
	Field          string `json:"field,omitempty"`
	Position       int    `json:"position"`
	Value          string `json:"value,omitempty"`
	Node           *Node  `json:"node,omitempty"`
}

// ProposalOperationsForSubtree extracts one user-facing edit and the syntax
// details required to materialize it. Insertions and deletions are atomic;
// updates retain changed descendants.
func (state *WorkingState) ProposalOperationsForSubtree(index int) ([]ProposalOperation, error) {
	if state == nil || index < 0 || index >= len(state.edits) {
		return nil, fmt.Errorf("proposal edit index %d is out of range", index)
	}
	selectedIndex := index
	selected := state.edits[selectedIndex]
	// Imports are atomic user-facing edits. Never transfer a descendant such as
	// the module string or `from` token as a standalone proposal operation.
	if selected.NodeKind != "typescript:import_statement" && selected.NodeKind != "tsx:import_statement" {
		for editIndex, candidate := range state.edits {
			if candidate.NodeKind != "typescript:import_statement" && candidate.NodeKind != "tsx:import_statement" {
				continue
			}
			for _, ancestorID := range selected.AncestorIDs {
				if ancestorID == candidate.NodeID {
					selectedIndex = editIndex
					selected = candidate
					break
				}
			}
			if selectedIndex != index {
				break
			}
		}
	}
	operations := make([]ProposalOperation, 0)
	if targetNode := state.target.find(selected.NodeID); targetNode != nil && targetNode.parent != nil && targetNode.parent.Kind == "typescript:named_imports" {
		if sourceParent := findProposalSourceNode(state.source, targetNode.parent); sourceParent != nil {
			return []ProposalOperation{{
				Kind:           "REPLACE_CHILDREN",
				NodeID:         sourceParent.ID,
				NodeGlobalID:   sourceParent.GlobalID,
				NodeKind:       sourceParent.Kind,
				ParentID:       sourceParent.parent.ID,
				ParentGlobalID: sourceParent.parent.GlobalID,
				Node:           targetNode.parent.clone(),
			}}, nil
		}
	}
	for editIndex, edit := range state.edits {
		include := editIndex == selectedIndex
		if !include && selected.Kind == "UPDATE" {
			for _, ancestorID := range edit.AncestorIDs {
				if ancestorID == selected.NodeID {
					include = true
					break
				}
			}
		}
		if !include && edit.Hidden && editParentIdentity(edit) == editParentIdentity(selected) {
			include = true
		}
		if !include {
			continue
		}
		node := state.target.find(edit.NodeID)
		if edit.Kind == "DELETE" {
			node = state.source.find(edit.NodeID)
		}
		if node == nil {
			return nil, fmt.Errorf("proposal node %s is missing", edit.NodeID)
		}
		parentID := edit.ParentID
		parentGlobalID := ""
		if node.parent != nil {
			if baseParent := findProposalSourceNode(state.source, node.parent); baseParent != nil {
				parentID = baseParent.ID
				parentGlobalID = baseParent.GlobalID
			}
		}
		beforeID := ""
		beforeGlobalID := ""
		beforeKind := ""
		beforeValue := ""
		if node.parent != nil {
			for _, sibling := range node.parent.Children[node.Index+1:] {
				baseParent := findProposalSourceNode(state.source, node.parent)
				if baseSibling := findProposalSourceSibling(baseParent, sibling); baseSibling != nil {
					beforeID = baseSibling.ID
					beforeGlobalID = baseSibling.GlobalID
					beforeKind = baseSibling.Kind
					beforeValue = baseSibling.Value
					break
				}
			}
		}
		nodeID := edit.NodeID
		if edit.Kind != "INSERT" {
			if baseNode := findProposalSourceNode(state.source, node); baseNode != nil {
				nodeID = baseNode.ID
			}
		}
		operations = append(operations, ProposalOperation{
			Kind:           edit.Kind,
			NodeID:         nodeID,
			NodeGlobalID:   node.GlobalID,
			NodeKind:       edit.NodeKind,
			ParentID:       parentID,
			ParentGlobalID: parentGlobalID,
			ParentKind:     parentKind(node.parent),
			BeforeID:       beforeID,
			BeforeGlobalID: beforeGlobalID,
			BeforeKind:     beforeKind,
			BeforeValue:    beforeValue,
			Field:          edit.Field,
			Position:       edit.Position,
			Value:          edit.Value,
			Node:           node.clone(),
		})
	}
	return operations, nil
}

func findProposalSourceNode(source, target *structuralASTNode) *structuralASTNode {
	if source == nil || target == nil {
		return nil
	}
	if target.GlobalID != "" {
		if node := source.findGlobal(target.GlobalID); node != nil {
			return node
		}
	}
	return source.find(target.ID)
}

func findProposalSourceSibling(parent, target *structuralASTNode) *structuralASTNode {
	if parent == nil || target == nil {
		return nil
	}
	// Punctuation nodes are repeated within a container, so a positional
	// insertion or the original structural path is safer than guessing by shape.
	if target.Kind == "typescript:," || target.Kind == "tsx:," {
		return parent.find(target.ID)
	}
	for _, candidate := range parent.Children {
		if candidate.ID == target.ID && candidate.Kind == target.Kind && candidate.Value == target.Value && proposalNodeShape(candidate) == proposalNodeShape(target) {
			return candidate
		}
	}
	for _, candidate := range parent.Children {
		if candidate.Kind == target.Kind && candidate.Value == target.Value && proposalNodeShape(candidate) == proposalNodeShape(target) {
			return candidate
		}
	}
	return nil
}

func proposalNodeShape(node *structuralASTNode) string {
	if node == nil {
		return ""
	}
	parts := []string{node.Kind, node.Value, node.Field}
	for _, child := range node.Children {
		parts = append(parts, child.Kind, child.Value, child.Field)
	}
	return strings.Join(parts, "\x00")
}

// ApplyProposalOperations applies portable operations to this working state.
// The operations are resolved against the shared base's structural paths, not
// against proposal-specific UUIDs.
func (state *WorkingState) ApplyProposalOperations(operations []ProposalOperation) error {
	if state == nil {
		return fmt.Errorf("working state is nil")
	}
	for _, operation := range operations {
		switch operation.Kind {
		case "REPLACE_CHILDREN":
			parent := findProposalNode(state.working, operation.NodeGlobalID, operation.NodeID)
			if parent == nil || operation.Node == nil {
				return fmt.Errorf("proposal container %s is missing", operation.NodeID)
			}
			parent.Children = make([]*structuralASTNode, len(operation.Node.Children))
			for index, child := range operation.Node.Children {
				parent.Children[index] = child.clone()
				parent.Children[index].parent = parent
			}
			reindexStructuralChildren(parent)
		case "INSERT":
			if existing := findProposalNode(state.working, operation.NodeGlobalID, operation.NodeID); existing != nil && existing.Kind == operation.NodeKind {
				continue
			}
			parent := findProposalNode(state.working, operation.ParentGlobalID, operation.ParentID)
			if parent == nil {
				return fmt.Errorf("proposal parent %s is missing", operation.ParentID)
			}
			if operation.Node == nil {
				return fmt.Errorf("proposal insert %s has no node", operation.NodeID)
			}
			insertPosition := operation.Position
			if operation.BeforeID != "" || operation.BeforeGlobalID != "" {
				for childIndex, child := range parent.Children {
					matchesAnchor := operation.BeforeGlobalID != "" && child.GlobalID == operation.BeforeGlobalID || operation.BeforeGlobalID == "" && child.ID == operation.BeforeID
					if matchesAnchor && (operation.BeforeKind == "" || child.Kind == operation.BeforeKind && child.Value == operation.BeforeValue) {
						insertPosition = childIndex
						break
					}
				}
				if insertPosition == operation.Position || operation.BeforeKind != "" {
					for childIndex, child := range parent.Children {
						if child.Kind == operation.BeforeKind && child.Value == operation.BeforeValue {
							insertPosition = childIndex
							break
						}
					}
				}
			}
			insertStructuralNode(parent, operation.Node.clone(), insertPosition)
		case "UPDATE":
			node := findProposalNode(state.working, operation.NodeGlobalID, operation.NodeID)
			if node == nil {
				return fmt.Errorf("proposal update node %s is missing", operation.NodeID)
			}
			node.Value = operation.Value
		case "DELETE":
			if node := findProposalNode(state.working, operation.NodeGlobalID, operation.NodeID); node != nil {
				removeStructuralNode(node)
			}
		default:
			return fmt.Errorf("unsupported proposal operation %q", operation.Kind)
		}
	}
	return nil
}

func findProposalNode(root *structuralASTNode, globalID, nodeID string) *structuralASTNode {
	if globalID != "" {
		if node := root.findGlobal(globalID); node != nil {
			return node
		}
	}
	return root.find(nodeID)
}
