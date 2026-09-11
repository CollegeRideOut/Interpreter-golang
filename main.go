package main

import (
	"bytes"
	"fmt"
	gumast "github.com/Xanonymous-GitHub/gumtree-go/ast"
	rl "github.com/gen2brain/raylib-go/raylib"
	"interpreter/ast"
	"interpreter/diffing"
	"interpreter/lexer"
	"interpreter/parser"
	treeeditdistance "interpreter/treeEditDistance"
	"os"
	"strings"
)

func main() {
	interactive, directory, err := parseArguments(os.Args[1:])
	if err != nil {
		panic(err)
	}

	if interactive {
		if err := runInteractive(directory); err != nil {
			panic(err)
		}
		return
	}
	if err := exportASTDiff(directory); err != nil {
		panic(err)
	}
}

func parseArguments(arguments []string) (bool, string, error) {
	interactive := false
	directory := "TestProgram"
	directorySet := false
	for _, argument := range arguments {
		switch argument {
		case "--interactive":
			interactive = true
		default:
			if strings.HasPrefix(argument, "-") {
				return false, "", fmt.Errorf("unknown option %q", argument)
			}
			if directorySet {
				return false, "", fmt.Errorf("only one repository directory may be provided")
			}
			directory = argument
			directorySet = true
		}
	}
	return interactive, directory, nil
}

func runRaylib(diff *inMemoryDiff) {
	rl.SetConfigFlags(rl.FlagWindowResizable)
	rl.InitWindow(1100, 700, "contuts structural review")
	defer rl.CloseWindow()
	rl.SetWindowMinSize(720, 420)
	rl.SetTargetFPS(60)
	font := rl.LoadFontEx("/usr/share/fonts/liberation/LiberationSans-Regular.ttf", 24, nil, 0)
	defer rl.UnloadFont(font)

	for !rl.WindowShouldClose() {
		updateReview(diff)
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(20, 23, 30, 255))
		drawReview(diff, font)
		rl.EndDrawing()
	}
}

func drawReview(diff *inMemoryDiff, font rl.Font) {
	width, _, margin, panelWidth, footerTop, footerHeight := reviewLayout()
	astTop := int32(92)
	astHeight := footerTop - astTop - 12
	contentGap := int32(12)
	contentWidth := (panelWidth - contentGap) / 2
	rightPanelX := margin + contentWidth + contentGap

	titleSize := 24
	if width < 850 {
		titleSize = 20
	}
	drawText(font, "EDIT SCRIPT ATTACHED TO ORIGINAL AST", int(margin+8), 24, titleSize, rl.RayWhite)
	drawText(font, "HEAD~1 -> working tree", int(margin+10), 58, 18, rl.LightGray)

	rl.DrawRectangle(margin, astTop, contentWidth, astHeight, rl.NewColor(30, 34, 43, 255))
	rl.DrawRectangle(rightPanelX, astTop, contentWidth, astHeight, rl.NewColor(30, 34, 43, 255))
	drawText(font, diff.sourceFile, int(margin+18), int(astTop+18), 18, rl.NewColor(130, 190, 255, 255))
	rows := makeASTRows(diff)
	drawASTRows(font, rows, diff, int(margin+18), int(astTop+56), contentWidth)
	fileTitle := "BEFORE FILE"
	if !bytes.Equal(diff.workingSource, diff.sourceSource) {
		fileTitle = "WORKING FILE"
	}
	drawText(font, fileTitle, int(rightPanelX+18), int(astTop+18), 18, rl.NewColor(150, 225, 175, 255))
	if diff.parseError != "" {
		drawText(font, "DRAFT INVALID: "+diff.parseError, int(rightPanelX+18), int(astTop+40), 14, rl.NewColor(255, 125, 125, 255))
	}
	drawSourceFile(font, diff.workingSource, int(rightPanelX+18), int(astTop+56))

	rl.DrawRectangle(margin, footerTop, panelWidth, footerHeight, rl.NewColor(42, 47, 58, 255))
	if len(diff.scripts) == 0 {
		drawText(font, "No edit scripts", int(margin+18), int(footerTop+18), 18, rl.LightGray)
		return
	}
	for index, script := range diff.scripts {
		y := footerTop + 10 + int32(index*24)
		checkboxColor := rl.NewColor(85, 92, 105, 255)
		if diff.selected[index] {
			checkboxColor = rl.NewColor(255, 205, 110, 255)
		}
		rl.DrawRectangle(margin+18, y, 18, 18, checkboxColor)
		status := ""
		if diff.applied[index] {
			status = " (applied)"
		}
		textColor := rl.NewColor(255, 205, 110, 255)
		if diff.applied[index] {
			textColor = rl.NewColor(130, 210, 155, 255)
		}
		drawText(font, fmt.Sprintf("[%d] %s%s", index+1, script, status), int(margin+46), int(y), 16, textColor)
	}
	drawText(font, "Click an AST edit, Enter toggle, Shift+Enter toggle subtree, Ctrl+Z undo, Ctrl+Y redo", int(margin+18), int(footerTop+54), 14, rl.LightGray)
}

func reviewLayout() (width, height, margin, panelWidth, footerTop, footerHeight int32) {
	width = int32(rl.GetScreenWidth())
	height = int32(rl.GetScreenHeight())
	margin = 24
	panelWidth = width - margin*2
	footerHeight = 92
	footerTop = height - footerHeight
	return
}

func updateReview(diff *inMemoryDiff) {
	controlDown := rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)
	if controlDown && rl.IsKeyPressed(rl.KeyZ) {
		undoEdit(diff)
		return
	}
	if controlDown && rl.IsKeyPressed(rl.KeyY) {
		redoEdit(diff)
		return
	}
	if rl.IsKeyPressed(rl.KeyEnter) {
		for index, selected := range diff.selected {
			if selected {
				toggleEdit(diff, index)
				return
			}
		}
		rows := makeASTRows(diff)
		if len(rows) > 0 {
			if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
				applyASTRowEdits(diff, rows[diff.selectedRow], true)
			} else {
				applyASTRowEdits(diff, rows[diff.selectedRow], false)
			}
		}
		return
	}
	rows := makeASTRows(diff)
	if len(rows) > 0 {
		if rl.IsKeyPressed(rl.KeyRight) {
			expandASTRow(diff, rows[diff.selectedRow], true)
			return
		}
		if rl.IsKeyPressed(rl.KeyLeft) {
			expandASTRow(diff, rows[diff.selectedRow], false)
			return
		}
		if rl.IsKeyPressed(rl.KeyUp) && diff.selectedRow > 0 {
			diff.selectedRow--
			return
		}
		if rl.IsKeyPressed(rl.KeyDown) && diff.selectedRow < len(rows)-1 {
			diff.selectedRow++
			return
		}
	}
	if !rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		return
	}

	_, _, margin, panelWidth, footerTop, _ := reviewLayout()
	mouse := rl.GetMousePosition()
	contentGap := int32(12)
	contentWidth := (panelWidth - contentGap) / 2
	for index := range rows {
		row := rl.Rectangle{
			X:      float32(margin + 10),
			Y:      float32(148 + index*24 - 4),
			Width:  float32(contentWidth - 20),
			Height: 22,
		}
		if rl.CheckCollisionPointRec(mouse, row) {
			diff.selectedRow = index
			return
		}
	}
	for index := range diff.scripts {
		row := rl.Rectangle{
			X:      float32(margin + 10),
			Y:      float32(footerTop + 6 + int32(index*24)),
			Width:  float32(panelWidth - 20),
			Height: 22,
		}
		if rl.CheckCollisionPointRec(mouse, row) {
			diff.selected[index] = !diff.selected[index]
			return
		}
	}
}

type astRow struct {
	node        *gumast.Node
	depth       int
	editIndices []int
}

func makeASTRows(diff *inMemoryDiff) []astRow {
	children := make(map[int][]int)
	for index, script := range diff.scripts {
		if script.parentIndex >= 0 {
			children[script.parentIndex] = append(children[script.parentIndex], index)
		}
	}
	rows := make([]astRow, 0, len(diff.scripts))
	var appendRow func(int, int)
	appendRow = func(index, depth int) {
		script := diff.scripts[index]
		node := script.source
		if script.kind == editInsert && script.target != nil {
			node = script.target
		}
		rows = append(rows, astRow{node: node, depth: depth, editIndices: []int{index}})
		if !diff.expanded[index] {
			return
		}
		for _, child := range children[index] {
			appendRow(child, depth+1)
		}
	}
	for index, script := range diff.scripts {
		if script.parentIndex < 0 {
			_ = script
			appendRow(index, 0)
		}
	}
	return rows
}

func drawASTRows(font rl.Font, rows []astRow, diff *inMemoryDiff, x, y int, width int32) {
	for index, row := range rows {
		rowY := y + index*24
		if index == diff.selectedRow {
			rl.DrawRectangle(int32(x-8), int32(rowY-4), width-20, 24, rl.NewColor(58, 74, 98, 255))
		}
		drawASTRow(font, row, diff, x, rowY)
	}
}

func drawASTRow(font rl.Font, row astRow, diff *inMemoryDiff, x, y int) {
	node := row.node
	line := strings.Repeat("  ", row.depth) + string(node.Label)
	if node.Value != "" {
		line += " [" + string(node.Value) + "]"
	}
	color := rl.LightGray
	if len(row.editIndices) > 0 {
		color = rl.NewColor(255, 205, 110, 255)
		for _, editIndex := range row.editIndices {
			script := diff.scripts[editIndex]
			if hasEditChildren(diff, editIndex) {
				indicator := "[-] "
				if !diff.expanded[editIndex] {
					indicator = "[+] "
				}
				line = strings.Repeat("  ", row.depth) + indicator + string(node.Label)
			}
			line += fmt.Sprintf("  [%d] %s", editIndex+1, script)
			if !diff.applied[editIndex] {
				continue
			}
			color = rl.NewColor(130, 210, 155, 255)
		}
	}
	drawText(font, line, x, y, 18, color)
}

func hasEditChildren(diff *inMemoryDiff, index int) bool {
	for _, script := range diff.scripts {
		if script.parentIndex == index {
			return true
		}
	}
	return false
}

func expandASTRow(diff *inMemoryDiff, row astRow, expanded bool) {
	if len(row.editIndices) == 0 {
		return
	}
	diff.expanded[row.editIndices[0]] = expanded
}

func applyASTRowEdits(diff *inMemoryDiff, row astRow, all bool) {
	editIndices := row.editIndices
	if all {
		editIndices = editSubtreeIndices(diff, row.editIndices[0])
	}
	if !all && len(editIndices) > 0 {
		toggleEdit(diff, editIndices[0])
		return
	}

	if len(editIndices) == 0 {
		return
	}
	apply := false
	for _, editIndex := range editIndices {
		if !diff.applied[editIndex] {
			apply = true
			break
		}
	}
	setEditsApplied(diff, editIndices, apply)
}

func editSubtreeIndices(diff *inMemoryDiff, root int) []int {
	indices := []int{root}
	for index, script := range diff.scripts {
		if script.parentIndex == root {
			indices = append(indices, editSubtreeIndices(diff, index)...)
		}
	}
	return indices
}

func toggleEdit(diff *inMemoryDiff, index int) {
	setEditsApplied(diff, []int{index}, !diff.applied[index])
}

func setEditsApplied(diff *inMemoryDiff, indices []int, applied bool) {
	desired := append([]bool(nil), diff.applied...)
	for _, index := range indices {
		if index >= 0 && index < len(desired) {
			desired[index] = applied
		}
	}

	diff.workingSource = append([]byte(nil), diff.sourceSource...)
	diff.workingDraft = diff.sourceDraft.clone(nil)
	for index := range diff.scripts {
		diff.scripts[index].start = diff.scripts[index].baseStart
		diff.scripts[index].end = diff.scripts[index].baseEnd
		diff.applied[index] = false
		diff.selected[index] = false
	}
	diff.undoStack = nil
	diff.redoStack = nil

	for index, shouldApply := range desired {
		if !shouldApply || hasDesiredEditAncestor(diff, index, desired) {
			continue
		}
		diff.selected[index] = true
		applySelectedEdits(diff)
	}
	if allEditsSelected(desired) && diff.targetSource != nil {
		diff.workingSource = append([]byte(nil), diff.targetSource...)
		updateParseStatus(diff)
	}
	diff.applied = desired
}

func hasDesiredEditAncestor(diff *inMemoryDiff, index int, desired []bool) bool {
	for parent := diff.scripts[index].parentIndex; parent >= 0; parent = diff.scripts[parent].parentIndex {
		if parent == index {
			break
		}
		if desired[parent] {
			return true
		}
	}
	return false
}

func allEditsSelected(selected []bool) bool {
	if len(selected) == 0 {
		return false
	}
	for _, value := range selected {
		if !value {
			return false
		}
	}
	return true
}

func applySelectedEdits(diff *inMemoryDiff) {
	for index, script := range diff.scripts {
		if !diff.selected[index] || diff.applied[index] {
			continue
		}
		if hasAppliedEditAncestor(diff, index) {
			diff.applied[index] = true
			diff.selected[index] = false
			if allEditsApplied(diff) && diff.targetSource != nil {
				diff.workingSource = append([]byte(nil), diff.targetSource...)
				updateParseStatus(diff)
			}
			return
		}
		var updated []byte
		var err error
		if diff.workingDraft != nil {
			applyDraftScript(diff, script)
			updated = renderDraft(diff.workingDraft)
		} else {
			updated, err = applyEdit(diff.workingSource, script)
		}
		if err != nil {
			continue
		}
		history := appliedEdit{
			index:        index,
			start:        script.start,
			end:          script.end,
			beforeSource: append([]byte(nil), diff.workingSource...),
			beforeStarts: scriptStarts(diff),
			beforeEnds:   scriptEnds(diff),
		}
		diff.workingSource = updated
		updateParseStatus(diff)
		diff.applied[index] = true
		diff.selected[index] = false
		diff.redoStack = nil
		shiftScripts(diff, index, script.start, len(script.replacement)-(script.end-script.start))
		if allEditsApplied(diff) && diff.targetSource != nil {
			diff.workingSource = append([]byte(nil), diff.targetSource...)
			updateParseStatus(diff)
		}
		history.afterSource = append([]byte(nil), diff.workingSource...)
		history.afterStarts = scriptStarts(diff)
		history.afterEnds = scriptEnds(diff)
		diff.undoStack = append(diff.undoStack, history)
		return
	}
}

func applyDraftScript(diff *inMemoryDiff, script editScript) {
	if diff.workingDraft == nil {
		return
	}
	if script.kind == editDelete {
		if node := diff.workingDraft.find(script.source.Id); node != nil {
			node.detach()
		}
		return
	}
	if script.target == nil {
		return
	}
	ensureDraftTargetPath(diff, script)
}

func ensureDraftTargetPath(diff *inMemoryDiff, script editScript) *draftNode {
	path := draftGumPath(script.target)
	anchor := diff.workingDraft.find(script.source.Id)
	start := 0
	for index, candidate := range path {
		if candidate.Label == script.source.Label && candidate.Value == script.source.Value {
			if existing := diff.workingDraft.find(candidate.Id); existing != nil {
				anchor = existing
			}
			start = index + 1
			break
		}
	}
	if anchor == nil {
		anchor = diff.workingDraft
	}
	for _, candidate := range path[start:] {
		if existing := diff.workingDraft.find(candidate.Id); existing != nil {
			anchor = existing
			continue
		}
		child := &draftNode{
			id:    candidate.Id,
			label: candidate.Label,
			value: candidate.Value,
			role:  draftGumChildRole(candidate.Parent, candidate),
		}
		anchor.insertChild(draftGumChildIndex(candidate.Parent, candidate), child)
		anchor = child
	}
	return anchor
}

func draftGumChildIndex(parent, child *gumast.Node) int {
	if parent == nil || child == nil {
		return -1
	}
	for index, candidate := range parent.OrderedChildren() {
		if candidate == child {
			return index
		}
	}
	return -1
}

func draftGumChildRole(parent, child *gumast.Node) string {
	if parent == nil || child == nil {
		return "child"
	}
	children := parent.OrderedChildren()
	index := 0
	for candidateIndex, candidate := range children {
		if candidate == child {
			index = candidateIndex
			break
		}
	}
	switch parent.Label {
	case "*ast.File":
		if child.Label == "*ast.Ident" && index == 0 {
			return "package"
		}
		return "decl"
	case "*ast.BlockStmt":
		return "stmt"
	case "*ast.ExprStmt":
		return "x"
	case "*ast.CallExpr":
		if index == 0 {
			return "fun"
		}
		return "arg"
	case "*ast.SelectorExpr":
		if index == 0 {
			return "x"
		}
		return "sel"
	case "*ast.FuncLit":
		if child.Label == "*ast.BlockStmt" {
			return "body"
		}
		return "type"
	case "*ast.IfStmt":
		if child.Label == "*ast.BlockStmt" {
			return "body"
		}
		return "cond"
	case "*ast.GenDecl":
		return "spec"
	case "*ast.ImportSpec":
		return "path"
	}
	return "child"
}

func draftGumPath(node *gumast.Node) []*gumast.Node {
	path := make([]*gumast.Node, 0)
	for current := node; current != nil; current = current.Parent {
		path = append(path, current)
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}

func hasAppliedEditAncestor(diff *inMemoryDiff, index int) bool {
	for parent := diff.scripts[index].parentIndex; parent >= 0; parent = diff.scripts[parent].parentIndex {
		if parent == index {
			break
		}
		if diff.applied[parent] {
			return true
		}
	}
	return false
}

func shiftScripts(diff *inMemoryDiff, changedIndex, changeStart, delta int) {
	if delta == 0 {
		return
	}
	for index := range diff.scripts {
		if index == changedIndex {
			continue
		}
		if diff.scripts[index].start >= changeStart {
			diff.scripts[index].start += delta
			diff.scripts[index].end += delta
		}
	}
}

func undoEdit(diff *inMemoryDiff) {
	if len(diff.undoStack) == 0 {
		return
	}

	last := len(diff.undoStack) - 1
	history := diff.undoStack[last]
	diff.undoStack = diff.undoStack[:last]
	diff.workingSource = append([]byte(nil), history.beforeSource...)
	diff.workingDraft = diff.sourceDraft.clone(nil)
	updateParseStatus(diff)
	restoreScriptPositions(diff, history.beforeStarts, history.beforeEnds)
	diff.applied[history.index] = false
	diff.selected[history.index] = false
	diff.redoStack = append(diff.redoStack, history)
}

func redoEdit(diff *inMemoryDiff) {
	if len(diff.redoStack) == 0 {
		return
	}

	last := len(diff.redoStack) - 1
	history := diff.redoStack[last]
	diff.redoStack = diff.redoStack[:last]
	diff.workingSource = append([]byte(nil), history.afterSource...)
	if diff.workingDraft == nil {
		diff.workingDraft = diff.sourceDraft.clone(nil)
	}
	applyDraftScript(diff, diff.scripts[history.index])
	updateParseStatus(diff)
	restoreScriptPositions(diff, history.afterStarts, history.afterEnds)
	diff.applied[history.index] = true
	diff.selected[history.index] = false
	if allEditsApplied(diff) && diff.targetSource != nil {
		diff.workingSource = append([]byte(nil), diff.targetSource...)
		updateParseStatus(diff)
	}
	diff.undoStack = append(diff.undoStack, history)
}

func updateParseStatus(diff *inMemoryDiff) {
	if diff.sourceTree == nil {
		diff.parseError = ""
		return
	}
	if _, _, err := parseGoAST(diff.workingSource); err != nil {
		diff.parseError = err.Error()
		return
	}
	diff.parseError = ""
}

func allEditsApplied(diff *inMemoryDiff) bool {
	for _, applied := range diff.applied {
		if !applied {
			return false
		}
	}
	return len(diff.applied) > 0
}

func scriptStarts(diff *inMemoryDiff) []int {
	starts := make([]int, len(diff.scripts))
	for index, script := range diff.scripts {
		starts[index] = script.start
	}
	return starts
}

func scriptEnds(diff *inMemoryDiff) []int {
	ends := make([]int, len(diff.scripts))
	for index, script := range diff.scripts {
		ends[index] = script.end
	}
	return ends
}

func restoreScriptPositions(diff *inMemoryDiff, starts, ends []int) {
	for index := range diff.scripts {
		if index < len(starts) {
			diff.scripts[index].start = starts[index]
		}
		if index < len(ends) {
			diff.scripts[index].end = ends[index]
		}
	}
}

func drawText(font rl.Font, text string, x, y, size int, color rl.Color) {
	rl.DrawTextEx(font, text, rl.Vector2{X: float32(x), Y: float32(y)}, float32(size), 1, color)
}

func drawSourceFile(font rl.Font, source []byte, x, y int) {
	for lineNumber, line := range strings.Split(string(source), "\n") {
		drawText(font, fmt.Sprintf("%2d  %s", lineNumber+1, line), x, y+lineNumber*24, 18, rl.LightGray)
	}
}

func hasAffectedDescendant(node *gumast.Node, scripts map[gumast.NodeIdType]editScript) bool {
	if node == nil {
		return false
	}
	if _, ok := scripts[node.Id]; ok {
		return true
	}
	for _, child := range node.OrderedChildren() {
		if hasAffectedDescendant(child, scripts) {
			return true
		}
	}
	return false
}

func runMonekyTestProgram() {

	sourceProgram := parseMonkeyProgram("let x = 5;")
	targetProgram := parseMonkeyProgram("let x = 10;")
	sourceTree := diffing.MonkeyASTToTree(sourceProgram)
	targetTree := diffing.MonkeyASTToTree(targetProgram)
	edits := diffing.GetTreeDiff(sourceTree, targetTree)

	fmt.Println("Source Monkey AST:")
	printTree(sourceTree, "", true, true)

	fmt.Println("Target Monkey AST:")
	printTree(targetTree, "", true, true)

	fmt.Println("Edit path:")
	for _, edit := range edits {
		switch edit.Type {
		case diffing.EditInsert:
			fmt.Printf("%s %s\n", edit.Type, edit.To.Label)
		case diffing.EditDelete:
			fmt.Printf("%s %s\n", edit.Type, edit.From.Label)
		case diffing.EditUpdate:
			fmt.Printf("%s %s -> %s\n", edit.Type, edit.From.Label, edit.To.Label)
		}
	}

}

func parseMonkeyProgram(input string) *ast.Program {
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		panic(p.Errors())
	}
	return program
}

func printTree(node *treeeditdistance.Node, prefix string, isLast, isRoot bool) {
	if node == nil {
		return
	}

	if isRoot {
		fmt.Println(node.Label)
	} else {
		connector := "|-- "
		if isLast {
			connector = "`-- "
		}
		fmt.Printf("%s%s%s\n", prefix, connector, node.Label)
	}

	for index, child := range node.Children {
		lastChild := index == len(node.Children)-1
		childPrefix := prefix
		if !isRoot {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "|   "
			}
		}
		printTree(child, childPrefix, lastChild, false)
	}
}
