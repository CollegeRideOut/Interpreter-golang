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
}
