package treeeditdistance

import (
	"sort"
	"strconv"
	"strings"
)

func DistanceForest(source []*Node, target []*Node) int {
	assignIDs(source)
	assignIDs(target)

	return DistanceForestC(source, target, make(map[ForestKey]int))
}

func DistanceBottomUp(source []*Node, target []*Node) int {
	assignIDs(source)
	assignIDs(target)

	sourcePostorder := postorderNodes(source)
	targetPostorder := postorderNodes(target)

	// An empty forest can only be transformed by inserting or deleting
	// every node in the other forest.
	if len(sourcePostorder) == 0 {
		return len(targetPostorder)
	}
	if len(targetPostorder) == 0 {
		return len(sourcePostorder)
	}

	sourceLeftmost := leftmostDescendants(source)
	targetLeftmost := leftmostDescendants(target)
	sourceKeyroots := keyroots(sourcePostorder, sourceLeftmost)
	targetKeyroots := keyroots(targetPostorder, targetLeftmost)

	// forestDistance is the working table for partial forests. treeDistance
	// stores the final answer for complete subtrees so a subtree can later be
	// matched as one unit inside a larger forest.
	forestDistance := initializeTreeDistanceTable(len(sourcePostorder), len(targetPostorder))
	treeDistance := initializeTreeDistanceTable(len(sourcePostorder), len(targetPostorder))

	// Each keyroot pair gives us one rectangular forest subproblem. The
	// regions share the same tables, so previously completed subtree values
	// remain available to later regions.
	for _, sourceKeyroot := range sourceKeyroots {
		for _, targetKeyroot := range targetKeyroots {
			initializeForestRegion(
				forestDistance,
				sourceKeyroot,
				targetKeyroot,
				sourceLeftmost,
				targetLeftmost,
			)

			sourceStart := sourceLeftmost[sourceKeyroot]
			targetStart := targetLeftmost[targetKeyroot]

			for sourceIndex := sourceStart; sourceIndex <= sourceKeyroot; sourceIndex++ {
				for targetIndex := targetStart; targetIndex <= targetKeyroot; targetIndex++ {
					sourceNode := sourcePostorder[sourceIndex-1]
					targetNode := targetPostorder[targetIndex-1]

					deleteCost := forestDistance[sourceIndex-1][targetIndex] + 1
					insertCost := forestDistance[sourceIndex][targetIndex-1] + 1

					// If both nodes are the roots of the current forests, the
					// diagonal operation compares the two node labels directly.
					if sourceLeftmost[sourceNode.ID] == sourceStart &&
						targetLeftmost[targetNode.ID] == targetStart {
						renameCost := forestDistance[sourceIndex-1][targetIndex-1] +
							renameCost(sourceNode, targetNode)

						forestDistance[sourceIndex][targetIndex] = min(
							deleteCost,
							insertCost,
							renameCost,
						)
						treeDistance[sourceIndex][targetIndex] = forestDistance[sourceIndex][targetIndex]
					} else {
						// Otherwise the diagonal operation consumes a complete
						// subtree whose answer was calculated earlier.
						matchSubtreeCost := forestDistance[sourceLeftmost[sourceNode.ID]-1][targetLeftmost[targetNode.ID]-1] +
							treeDistance[sourceIndex][targetIndex]

						forestDistance[sourceIndex][targetIndex] = min(
							deleteCost,
							insertCost,
							matchSubtreeCost,
						)
					}
				}
			}
		}
	}

	return forestDistance[len(sourcePostorder)][len(targetPostorder)]
}

// postorderNodes returns every node after its children, from left to right.
func postorderNodes(forest []*Node) []*Node {
	var nodes []*Node

	var walk func(*Node)
	walk = func(node *Node) {
		if node == nil {
			return
		}

		for _, child := range node.Children {
			walk(child)
		}

		nodes = append(nodes, node)
	}

	for _, root := range forest {
		walk(root)
	}

	return nodes
}

// leftmostDescendants returns each node's leftmost descendant postorder ID.
func leftmostDescendants(forest []*Node) map[int]int {
	leftmost := make(map[int]int)

	var walk func(*Node)
	walk = func(node *Node) {
		if node == nil {
			return
		}

		leftmostNode := node
		for {
			var firstChild *Node
			for _, child := range leftmostNode.Children {
				if child != nil {
					firstChild = child
					break
				}
			}

			if firstChild == nil {
				break
			}

			leftmostNode = firstChild
		}

		leftmost[node.ID] = leftmostNode.ID

		for _, child := range node.Children {
			walk(child)
		}
	}

	for _, root := range forest {
		walk(root)
	}

	return leftmost
}

// keyroots returns the last node for each distinct leftmost-descendant ID.
func keyroots(nodes []*Node, leftmost map[int]int) []int {
	lastNodeForLeftmost := make(map[int]int)

	for _, node := range nodes {
		lastNodeForLeftmost[leftmost[node.ID]] = node.ID
	}

	keys := make([]int, 0, len(lastNodeForLeftmost))
	for _, nodeID := range lastNodeForLeftmost {
		keys = append(keys, nodeID)
	}
	sort.Ints(keys)

	return keys
}

// initializeForestRegion sets the origin and borders for one keyroot pair.
func initializeForestRegion(
	table [][]int,
	sourceKeyroot, targetKeyroot int,
	sourceLeftmost, targetLeftmost map[int]int,
) {
	sourceStart := sourceLeftmost[sourceKeyroot] - 1
	targetStart := targetLeftmost[targetKeyroot] - 1
	table[sourceStart][targetStart] = 0

	for sourceIndex := sourceStart + 1; sourceIndex <= sourceKeyroot; sourceIndex++ {
		table[sourceIndex][targetStart] = sourceIndex - sourceStart
	}

	for targetIndex := targetStart + 1; targetIndex <= targetKeyroot; targetIndex++ {
		table[sourceStart][targetIndex] = targetIndex - targetStart
	}
}

// initializeForestDistanceTable creates the table and its empty-forest bases.
func initializeForestDistanceTable(sourceSize, targetSize int) [][]int {
	table := initializeTreeDistanceTable(sourceSize, targetSize)
	for sourceIndex := range table {
		table[sourceIndex][0] = sourceIndex
	}

	for targetIndex := 0; targetIndex <= targetSize; targetIndex++ {
		table[0][targetIndex] = targetIndex
	}

	return table
}

func initializeTreeDistanceTable(sourceSize, targetSize int) [][]int {
	table := make([][]int, sourceSize+1)
	for sourceIndex := range table {
		table[sourceIndex] = make([]int, targetSize+1)
	}

	return table
}

func renameCost(source, target *Node) int {
	if source.Label == target.Label {
		return 0
	}

	return 1
}

type ForestKey struct {
	source string
	target string
}

func DistanceForestC(source []*Node, target []*Node, cache map[ForestKey]int) int {
	key := ForestKey{
		source: forestIDs(source),
		target: forestIDs(target),
	}

	if distance, ok := cache[key]; ok {
		return distance
	}

	distance := distanceForestUncached(source, target, cache)
	cache[key] = distance
	return distance
}

func distanceForestUncached(source []*Node, target []*Node, cache map[ForestKey]int) int {
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

	deleteCost := 1 + DistanceForestC(DeleteRootTransformation(source), target, cache)
	insertCost := 1 + DistanceForestC(source, DeleteRootTransformation(target), cache)

	renameCost := 0

	if source[0].Label != target[0].Label {
		renameCost = 1
	}

	matchCost := renameCost +
		DistanceForestC(source[0].Children, target[0].Children, cache) +
		DistanceForestC(source[1:], target[1:], cache)

	return min(deleteCost, insertCost, matchCost)

}

func forestIDs(forest []*Node) string {
	var ids strings.Builder
	for _, node := range forest {
		if node == nil {
			ids.WriteString("nil,")
			continue
		}
		ids.WriteString(strconv.Itoa(node.ID))
		ids.WriteByte(',')
	}
	return ids.String()
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

func DeleteRootTransformation(forest []*Node) []*Node {
	root := forest[0]

	var newForest []*Node

	rest := forest[1:]
	newForest = append(newForest, root.Children...)
	newForest = append(newForest, rest...)

	return newForest

}
