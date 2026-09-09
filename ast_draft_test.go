package main

import (
	"testing"

	gumast "github.com/Xanonymous-GitHub/gumtree-go/ast"
)

func TestDraftNodeInsertAndRemove(t *testing.T) {
	root := &draftNode{id: gumast.NodeIdType("1"), label: "File"}
	block := &draftNode{id: gumast.NodeIdType("2"), label: "BlockStmt"}
	call := &draftNode{id: gumast.NodeIdType("3"), label: "CallExpr"}
	ifNode := &draftNode{id: gumast.NodeIdType("4"), label: "IfStmt"}

	root.appendChild(block)
	block.appendChild(call)
	block.insertChild(0, ifNode)

	if block.children[0] != ifNode || block.children[1] != call {
		t.Fatalf("children were not inserted in order: %#v", block.children)
	}
	if ifNode.parent != block || call.parent != block {
		t.Fatal("inserted nodes did not receive their parent")
	}
	if !block.removeChild(ifNode) || ifNode.parent != nil {
		t.Fatal("remove did not detach the child")
	}
}

func TestDraftCloneDoesNotShareChildren(t *testing.T) {
	root := &draftNode{id: gumast.NodeIdType("1"), label: "File"}
	child := &draftNode{id: gumast.NodeIdType("2"), label: "FuncDecl"}
	root.appendChild(child)

	clone := root.clone(nil)
	clone.children[0].label = gumast.NodeLabelType("BlockStmt")
	if root.children[0].label == clone.children[0].label {
		t.Fatal("clone shares child state with original")
	}
	if clone.children[0].parent != clone {
		t.Fatal("clone child has incorrect parent")
	}
}

func TestDraftPath(t *testing.T) {
	root := &draftNode{id: gumast.NodeIdType("1")}
	middle := &draftNode{id: gumast.NodeIdType("2")}
	leaf := &draftNode{id: gumast.NodeIdType("3")}
	root.appendChild(middle)
	middle.appendChild(leaf)

	path := draftPath(leaf)
	if len(path) != 3 || path[0] != root || path[1] != middle || path[2] != leaf {
		t.Fatalf("path = %#v, want root-to-leaf path", path)
	}
}
