package treeeditdistance

// Node is a labeled node in an ordered tree. Children are kept in the order
// in which they are added, which is significant for tree edit distance.
type Node struct {
	ID       int
	Label    string
	Value    any
	Children []*Node
}

// NewNode creates a node with the supplied label and optional children.
func NewNode(label string, children ...*Node) *Node {
	return &Node{
		Label:    label,
		Children: children,
	}
}

// NewNodeWithValue creates a node while preserving the original value.
func NewNodeWithValue(label string, value any, children ...*Node) *Node {
	return &Node{
		Label:    label,
		Value:    value,
		Children: children,
	}
}

// AddChild appends child to the node's children and returns the child. The
// return value makes it convenient to build a tree inline.
func (n *Node) AddChild(child *Node) *Node {
	if child == nil {
		return nil
	}

	n.Children = append(n.Children, child)
	return child
}

// IsLeaf reports whether the node has no children.
func (n *Node) IsLeaf() bool {
	return n == nil || len(n.Children) == 0
}

// Tree is a rooted, ordered tree.
type Tree struct {
	Root *Node
}

// NewTree creates a tree with root as its root node.
func NewTree(root *Node) *Tree {
	return &Tree{Root: root}
}

// assignIDs gives every node a unique 1-based postorder ID.
func assignIDs(forest []*Node) {
	nextID := 1

	var walk func(*Node)
	walk = func(node *Node) {
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

// CreateSimilarTrees creates the two example trees shown in the tree edit
// distance example.
//
// T1:             T2:
//
//	     d               f
//	   /   \           /   \
//	  b     f         e     g
//	 / \   / \        |
//	a   c e   g       x
func CreateSimilarTrees() (*Tree, *Tree) {
	return createSmilarTrees()
}

// createSmilarTrees is the internal constructor for the two example trees.
func createSmilarTrees() (*Tree, *Tree) {
	tree1Root := NewNode("d")
	tree1Root.AddChild(NewNode("b", NewNode("a"), NewNode("c")))
	tree1Root.AddChild(NewNode("f", NewNode("e"), NewNode("g")))

	tree2Root := NewNode("f")
	tree2Root.AddChild(NewNode("e", NewNode("x")))
	tree2Root.AddChild(NewNode("g"))

	return NewTree(tree1Root), NewTree(tree2Root)
}

// Size returns the number of nodes in the tree.
func (t *Tree) Size() int {
	if t == nil || t.Root == nil {
		return 0
	}

	size := 0
	t.WalkPreOrder(func(*Node) {
		size++
	})
	return size
}

// WalkPreOrder visits each node before its children, from left to right.
func (t *Tree) WalkPreOrder(visit func(*Node)) {
	if t == nil || t.Root == nil || visit == nil {
		return
	}

	var walk func(*Node)
	walk = func(node *Node) {
		visit(node)
		for _, child := range node.Children {
			if child != nil {
				walk(child)
			}
		}
	}
	walk(t.Root)
}
