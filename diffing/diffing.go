package diffing

import treeeditdistance "interpreter/treeEditDistance"

type EditType string

const (
	EditInsert EditType = "insert"
	EditDelete EditType = "delete"
	EditUpdate EditType = "update"
)

type Edit struct {
	Type   EditType
	From   *treeeditdistance.Node
	To     *treeeditdistance.Node
	Parent *treeeditdistance.Node
	Index  int
}

// GetTreeDiff returns the edit path that transforms source into target.
func GetTreeDiff(source, target *treeeditdistance.Node) []Edit {
	sourceForest := []*treeeditdistance.Node{}
	targetForest := []*treeeditdistance.Node{}
	if source != nil {
		sourceForest = append(sourceForest, source)
	}
	if target != nil {
		targetForest = append(targetForest, target)
	}

	assignPostorderIDs(append(sourceForest, targetForest...))
	return diffForest(
		sourceForest,
		targetForest,
		make(map[treeeditdistance.ForestKey]int),
		nil,
		nil,
	)
}

func diffForest(
	source []*treeeditdistance.Node,
	target []*treeeditdistance.Node,
	cache map[treeeditdistance.ForestKey]int,
	sourceParent, targetParent *treeeditdistance.Node,
) []Edit {
	if len(source) == 0 && len(target) == 0 {
		return nil
	}

	if len(source) == 0 {
		edits := []Edit{}
		for _, node := range target {
			edits = appendInsertEdits(edits, node, sourceParent, targetParent)
		}
		return edits
	}

	if len(target) == 0 {
		edits := []Edit{}
		for _, node := range source {
			edits = appendDeleteEdits(edits, node, sourceParent)
		}
		return edits
	}

	current := treeeditdistance.DistanceForestC(source, target, cache)

	deleteCost := 1 + treeeditdistance.DistanceForestC(
		treeeditdistance.DeleteRootTransformation(source),
		target,
		cache,
	)

	insertCost := 1 + treeeditdistance.DistanceForestC(
		source,
		treeeditdistance.DeleteRootTransformation(target),
		cache,
	)

	renameCost := 0
	if source[0].Label != target[0].Label {
		renameCost = 1
	}

	matchCost := renameCost +
		treeeditdistance.DistanceForestC(source[0].Children, target[0].Children, cache) +
		treeeditdistance.DistanceForestC(source[1:], target[1:], cache)

	if current == matchCost {
		edits := []Edit{}

		if renameCost == 1 {
			edits = append(edits, Edit{
				Type:   EditUpdate,
				From:   source[0],
				To:     target[0],
				Parent: sourceParent,
			})
		}

		edits = append(
			edits,
			diffForest(source[0].Children, target[0].Children, cache, source[0], target[0])...,
		)

		edits = append(
			edits,
			diffForest(source[1:], target[1:], cache, sourceParent, targetParent)...,
		)

		return edits
	}

	if current == deleteCost {
		return append(
			[]Edit{{
				Type:   EditDelete,
				From:   source[0],
				Parent: sourceParent,
			}},
			diffForest(treeeditdistance.DeleteRootTransformation(source), target, cache, sourceParent, targetParent)...,
		)
	}

	if current == insertCost {
		return append(
			[]Edit{{
				Type:   EditInsert,
				To:     target[0],
				Parent: sourceParent,
				Index:  childIndex(targetParent, target[0]),
			}},
			diffForest(source, treeeditdistance.DeleteRootTransformation(target), cache, sourceParent, targetParent)...,
		)
	}

	// Otherwise we matched the roots.
	edits := []Edit{}

	if renameCost == 1 {
		edits = append(edits, Edit{
			Type:   EditUpdate,
			From:   source[0],
			To:     target[0],
			Parent: sourceParent,
		})
	}

	edits = append(
		edits,
		diffForest(source[0].Children, target[0].Children, cache, source[0], target[0])...,
	)

	edits = append(
		edits,
		diffForest(source[1:], target[1:], cache, sourceParent, targetParent)...,
	)

	return edits
}

func assignPostorderIDs(forest []*treeeditdistance.Node) {
	nextID := 1

	var walk func(*treeeditdistance.Node)
	walk = func(node *treeeditdistance.Node) {
		if node == nil {
			return
		}

		for _, child := range node.Children {
			walk(child)
		}

		node.ID = nextID
		nextID++
	}

	for _, root := range forest {
		walk(root)
	}
}

func appendInsertEdits(
	edits []Edit,
	node, sourceParent, targetParent *treeeditdistance.Node,
) []Edit {
	if node == nil {
		return edits
	}

	edits = append(edits, Edit{
		Type:   EditInsert,
		To:     node,
		Parent: sourceParent,
		Index:  childIndex(targetParent, node),
	})
	for _, child := range node.Children {
		edits = appendInsertEdits(edits, child, node, node)
	}

	return edits
}

func appendDeleteEdits(edits []Edit, node, parent *treeeditdistance.Node) []Edit {
	if node == nil {
		return edits
	}

	for _, child := range node.Children {
		edits = appendDeleteEdits(edits, child, node)
	}
	edits = append(edits, Edit{Type: EditDelete, From: node, Parent: parent})

	return edits
}

func childIndex(parent, child *treeeditdistance.Node) int {
	if parent == nil {
		return 0
	}

	for index, candidate := range parent.Children {
		if candidate == child {
			return index
		}
	}

	return len(parent.Children)
}

// ApplyEdit applies one edit to a forest and returns the updated forest.
func ApplyEdit(forest []*treeeditdistance.Node, edit Edit) []*treeeditdistance.Node {
	switch edit.Type {
	case EditUpdate:
		if edit.From != nil && edit.To != nil {
			edit.From.Label = edit.To.Label
		}
	case EditInsert:
		if edit.To != nil {
			inserted := &treeeditdistance.Node{ID: edit.To.ID, Label: edit.To.Label}
			parent := findNodeByID(forest, edit.Parent)
			if parent == nil {
				index := edit.Index
				if index < 0 || index > len(forest) {
					index = len(forest)
				}
				forest = append(forest, nil)
				copy(forest[index+1:], forest[index:])
				forest[index] = inserted
			} else {
				index := edit.Index
				if index < 0 || index > len(parent.Children) {
					index = len(parent.Children)
				}
				parent.Children = append(parent.Children, nil)
				copy(parent.Children[index+1:], parent.Children[index:])
				parent.Children[index] = inserted
			}
		}
	case EditDelete:
		forest = deleteNode(forest, edit.From)
	}

	return forest
}

func findNodeByID(forest []*treeeditdistance.Node, target *treeeditdistance.Node) *treeeditdistance.Node {
	if target == nil {
		return nil
	}

	var find func(*treeeditdistance.Node) *treeeditdistance.Node
	find = func(node *treeeditdistance.Node) *treeeditdistance.Node {
		if node == nil {
			return nil
		}
		if node.ID == target.ID {
			return node
		}
		for _, child := range node.Children {
			if found := find(child); found != nil {
				return found
			}
		}
		return nil
	}

	for _, root := range forest {
		if found := find(root); found != nil {
			return found
		}
	}

	return nil
}

func deleteNode(forest []*treeeditdistance.Node, target *treeeditdistance.Node) []*treeeditdistance.Node {
	if target == nil {
		return forest
	}

	for index, root := range forest {
		if root == target || root.ID == target.ID {
			return replaceNodeInForest(forest, index, root.Children)
		}
		if deleteChild(root, target) {
			return forest
		}
	}

	return forest
}

func deleteChild(parent, target *treeeditdistance.Node) bool {
	for index, child := range parent.Children {
		if child == target || child.ID == target.ID {
			parent.Children = replaceNodeInForest(parent.Children, index, child.Children)
			return true
		}
		if deleteChild(child, target) {
			return true
		}
	}

	return false
}

func replaceNodeInForest(
	forest []*treeeditdistance.Node,
	index int,
	replacement []*treeeditdistance.Node,
) []*treeeditdistance.Node {
	result := make([]*treeeditdistance.Node, 0, len(forest)-1+len(replacement))
	result = append(result, forest[:index]...)
	result = append(result, replacement...)
	result = append(result, forest[index+1:]...)
	return result
}
