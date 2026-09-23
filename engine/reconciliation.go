package engine

import "fmt"

func applyStructuralEdit(working, target *structuralASTNode, edit structuralEdit) error {
	return applyStructuralEditWithOptions(working, target, edit, ApplyOptions{Reconcile: true})
}

func applyStructuralEditWithOptions(working, target *structuralASTNode, edit structuralEdit, options ApplyOptions) error {
	if edit.Kind == "DELETE" {
		node := findWorkingEditNode(working, edit)
		if node == nil {
			return nil
		}
		removeStructuralNode(node)
		return nil
	}
	targetNode := findTargetEditNode(target, edit)
	if targetNode == nil {
		return fmt.Errorf("target node %s is missing", edit.NodeID)
	}
	if edit.Kind == "UPDATE" {
		if node := findStructuralNode(working, targetNode); node != nil {
			node.Value = targetNode.Value
			return nil
		}
		return fmt.Errorf("node %s is not present for update", edit.NodeID)
	}
	if existing := findWorkingEditNode(working, edit); existing != nil {
		if options.Reconcile && targetNode.parent != nil {
			attachStructuralNode(working, existing, targetNode, targetNode.parent)
			attachStructuralDescendants(working, target, targetNode)
		}
		return nil
	}
	parent := findStructuralNode(working, targetNode.parent)
	if parent == nil {
		parent = nearestStructuralContainer(working, targetNode)
	}
	if parent == nil {
		return fmt.Errorf("parent for %s is unavailable", edit.NodeID)
	}
	child := shallowStructuralNode(targetNode)
	insertIndex := targetNode.Index
	if targetNode.parent == nil || findStructuralNode(working, targetNode.parent) == nil {
		orphanLocation(child, targetNode, parent)
		insertIndex = len(parent.Children)
		child.Index = insertIndex
	} else {
		insertIndex = structuralInsertPosition(working, parent, targetNode.parent, targetNode)
	}
	insertStructuralNode(parent, child, insertIndex)
	seedStructuralSlots(working, child, targetNode)
	if options.Reconcile {
		attachStructuralDescendants(working, target, targetNode)
	}
	return nil
}

// seedStructuralSlots creates the empty shape required for a construct. The
// edit script still supplies the slot contents as independent edits.
func seedStructuralSlots(working, node, target *structuralASTNode) {
	if node == nil || target == nil {
		return
	}
	var fields []string
	switch node.Kind {
	case "*ast.FuncDecl", "*ast.FuncLit":
		fields = []string{"Type", "Body"}
	case "*ast.IfStmt":
		fields = []string{"Cond", "Body"}
	default:
		return
	}
	for _, field := range fields {
		if child := childForField(target, field); child != nil {
			if findGlobalOrPath(working, child.GlobalID, child.ID) != nil {
				continue
			}
			insertStructuralNode(node, shallowStructuralNode(child), len(node.Children))
		}
	}
}

func reconciliationCandidates(working, target *structuralASTNode, ancestorID string) []ReconciliationCandidate {
	ancestor := target.find(ancestorID)
	if ancestor == nil {
		return nil
	}
	candidates := make([]ReconciliationCandidate, 0)
	walkStructuralNodes(ancestor, func(targetNode *structuralASTNode) {
		if targetNode == ancestor {
			return
		}
		current := findGlobalOrPath(working, targetNode.GlobalID, targetNode.ID)
		if current == nil || targetNode.parent == nil || current.parent != nil && current.parent.ID == targetNode.parent.ID {
			return
		}
		candidate := ReconciliationCandidate{
			NodeID:                 targetNode.ID,
			NodeGlobalID:           targetNode.GlobalID,
			OriginalParentID:       targetNode.parent.ID,
			OriginalParentGlobalID: targetNode.OriginalParentGlobalID,
			CurrentParentID:        current.CurrentParentID,
			CurrentParentGlobalID:  current.CurrentParentGlobalID,
			OriginalPath:           targetNode.OriginalPath,
			CurrentPath:            current.CurrentPath,
			CanReconcile:           false,
		}
		if findStructuralNode(working, targetNode.parent) == nil {
			candidate.Reason = "intended parent is not present"
		} else {
			candidate.CanReconcile = true
		}
		candidates = append(candidates, candidate)
	})
	return candidates
}

func applyReconciliationCandidate(working, target *structuralASTNode, candidate ReconciliationCandidate) error {
	targetNode := findGlobalOrPath(target, candidate.NodeGlobalID, candidate.NodeID)
	if targetNode == nil {
		return fmt.Errorf("target node %s is missing", candidate.NodeID)
	}
	current := findGlobalOrPath(working, candidate.NodeGlobalID, candidate.NodeID)
	if current == nil {
		return fmt.Errorf("current node %s is missing", candidate.NodeID)
	}
	if targetNode.parent == nil {
		return nil
	}
	if findStructuralNode(working, targetNode.parent) == nil {
		return fmt.Errorf("intended parent for %s is not present", candidate.NodeID)
	}
	attachStructuralNode(working, current, targetNode, targetNode.parent)
	return nil
}

func normalizeAppliedStructure(working, target *structuralASTNode, edits []structuralEdit, statuses []EditStatus) {
	for pass := 0; pass < len(edits)+1; pass++ {
		changed := false
		for index, edit := range edits {
			if statuses[index] != EditApplied || edit.Kind == "DELETE" {
				continue
			}
			targetNode := findTargetEditNode(target, edit)
			current := findWorkingEditNode(working, edit)
			if targetNode == nil || current == nil || targetNode.parent == nil {
				continue
			}
			parent := findStructuralNode(working, targetNode.parent)
			if parent == nil {
				continue
			}
			oldParent := current.parent
			oldField, oldIndex := current.Field, current.Index
			attachStructuralNode(working, current, targetNode, targetNode.parent)
			if current.parent != oldParent || current.Field != oldField || current.Index != oldIndex {
				changed = true
			}
		}
		if !changed {
			return
		}
	}
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
		if child.Kind == target.Kind && child.Field == target.Field && child.Index == target.Index {
			return child
		}
	}
	return nil
}

func findStructuralNode(working, target *structuralASTNode) *structuralASTNode {
	if target == nil {
		return nil
	}
	if node := findGlobalOrPath(working, target.GlobalID, target.ID); node != nil {
		return node
	}
	return resolveStructuralNode(working, target)
}

func findGlobalOrPath(root *structuralASTNode, globalID, pathID string) *structuralASTNode {
	if root == nil {
		return nil
	}
	if globalID != "" {
		return root.findGlobal(globalID)
	}
	return root.find(pathID)
}

func findTargetEditNode(target *structuralASTNode, edit structuralEdit) *structuralASTNode {
	return findGlobalOrPath(target, edit.NodeGlobalID, edit.NodeID)
}

func findWorkingEditNode(working *structuralASTNode, edit structuralEdit) *structuralASTNode {
	if edit.SourceGlobalID != "" {
		if node := working.findGlobal(edit.SourceGlobalID); node != nil {
			return node
		}
	}
	return findGlobalOrPath(working, edit.NodeGlobalID, edit.NodeID)
}

func walkStructuralNodes(root *structuralASTNode, visit func(*structuralASTNode)) {
	if root == nil || visit == nil {
		return
	}
	visit(root)
	for _, child := range root.Children {
		walkStructuralNodes(child, visit)
	}
}

func nearestStructuralContainer(working, target *structuralASTNode) *structuralASTNode {
	for ancestor := target.parent; ancestor != nil; ancestor = ancestor.parent {
		if current := findStructuralNode(working, ancestor); current != nil {
			return current
		}
	}
	return nil
}

func orphanLocation(child, target, container *structuralASTNode) {
	ancestor := target
	for ancestor.parent != nil {
		if ancestor.parent.ID == container.ID {
			child.Field = ancestor.Field
			child.Index = ancestor.Index
			return
		}
		ancestor = ancestor.parent
	}
	child.Field = "child"
	child.Index = len(container.Children)
}

func attachStructuralNode(working, node, targetNode, targetParent *structuralASTNode) {
	parent := findStructuralNode(working, targetParent)
	if parent == nil || node == parent {
		return
	}
	if node.parent == parent && node.Field == targetNode.Field && node.Index == targetNode.Index {
		return
	}
	if node.parent != nil {
		removeStructuralNode(node)
	}
	node.Field = targetNode.Field
	node.Index = targetNode.Index
	insertStructuralNode(parent, node, structuralInsertPosition(working, parent, targetParent, targetNode))
}

func structuralInsertPosition(working, parent, targetParent, targetNode *structuralASTNode) int {
	if parent == nil {
		return 0
	}
	if targetParent == nil || targetNode == nil {
		return len(parent.Children)
	}
	position := 0
	for _, candidate := range targetParent.Children {
		if candidate == targetNode {
			return position
		}
		current := findGlobalOrPath(working, candidate.GlobalID, candidate.ID)
		if current == nil || current.parent != parent {
			continue
		}
		for index, child := range parent.Children {
			if child == current {
				position = index + 1
				break
			}
		}
	}
	return position
}

func attachStructuralDescendants(working, target, targetNode *structuralASTNode) {
	current := findStructuralNode(working, targetNode)
	if current == nil {
		return
	}
	for _, targetChild := range targetNode.Children {
		if child := findStructuralNode(working, targetChild); child != nil {
			attachStructuralNode(working, child, targetChild, targetNode)
			attachStructuralDescendants(working, target, targetChild)
		}
	}
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
	reindexStructuralChildren(parent)
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
			reindexStructuralChildren(parent)
			return
		}
	}
}

func reindexStructuralChildren(parent *structuralASTNode) {
	if parent == nil {
		return
	}
	fieldIndices := make(map[string]int)
	for _, child := range parent.Children {
		child.Index = fieldIndices[child.Field]
		fieldIndices[child.Field]++
		setStructuralOwnership(child, parent)
	}
}

func setStructuralOwnership(node, parent *structuralASTNode) {
	if node == nil {
		return
	}
	if parent == nil {
		node.CurrentParentID = ""
		node.CurrentParentGlobalID = ""
		node.CurrentPath = node.ID
		return
	}
	node.CurrentParentID = parent.ID
	node.CurrentParentGlobalID = parent.GlobalID
	node.CurrentField = node.Field
	node.CurrentIndex = node.Index
	node.CurrentPath = fmt.Sprintf("%s.%s[%d]", parent.CurrentPath, node.CurrentField, node.CurrentIndex)
}

func setStructuralOriginalOwnership(node, parent *structuralASTNode) {
	if node == nil || parent == nil {
		return
	}
	node.OriginalParentID = parent.ID
	node.OriginalParentGlobalID = parent.GlobalID
}

func restoreStructuralParents(node, parent *structuralASTNode) {
	if node == nil {
		return
	}
	node.parent = parent
	if parent != nil {
		setStructuralOwnership(node, parent)
	}
	for _, child := range node.Children {
		restoreStructuralParents(child, node)
	}
}
