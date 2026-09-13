package main

import "fmt"

type EditStatus string

const (
	EditUnapplied EditStatus = "unapplied"
	EditApplied   EditStatus = "applied"
	EditRemoved   EditStatus = "removed"
)

type WorkingSnapshot struct {
	Root   *structuralASTNode
	Edits  []structuralEdit
	Status []EditStatus
}

type ValidationReport struct {
	Valid       bool
	Diagnostics []string
}

type WorkingState struct {
	source  *structuralASTNode
	target  *structuralASTNode
	working *structuralASTNode
	edits   []structuralEdit
	status  []EditStatus
	options []ApplyOptions
}

func NewWorkingState(source, target *structuralASTNode, edits []structuralEdit) (*WorkingState, error) {
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

func (state *WorkingState) Apply(index int) error {
	return state.ApplyWithOptions(index, ApplyOptions{Reconcile: true})
}

func (state *WorkingState) ApplyWithOptions(index int, options ApplyOptions) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	if state.status[index] == EditApplied && !options.Reconcile {
		return nil
	}
	if err := applyStructuralEditWithOptions(state.working, state.target, state.edits[index], options); err != nil {
		return err
	}
	state.status[index] = EditApplied
	state.options[index] = options
	return nil
}

func (state *WorkingState) Remove(index int) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	state.status[index] = EditRemoved
	return state.rebuild()
}

func (state *WorkingState) Reconcile(candidate ReconciliationCandidate) error {
	return ApplyReconciliationCandidate(state.working, state.target, candidate)
}

func (state *WorkingState) Candidates(ancestorID string) []ReconciliationCandidate {
	return ReconciliationCandidates(state.working, state.target, ancestorID)
}

func (state *WorkingState) Snapshot() WorkingSnapshot {
	return WorkingSnapshot{
		Root:   state.working.clone(),
		Edits:  append([]structuralEdit(nil), state.edits...),
		Status: append([]EditStatus(nil), state.status...),
	}
}

func (state *WorkingState) ValidateGo() ValidationReport {
	diagnostics := make([]string, 0)
	var walk func(*structuralASTNode)
	walk = func(node *structuralASTNode) {
		if node == nil {
			return
		}
		for _, child := range node.Children {
			if node.Kind == "*ast.BlockStmt" && child.Field == "List" && !isGoStatementKind(child.Kind) {
				diagnostics = append(diagnostics, fmt.Sprintf("%s cannot be in BlockStmt.List", child.Kind))
			}
			walk(child)
		}
		if node.Kind == "*ast.AssignStmt" && (len(childrenForField(node, "Lhs")) == 0 || len(childrenForField(node, "Rhs")) == 0) {
			diagnostics = append(diagnostics, "assignment is missing a left-hand or right-hand expression")
		}
	}
	walk(state.working)
	return ValidationReport{Valid: len(diagnostics) == 0, Diagnostics: diagnostics}
}

func childrenForField(node *structuralASTNode, field string) []*structuralASTNode {
	children := make([]*structuralASTNode, 0)
	for _, child := range node.Children {
		if child.Field == field {
			children = append(children, child)
		}
	}
	return children
}

func isGoStatementKind(kind string) bool {
	switch kind {
	case "*ast.AssignStmt", "*ast.BlockStmt", "*ast.BranchStmt", "*ast.CaseClause", "*ast.CommClause", "*ast.DeclStmt", "*ast.DeferStmt", "*ast.EmptyStmt", "*ast.ExprStmt", "*ast.ForStmt", "*ast.GoStmt", "*ast.IfStmt", "*ast.IncDecStmt", "*ast.LabeledStmt", "*ast.RangeStmt", "*ast.ReturnStmt", "*ast.SelectStmt", "*ast.SendStmt", "*ast.SwitchStmt", "*ast.TypeSwitchStmt":
		return true
	default:
		return false
	}
}

func (state *WorkingState) rebuild() error {
	state.working = state.source.clone()
	for index, status := range state.status {
		if status != EditApplied {
			continue
		}
		if err := applyStructuralEditWithOptions(state.working, state.target, state.edits[index], state.options[index]); err != nil {
			return fmt.Errorf("replay edit %d: %w", index, err)
		}
	}
	return nil
}

func (state *WorkingState) checkEditIndex(index int) error {
	if index < 0 || index >= len(state.edits) {
		return fmt.Errorf("edit index %d is out of range", index)
	}
	return nil
}
