package engine

// LiftOptions controls which structural edits are hidden from a projected view.
// The canonical edit list is never changed by projection.
type LiftOptions struct {
	HiddenKinds map[string]bool
}

// EditView is one row in a projected edit tree. EditIndex points back to the
// canonical edit and is therefore stable across different projections.
type EditView struct {
	EditIndex       int  `json:"editIndex"`
	Depth           int  `json:"depth"`
	HasChildren     bool `json:"hasChildren"`
	DescendantCount int  `json:"descendantCount"`
}

// ProjectEdits hides configured structural shells while retaining their
// descendants at the shell's depth. This is a view operation, not a mutation.
func ProjectEdits(edits []Edit, options LiftOptions) []EditView {
	children := make(map[string][]int)
	known := make(map[string]bool, len(edits))
	for index, edit := range edits {
		known[edit.NodeID] = true
		if edit.ParentID != "" {
			children[edit.ParentID] = append(children[edit.ParentID], index)
		}
	}

	count := func(nodeID string) int {
		var walk func(string) int
		walk = func(id string) int {
			total := 0
			for _, child := range children[id] {
				total++
				total += walk(edits[child].NodeID)
			}
			return total
		}
		return walk(nodeID)
	}

	views := make([]EditView, 0, len(edits))
	var appendView func(int, int)
	appendView = func(index, depth int) {
		edit := edits[index]
		childIndexes := children[edit.NodeID]
		if options.HiddenKinds[edit.NodeKind] {
			for _, child := range childIndexes {
				appendView(child, depth)
			}
			return
		}
		views = append(views, EditView{
			EditIndex:       index,
			Depth:           depth,
			HasChildren:     len(childIndexes) > 0,
			DescendantCount: count(edit.NodeID),
		})
		for _, child := range childIndexes {
			appendView(child, depth+1)
		}
	}

	for index, edit := range edits {
		if edit.ParentID == "" || !known[edit.ParentID] {
			appendView(index, 0)
		}
	}
	return views
}

// ApplyProjected applies a canonical edit and any hidden ancestors required
// to make that projected row materializable.
func (state *WorkingState) ApplyProjected(index int, options ApplyOptions, lifting LiftOptions) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	byNode := make(map[string]int, len(state.edits))
	for editIndex, edit := range state.edits {
		byNode[edit.NodeID] = editIndex
	}
	ancestors := make([]int, 0)
	for parentID := state.edits[index].ParentID; parentID != ""; {
		parentIndex, ok := byNode[parentID]
		if !ok {
			break
		}
		if !lifting.HiddenKinds[state.edits[parentIndex].NodeKind] {
			break
		}
		ancestors = append(ancestors, parentIndex)
		parentID = state.edits[parentIndex].ParentID
	}
	for ancestorIndex := len(ancestors) - 1; ancestorIndex >= 0; ancestorIndex-- {
		if state.status[ancestors[ancestorIndex]] != EditApplied {
			if err := state.ApplyWithOptions(ancestors[ancestorIndex], options); err != nil {
				return err
			}
		}
	}
	return state.ApplyWithOptions(index, options)
}

// RemoveProjected removes a projected edit and any now-empty hidden
// ancestors that existed only to contain it.
func (state *WorkingState) RemoveProjected(index int, lifting LiftOptions) error {
	if err := state.Remove(index); err != nil {
		return err
	}
	byNode := make(map[string]int, len(state.edits))
	for editIndex, edit := range state.edits {
		byNode[edit.NodeID] = editIndex
	}
	for parentID := state.edits[index].ParentID; parentID != ""; {
		parentIndex, ok := byNode[parentID]
		if !ok || !lifting.HiddenKinds[state.edits[parentIndex].NodeKind] {
			break
		}
		if state.status[parentIndex] != EditApplied || state.hasAppliedChildren(parentID) {
			break
		}
		if err := state.Remove(parentIndex); err != nil {
			return err
		}
		parentID = state.edits[parentIndex].ParentID
	}
	return nil
}

func (state *WorkingState) hasAppliedChildren(parentID string) bool {
	for index, edit := range state.edits {
		if edit.ParentID == parentID && (state.status[index] == EditApplied || state.status[index] == EditPrepared) {
			return true
		}
	}
	return false
}
