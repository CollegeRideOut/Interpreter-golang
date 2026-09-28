package engine

import html "github.com/tree-sitter/tree-sitter-html/bindings/go"

// ParseHTML converts HTML source into the editable structural tree.
func ParseHTML(source []byte) (*Node, error) {
	return parseTreeSitter(source, "html", html.Language())
}

// DiffHTML returns HTML trees and their structural edit script.
func DiffHTML(source, target []byte) (sourceTree, targetTree *Node, edits []Edit, err error) {
	sourceTree, err = ParseHTML(source)
	if err != nil {
		return nil, nil, nil, err
	}
	targetTree, err = ParseHTML(target)
	if err != nil {
		return nil, nil, nil, err
	}
	edits = simpleStructuralEditScripts(sourceTree, targetTree)
	bindStructuralEditIdentities(sourceTree, targetTree, edits)
	return sourceTree, targetTree, edits, nil
}

// NewWorkingStateFromHTML creates an editable HTML session.
func NewWorkingStateFromHTML(source, target []byte) (*WorkingState, error) {
	sourceTree, targetTree, edits, err := DiffHTML(source, target)
	if err != nil {
		return nil, err
	}
	return NewWorkingState(sourceTree, targetTree, edits)
}
