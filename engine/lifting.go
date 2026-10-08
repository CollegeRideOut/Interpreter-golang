package engine

import "strings"

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
		known[editIdentity(edit)] = true
		if parent := editParentIdentity(edit); parent != "" {
			children[parent] = append(children[parent], index)
		}
	}

	count := func(nodeID string) int {
		var walk func(string) int
		walk = func(id string) int {
			total := 0
			for _, child := range children[id] {
				total++
				total += walk(editIdentity(edits[child]))
			}
			return total
		}
		return walk(nodeID)
	}

	views := make([]EditView, 0, len(edits))
	var appendView func(int, int)
	appendView = func(index, depth int) {
		edit := edits[index]
		childIndexes := children[editIdentity(edit)]
		if edit.Hidden || options.HiddenKinds[edit.NodeKind] {
			for _, child := range childIndexes {
				appendView(child, depth)
			}
			return
		}
		views = append(views, EditView{
			EditIndex:       index,
			Depth:           depth,
			HasChildren:     len(childIndexes) > 0,
			DescendantCount: count(editIdentity(edit)),
		})
		for _, child := range childIndexes {
			appendView(child, depth+1)
		}
	}

	for index, edit := range edits {
		parent := editParentIdentity(edit)
		if parent == "" || !known[parent] {
			appendView(index, 0)
		}
	}
	return views
}

func editIdentity(edit structuralEdit) string {
	if edit.NodeGlobalID != "" {
		return edit.NodeGlobalID
	}
	if edit.SourceGlobalID != "" {
		return edit.SourceGlobalID
	}
	return edit.NodeID
}

func editParentIdentity(edit structuralEdit) string {
	if edit.ParentGlobalID != "" {
		return edit.ParentGlobalID
	}
	return edit.ParentID
}

// ApplyProjected applies a canonical edit and any edited ancestors required
// to make that projected row materializable. Hidden ancestors are included
// automatically because they are omitted from the projected view.
func (state *WorkingState) ApplyProjected(index int, options ApplyOptions, lifting LiftOptions) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	byNode := make(map[string]int, len(state.edits))
	for editIndex, edit := range state.edits {
		byNode[editIdentity(edit)] = editIndex
	}
	ancestors := make([]int, 0)
	for parentID := editParentIdentity(state.edits[index]); parentID != ""; {
		parentIndex, ok := byNode[parentID]
		if !ok {
			break
		}
		ancestors = append(ancestors, parentIndex)
		parentID = editParentIdentity(state.edits[parentIndex])
	}
	for ancestorIndex := len(ancestors) - 1; ancestorIndex >= 0; ancestorIndex-- {
		if state.status[ancestors[ancestorIndex]] != EditApplied {
			if err := state.ApplyWithOptions(ancestors[ancestorIndex], options); err != nil {
				return err
			}
		}
	}
	for _, editIndex := range state.replacementEdits(index) {
		if state.status[editIndex] == EditApplied {
			continue
		}
		if err := state.ApplyWithOptions(editIndex, options); err != nil {
			return err
		}
	}
	if state.edits[index].Kind == "INSERT" && isSyntaxManagedLanguage(state.edits[index].NodeKind) {
		if err := state.applyStructuralDescendants(index, options); err != nil {
			return err
		}
	}
	for _, editIndex := range state.implicitSyntaxEdits(index) {
		if state.status[editIndex] == EditApplied {
			continue
		}
		if err := state.ApplyWithOptions(editIndex, options); err != nil {
			return err
		}
	}
	return nil
}

func isSyntaxManagedLanguage(kind string) bool {
	return strings.HasPrefix(kind, "typescript:") || strings.HasPrefix(kind, "tsx:") || strings.HasPrefix(kind, "html:")
}

func (state *WorkingState) applyStructuralDescendants(index int, options ApplyOptions) error {
	parentID := editIdentity(state.edits[index])
	for childIndex, edit := range state.edits {
		if editParentIdentity(edit) != parentID || state.status[childIndex] == EditApplied {
			continue
		}
		if err := state.ApplyWithOptions(childIndex, options); err != nil {
			return err
		}
		if err := state.applyStructuralDescendants(childIndex, options); err != nil {
			return err
		}
	}
	return nil
}

// ApplyProjectedSubtree applies a projected edit and all dependent children.
func (state *WorkingState) ApplyProjectedSubtree(index int, options ApplyOptions, lifting LiftOptions) error {
	if err := state.ApplyProjected(index, options, lifting); err != nil {
		return err
	}
	parentID := editIdentity(state.edits[index])
	for childIndex, edit := range state.edits {
		if editParentIdentity(edit) == parentID {
			if err := state.ApplyProjectedSubtree(childIndex, options, lifting); err != nil {
				return err
			}
		}
	}
	return nil
}

// replacementEdits keeps a delete/insert pair for the same AST slot together.
// Applying only one side of a replacement can leave an intermediate tree such
// as `_ =`, which is useful internally but misleading as a user action.
func (state *WorkingState) replacementEdits(index int) []int {
	selected := state.edits[index]
	indexes := []int{index}
	for candidateIndex, candidate := range state.edits {
		if candidateIndex == index || candidate.ParentGlobalID == "" || candidate.ParentGlobalID != selected.ParentGlobalID || candidate.Field != selected.Field || candidate.Position != selected.Position {
			continue
		}
		if (selected.Kind == "DELETE" && candidate.Kind == "INSERT") || (selected.Kind == "INSERT" && candidate.Kind == "DELETE") {
			if candidate.Kind == "DELETE" {
				indexes = append([]int{candidateIndex}, indexes...)
			} else {
				indexes = append(indexes, candidateIndex)
			}
		}
	}
	return indexes
}

// RemoveProjected removes a projected edit and any now-empty hidden
// ancestors that existed only to contain it.
func (state *WorkingState) RemoveProjected(index int, lifting LiftOptions) error {
	if err := state.Remove(index); err != nil {
		return err
	}
	for _, editIndex := range state.implicitSyntaxEdits(index) {
		if state.status[editIndex] != EditApplied && state.status[editIndex] != EditPrepared {
			continue
		}
		if err := state.Remove(editIndex); err != nil {
			return err
		}
	}
	for _, editIndex := range state.replacementEdits(index) {
		if editIndex == index || (state.status[editIndex] != EditApplied && state.status[editIndex] != EditPrepared) {
			continue
		}
		if err := state.Remove(editIndex); err != nil {
			return err
		}
	}
	byNode := make(map[string]int, len(state.edits))
	for editIndex, edit := range state.edits {
		byNode[editIdentity(edit)] = editIndex
	}
	for parentID := editParentIdentity(state.edits[index]); parentID != ""; {
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
		parentID = editParentIdentity(state.edits[parentIndex])
	}
	return nil
}

// implicitSyntaxEdits returns concrete syntax edits in the same structural
// container as a semantic edit. They are implementation details of the
// container, such as the comma beside a newly inserted parameter.
func (state *WorkingState) implicitSyntaxEdits(index int) []int {
	if index < 0 || index >= len(state.edits) {
		return nil
	}
	selected := state.edits[index]
	parent := editParentIdentity(selected)
	if parent == "" {
		return nil
	}
	result := make([]int, 0)
	for candidateIndex, candidate := range state.edits {
		if candidateIndex == index || !candidate.Hidden || editParentIdentity(candidate) != parent {
			continue
		}
		result = append(result, candidateIndex)
	}
	return result
}

// RemoveProjectedSubtree removes a projected edit and all dependent children.
func (state *WorkingState) RemoveProjectedSubtree(index int, lifting LiftOptions) error {
	if err := state.checkEditIndex(index); err != nil {
		return err
	}
	parentID := editIdentity(state.edits[index])
	children := make([]int, 0)
	for childIndex, edit := range state.edits {
		if editParentIdentity(edit) == parentID {
			children = append(children, childIndex)
		}
	}
	for _, childIndex := range children {
		if err := state.RemoveProjectedSubtree(childIndex, lifting); err != nil {
			return err
		}
	}
	return state.RemoveProjected(index, lifting)
}

func (state *WorkingState) hasAppliedChildren(parentID string) bool {
	for index, edit := range state.edits {
		if editParentIdentity(edit) == parentID && (state.status[index] == EditApplied || state.status[index] == EditPrepared) {
			return true
		}
	}
	return false
}
