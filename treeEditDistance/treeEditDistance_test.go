package treeeditdistance

import (
	"reflect"
	"testing"
)

func TestTreeConstructionAndTraversal(t *testing.T) {
	root := NewNode("program")
	root.AddChild(NewNode("let")).AddChild(NewNode("identifier"))
	root.AddChild(NewNode("return"))
	tree := NewTree(root)

	var labels []string
	tree.WalkPreOrder(func(node *Node) {
		labels = append(labels, node.Label)
	})

	want := []string{"program", "let", "identifier", "return"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("pre-order traversal = %v, want %v", labels, want)
	}
	if tree.Size() != 4 {
		t.Fatalf("tree size = %d, want 4", tree.Size())
	}
}

func TestEmptyTree(t *testing.T) {
	if NewTree(nil).Size() != 0 {
		t.Fatal("empty tree should have size 0")
	}
	if !NewNode("leaf").IsLeaf() {
		t.Fatal("node without children should be a leaf")
	}
}

func TestCreateSimilarTrees(t *testing.T) {
	tree1, tree2 := CreateSimilarTrees()

	var tree1Labels, tree2Labels []string
	tree1.WalkPreOrder(func(node *Node) {
		tree1Labels = append(tree1Labels, node.Label)
	})
	tree2.WalkPreOrder(func(node *Node) {
		tree2Labels = append(tree2Labels, node.Label)
	})

	if !reflect.DeepEqual(tree1Labels, []string{"d", "b", "a", "c", "f", "e", "g"}) {
		t.Fatalf("T1 labels = %v", tree1Labels)
	}
	if !reflect.DeepEqual(tree2Labels, []string{"f", "e", "x", "g"}) {
		t.Fatalf("T2 labels = %v", tree2Labels)
	}
}

func TestDistanceSimilarTrees(t *testing.T) {
	tree1, tree2 := CreateSimilarTrees()

	got := DistanceForest(
		[]*Node{tree1.Root},
		[]*Node{tree2.Root},
	)

	want := 5
	if got != want {
		t.Fatalf("distance = %d, want %d", got, want)
	}

	if got := DistanceBottomUp([]*Node{tree1.Root}, []*Node{tree2.Root}); got != want {
		t.Fatalf("bottom-up distance = %d, want %d", got, want)
	}
}

func TestDistanceBottomUpSimpleChanges(t *testing.T) {
	source := NewNode("1", NewNode("2", NewNode("3")))
	target := NewNode("1", NewNode("2", NewNode("4")))

	if got := DistanceBottomUp([]*Node{source}, []*Node{target}); got != 1 {
		t.Fatalf("bottom-up rename distance = %d, want 1", got)
	}

	if got := DistanceBottomUp(nil, []*Node{target}); got != 3 {
		t.Fatalf("bottom-up insertion distance = %d, want 3", got)
	}
	if got := DistanceBottomUp([]*Node{source}, nil); got != 3 {
		t.Fatalf("bottom-up deletion distance = %d, want 3", got)
	}
}

func TestPostorderIDsAndLeftmostDescendants(t *testing.T) {
	root := NewNode("a", NewNode("b", NewNode("c")), NewNode("d"))
	assignIDs([]*Node{root})

	if root.Children[0].Children[0].ID != 1 ||
		root.Children[0].ID != 2 ||
		root.Children[1].ID != 3 ||
		root.ID != 4 {
		t.Fatalf("postorder IDs are incorrect")
	}

	got := leftmostDescendants([]*Node{root})
	want := map[int]int{
		1: 1, // c -> c
		2: 1, // b -> c
		3: 3, // d -> d
		4: 1, // a -> c
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("leftmost descendants = %v, want %v", got, want)
	}
}

func TestInitializeForestDistanceTable(t *testing.T) {
	got := initializeForestDistanceTable(2, 3)
	want := [][]int{
		{0, 1, 2, 3},
		{1, 0, 0, 0},
		{2, 0, 0, 0},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forest distance table = %v, want %v", got, want)
	}
}

func TestKeyroots(t *testing.T) {
	root := NewNode("a", NewNode("b", NewNode("c")), NewNode("d"))
	assignIDs([]*Node{root})

	nodes := postorderNodes([]*Node{root})
	got := keyroots(nodes, leftmostDescendants([]*Node{root}))
	want := []int{3, 4}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keyroots = %v, want %v", got, want)
	}
}

func TestInitializeForestRegion(t *testing.T) {
	root := NewNode("4", NewNode("2", NewNode("1")), NewNode("3"))
	assignIDs([]*Node{root})

	table := initializeForestDistanceTable(4, 4)
	leftmost := leftmostDescendants([]*Node{root})
	initializeForestRegion(table, 3, 3, leftmost, leftmost)

	if table[2][2] != 0 {
		t.Fatalf("region origin = %d, want 0", table[2][2])
	}
	if table[3][2] != 1 {
		t.Fatalf("source border = %d, want 1", table[3][2])
	}
	if table[2][3] != 1 {
		t.Fatalf("target border = %d, want 1", table[2][3])
	}
}
