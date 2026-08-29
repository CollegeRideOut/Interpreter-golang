package treeeditdistance

func DistanceForest(source []*Node, target []*Node) int {
	if len(source) == 0 && len(target) == 0 {
		return 0
	}

	if len(source) == 0 {
		// Insert every node in the target forest.
		return calculateSizeOfForest(target)
	}

	if len(target) == 0 {
		// Delete every node in the source forest.
		return calculateSizeOfForest(source)
	}

	deleteCost := 1 + DistanceForest(deleteRootTransformation(source), target)
	insertCost := 1 + DistanceForest(source, deleteRootTransformation(target))

	renameCost := 0

	if source[0].Label != target[0].Label {
		renameCost = 1
	}

	matchCost := renameCost +
		DistanceForest(source[0].Children, target[0].Children) +
		DistanceForest(source[1:], target[1:])

	return min(deleteCost, insertCost, matchCost)

}

func calculateSizeOfForest(forest []*Node) int {
	size := 0

	for _, root := range forest {
		size += calculateSizeOfNode(root)
	}

	return size
}

func calculateSizeOfNode(node *Node) int {
	if node == nil {
		return 0
	}

	size := 1 // Count the current node.

	for _, child := range node.Children {
		size += calculateSizeOfNode(child)
	}

	return size
}

func deleteRootTransformation(forest []*Node) []*Node {
	root := forest[0]

	var newForest []*Node

	rest := forest[1:]
	newForest = append(newForest, root.Children...)
	newForest = append(newForest, rest...)

	return newForest

}
