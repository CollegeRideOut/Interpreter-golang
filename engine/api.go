package engine

import "fmt"

// Node is the structural representation of a Go AST node.
//
// Nodes are intentionally editable even when the resulting tree is not valid
// Go. ValidateGo reports structural diagnostics without discarding the tree.
type Node = structuralASTNode

// Edit describes one structural change from a source tree to a target tree.
type Edit = structuralEdit

type EditAncestor struct {
	NodeID    string `json:"nodeId"`
	GlobalID  string `json:"globalId,omitempty"`
	NodeKind  string `json:"nodeKind"`
	Field     string `json:"field,omitempty"`
	Value     string `json:"value,omitempty"`
	StartLine int    `json:"startLine,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
}

// ApplyOptions controls how an edit is applied to the working tree.
type ApplyOptions struct {
	Reconcile bool
}

// ReconciliationCandidate describes a node that can potentially be moved
// back to its intended parent in the working tree.
type ReconciliationCandidate struct {
	NodeID                 string `json:"nodeId"`
	NodeGlobalID           string `json:"nodeGlobalId"`
	OriginalParentID       string `json:"originalParentId,omitempty"`
	OriginalParentGlobalID string `json:"originalParentGlobalId,omitempty"`
	CurrentParentID        string `json:"currentParentId,omitempty"`
	CurrentParentGlobalID  string `json:"currentParentGlobalId,omitempty"`
	OriginalPath           string `json:"originalPath,omitempty"`
	CurrentPath            string `json:"currentPath,omitempty"`
	CanReconcile           bool   `json:"canReconcile"`
	Reason                 string `json:"reason,omitempty"`
}

// WorkingState owns a source/target edit session and its cumulative working
// tree. Its fields are private so callers can change state only through the
// high-level operations below.
type WorkingState struct {
	source  *structuralASTNode
	target  *structuralASTNode
	working *structuralASTNode
	edits   []structuralEdit
	status  []EditStatus
	options []ApplyOptions
}

// EditStatus is the lifecycle status of one edit in a working session.
type EditStatus string

const (
	EditUnapplied EditStatus = "unapplied"
	EditApplied   EditStatus = "applied"
	EditPrepared  EditStatus = "prepared"
	EditRemoved   EditStatus = "removed"
)

// WorkingSnapshot is an immutable copy of the current session state.
type WorkingSnapshot struct {
	Root              *Node        `json:"root"`
	RenderedCode      string       `json:"renderedCode"`
	RenderDiagnostics []string     `json:"renderDiagnostics,omitempty"`
	Edits             []Edit       `json:"edits"`
	Status            []EditStatus `json:"status"`
}

// ValidationReport contains structural validity diagnostics for the current
// working tree.
type ValidationReport struct {
	Valid       bool
	Diagnostics []string
}

// Parse converts valid Go source into the editable structural representation.
func Parse(source []byte) (*Node, error) {
	tree, fileSet, err := parseGoAST(source)
	if err != nil {
		return nil, err
	}
	return structuralASTTree(tree, fileSet), nil
}

// Diff parses source and target and returns both structural trees plus the
// identity-aware edit script needed to transform source into target.
func Diff(source, target []byte) (sourceTree, targetTree *Node, edits []Edit, err error) {
	sourceAST, sourceFileSet, err := parseGoAST(source)
	if err != nil {
		return nil, nil, nil, err
	}
	targetAST, targetFileSet, err := parseGoAST(target)
	if err != nil {
		return nil, nil, nil, err
	}

	sourceTree = structuralASTTree(sourceAST, sourceFileSet)
	targetTree = structuralASTTree(targetAST, targetFileSet)
	edits = simpleASTEditScripts(sourceAST, targetAST, sourceFileSet, targetFileSet)
	bindStructuralEditIdentities(sourceTree, targetTree, edits)
	return sourceTree, targetTree, edits, nil
}

// NewWorkingStateFromSource parses source and target before creating a state.
func NewWorkingStateFromSource(source, target []byte) (*WorkingState, error) {
	sourceTree, targetTree, edits, err := Diff(source, target)
	if err != nil {
		return nil, err
	}
	return NewWorkingState(sourceTree, targetTree, edits)
}

// NewWorkingState creates a working session from already parsed trees and an
// edit script. Most callers should prefer NewWorkingStateFromSource.
func NewWorkingState(source, target *Node, edits []Edit) (*WorkingState, error) {
	if source == nil {
		return nil, fmt.Errorf("source tree is nil")
	}
	if target == nil {
		return nil, fmt.Errorf("target tree is nil")
	}
	sourceCopy := source.clone()
	targetCopy := target.clone()
	linkStructuralIdentities(sourceCopy, targetCopy)
	state := &WorkingState{
		source:  sourceCopy,
		target:  targetCopy,
		working: sourceCopy.clone(),
		edits:   append([]structuralEdit(nil), edits...),
		status:  make([]EditStatus, len(edits)),
		options: make([]ApplyOptions, len(edits)),
	}
	bindStructuralEditIdentities(state.source, state.target, state.edits)
	for index := range state.status {
		state.status[index] = EditUnapplied
		state.options[index] = ApplyOptions{Reconcile: true}
	}
	return state, nil
}

// ReconciliationCandidates returns nodes under ancestorID whose current
// parent differs from their intended target parent.
func ReconciliationCandidates(working, target *Node, ancestorID string) []ReconciliationCandidate {
	return reconciliationCandidates(working, target, ancestorID)
}

// ApplyReconciliationCandidate restores a candidate to its intended parent.
func ApplyReconciliationCandidate(working, target *Node, candidate ReconciliationCandidate) error {
	return applyReconciliationCandidate(working, target, candidate)
}

// Clone returns an independent copy of the node and its descendants.
func (node *structuralASTNode) Clone() *Node {
	if node == nil {
		return nil
	}
	return node.clone()
}
