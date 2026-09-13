package engine

import "fmt"

// Apply applies one edit and reconciles its descendants by default.
func (state *WorkingState) Apply(index int) error {
	return state.ApplyWithOptions(index, ApplyOptions{Reconcile: true})
}

// ApplyWithOptions applies one edit using explicit reconciliation behavior.
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
	state.prepareStructuralSlotEdits(index)
	if options.Reconcile {
		normalizeAppliedStructure(state.working, state.target, state.edits, state.status)
	}
	return nil
}

func (state *WorkingState) prepareStructuralSlotEdits(index int) {
	parent := state.edits[index]
	for childIndex, edit := range state.edits {
		if edit.ParentID != parent.NodeID || state.status[childIndex] == EditApplied {
			continue
		}
		if parent.NodeKind == "*ast.FuncDecl" || parent.NodeKind == "*ast.FuncLit" {
			if edit.NodeKind == "*ast.FuncType" || edit.NodeKind == "*ast.BlockStmt" {
				state.status[childIndex] = EditPrepared
			}
		}
		if parent.NodeKind == "*ast.IfStmt" && (edit.Field == "Cond" || edit.Field == "Body") {
			state.status[childIndex] = EditPrepared
		}
	}
}

// ApplySubtree applies an edit and every dependent child edit below it.
// Parents are applied before children so the intermediate tree can be built
// in structural order.
func (state *WorkingState) ApplySubtree(index int) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	visited := make(map[int]bool)
	var apply func(int) error
	apply = func(current int) error {
		if visited[current] {
			return nil
		}
		visited[current] = true
		if state.status[current] != EditApplied {
			if err := state.Apply(current); err != nil {
				return err
			}
		}
		for childIndex := range state.edits {
			if state.edits[childIndex].ParentID == state.edits[current].NodeID {
				if err := apply(childIndex); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return apply(index)
}

// ApplySiblings applies the selected edit and every edit with the same parent.
// Their parents and ancestors are deliberately not included.
func (state *WorkingState) ApplySiblings(index int) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	parentID := state.edits[index].ParentID
	for siblingIndex, edit := range state.edits {
		if edit.ParentID == parentID {
			if err := state.ApplySubtree(siblingIndex); err != nil {
				return err
			}
		}
	}
	return nil
}

// Remove removes an applied edit and rebuilds the working tree.
func (state *WorkingState) Remove(index int) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	state.status[index] = EditRemoved
	return state.rebuild()
}

// RemoveSubtree removes an edit and every dependent child edit, then rebuilds.
func (state *WorkingState) RemoveSubtree(index int) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	visited := make(map[int]bool)
	var remove func(int)
	remove = func(current int) {
		if visited[current] {
			return
		}
		visited[current] = true
		state.status[current] = EditRemoved
		for childIndex := range state.edits {
			if state.edits[childIndex].ParentID == state.edits[current].NodeID {
				remove(childIndex)
			}
		}
	}
	remove(index)
	return state.rebuild()
}

// RemoveSiblings removes the selected edit and every edit with the same parent.
// Their parents and ancestors are deliberately not included.
func (state *WorkingState) RemoveSiblings(index int) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	parentID := state.edits[index].ParentID
	for siblingIndex, edit := range state.edits {
		if edit.ParentID == parentID {
			if err := state.RemoveSubtree(siblingIndex); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reconcile moves a candidate back to its intended parent.
func (state *WorkingState) Reconcile(candidate ReconciliationCandidate) error {
	return ApplyReconciliationCandidate(state.working, state.target, candidate)
}

// Candidates lists reconciliation opportunities below an ancestor node.
func (state *WorkingState) Candidates(ancestorID string) []ReconciliationCandidate {
	return ReconciliationCandidates(state.working, state.target, ancestorID)
}

// Snapshot returns a copy suitable for UI or transport serialization.
func (state *WorkingState) Snapshot() WorkingSnapshot {
	root := state.working.clone()
	rendered := RenderBestEffort(root)
	return WorkingSnapshot{
		Root:              root,
		RenderedCode:      rendered.Code,
		RenderDiagnostics: append([]string(nil), rendered.Diagnostics...),
		Edits:             append([]structuralEdit(nil), state.edits...),
		Status:            append([]EditStatus(nil), state.status...),
	}
}

// ValidateGo reports structural validity diagnostics for the working tree.
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
	for index, status := range state.status {
		if status == EditApplied && state.options[index].Reconcile {
			normalizeAppliedStructure(state.working, state.target, state.edits, state.status)
			break
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
