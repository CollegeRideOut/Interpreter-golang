package main

import (
	"bytes"
	"fmt"
	gumast "github.com/Xanonymous-GitHub/gumtree-go/ast"
	rl "github.com/gen2brain/raylib-go/raylib"
	goParser "go/parser"
	"go/token"
	"interpreter/ast"
	"interpreter/diffing"
	"interpreter/lexer"
	"interpreter/parser"
	treeeditdistance "interpreter/treeEditDistance"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	diff, err := prepareInMemoryDiff()
	if err != nil {
		panic(err)
	}
	runRaylib(diff)
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
	drawText(font, "In-memory example", int(margin+10), 58, 18, rl.LightGray)

	rl.DrawRectangle(margin, astTop, contentWidth, astHeight, rl.NewColor(30, 34, 43, 255))
	rl.DrawRectangle(rightPanelX, astTop, contentWidth, astHeight, rl.NewColor(30, 34, 43, 255))
	drawText(font, diff.sourceFile, int(margin+18), int(astTop+18), 18, rl.NewColor(130, 190, 255, 255))
	rows := makeASTRows(diff.focusedRoot, diff.scriptsByNode, diff.scriptIndicesByNode, diff.scripts)
	drawASTRows(font, rows, diff, int(margin+18), int(astTop+56), contentWidth)
	fileTitle := "BEFORE FILE"
	if !bytes.Equal(diff.workingSource, diff.sourceSource) {
		fileTitle = "WORKING FILE"
	}
	drawText(font, fileTitle, int(rightPanelX+18), int(astTop+18), 18, rl.NewColor(150, 225, 175, 255))
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
	drawText(font, "Click an AST edit, Enter apply, Shift+Enter apply children, Ctrl+Z undo, Ctrl+Y redo", int(margin+18), int(footerTop+54), 14, rl.LightGray)
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
			if selected && !diff.applied[index] {
				applySelectedEdits(diff)
				return
			}
		}
		rows := makeASTRows(diff.focusedRoot, diff.scriptsByNode, diff.scriptIndicesByNode, diff.scripts)
		if len(rows) > 0 {
			if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
				applyASTRowEdits(diff, rows[diff.selectedRow], true)
			} else {
				applyASTRowEdits(diff, rows[diff.selectedRow], false)
			}
		}
		return
	}
	rows := makeASTRows(diff.focusedRoot, diff.scriptsByNode, diff.scriptIndicesByNode, diff.scripts)
	if len(rows) > 0 {
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
		if rl.CheckCollisionPointRec(mouse, row) && !diff.applied[index] {
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

func makeASTRows(root *gumast.Node, scripts map[gumast.NodeIdType]editScript, scriptIndices map[gumast.NodeIdType][]int, orderedScripts []editScript) []astRow {
	rows := make([]astRow, 0, len(orderedScripts)+4)
	var walk func(*gumast.Node, int)
	walk = func(node *gumast.Node, depth int) {
		if node == nil || !hasAffectedDescendant(node, scripts) {
			return
		}

		editIndices := scriptIndices[node.Id]
		if len(editIndices) == 0 {
			rows = append(rows, astRow{node: node, depth: depth})
		} else {
			// Keep edits on the same source node independently selectable.
			for _, editIndex := range editIndices {
				rows = append(rows, astRow{node: node, depth: depth, editIndices: []int{editIndex}})
			}
		}
		for _, child := range node.OrderedChildren() {
			walk(child, depth+1)
		}
	}
	walk(root, 0)
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
			line += fmt.Sprintf("  [%d] %s", editIndex+1, script)
			if !diff.applied[editIndex] {
				continue
			}
			color = rl.NewColor(130, 210, 155, 255)
		}
	}
	drawText(font, line, x, y, 18, color)
}

func applyASTRowEdits(diff *inMemoryDiff, row astRow, all bool) {
	editIndices := row.editIndices
	if all {
		editIndices = subtreeEditIndices(row.node, diff.scriptIndicesByNode)
	}
	for _, editIndex := range editIndices {
		if diff.applied[editIndex] {
			continue
		}
		if !all {
			diff.selected[editIndex] = true
			applySelectedEdits(diff)
			return
		}
		diff.selected[editIndex] = true
		applySelectedEdits(diff)
	}
}

func subtreeEditIndices(node *gumast.Node, indicesByNode map[gumast.NodeIdType][]int) []int {
	if node == nil {
		return nil
	}

	indices := append([]int(nil), indicesByNode[node.Id]...)
	for _, child := range node.OrderedChildren() {
		indices = append(indices, subtreeEditIndices(child, indicesByNode)...)
	}
	return indices
}

func applySelectedEdits(diff *inMemoryDiff) {
	for index, script := range diff.scripts {
		if !diff.selected[index] || diff.applied[index] {
			continue
		}
		updated, err := applyEdit(diff.workingSource, script)
		if err != nil {
			continue
		}
		diff.workingSource = updated
		diff.applied[index] = true
		diff.selected[index] = false
		diff.undoStack = append(diff.undoStack, appliedEdit{index: index})
		diff.redoStack = nil
		shiftScripts(diff, index, script.start, len(script.replacement)-(script.end-script.start))
		return
	}
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
	script := diff.scripts[history.index]
	updated, err := applyEdit(diff.workingSource, script.reverse())
	if err != nil {
		return
	}

	diff.undoStack = diff.undoStack[:last]
	diff.workingSource = updated
	diff.applied[history.index] = false
	diff.selected[history.index] = false
	shiftScripts(diff, history.index, script.start, len(script.original)-len(script.replacement))
	diff.redoStack = append(diff.redoStack, history)
}

func redoEdit(diff *inMemoryDiff) {
	if len(diff.redoStack) == 0 {
		return
	}

	last := len(diff.redoStack) - 1
	history := diff.redoStack[last]
	script := diff.scripts[history.index]
	updated, err := applyEdit(diff.workingSource, script)
	if err != nil {
		return
	}

	diff.redoStack = diff.redoStack[:last]
	diff.workingSource = updated
	diff.applied[history.index] = true
	diff.selected[history.index] = false
	shiftScripts(diff, history.index, script.start, len(script.replacement)-(script.end-script.start))
	diff.undoStack = append(diff.undoStack, history)
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

func runTestProgram() {
	clonePath, err := clonePreviousTestProgram()
	if err != nil {
		panic(err)
	}
	fmt.Printf("TestProgram clone at HEAD~1: %s\n", clonePath)

	beforeSource, err := readTestProgramAt("HEAD~1")
	if err != nil {
		panic(err)
	}

	currentSource, err := readTestProgramAt("HEAD")
	if err != nil {
		panic(err)
	}

	beforeAST, err := goParser.ParseFile(token.NewFileSet(), "main.go", beforeSource, 0)
	if err != nil {
		panic(err)
	}

	currentAST, err := goParser.ParseFile(token.NewFileSet(), "main.go", currentSource, 0)
	if err != nil {
		panic(err)
	}

	beforeTree := diffing.GoASTToTree(beforeAST)
	currentTree := diffing.GoASTToTree(currentAST)
	edits := diffing.GetTreeDiff(beforeTree, currentTree)

	fmt.Println("Before Go AST:")
	printTree(beforeTree, "", true, true)

	fmt.Println("Current Go AST:")
	printTree(currentTree, "", true, true)

	fmt.Println("Go AST edit path:")
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

func clonePreviousTestProgram() (string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cloneParent, err := os.MkdirTemp("", "test-program-clone-")
	if err != nil {
		return "", err
	}

	sourceRepository := filepath.Join(workingDirectory, "TestProgram")
	clonePath := filepath.Join(cloneParent, "TestProgram")
	cloneCommand := exec.Command("git", "clone", "--no-local", sourceRepository, clonePath)
	if output, err := cloneCommand.CombinedOutput(); err != nil {
		return "", fmt.Errorf("clone TestProgram: %w\n%s", err, output)
	}

	checkoutCommand := exec.Command("git", "checkout", "--detach", "HEAD~1")
	checkoutCommand.Dir = clonePath
	if output, err := checkoutCommand.CombinedOutput(); err != nil {
		return "", fmt.Errorf("checkout previous TestProgram commit: %w\n%s", err, output)
	}

	return clonePath, nil
}

func readTestProgramAt(revision string) ([]byte, error) {
	command := exec.Command("git", "show", revision+":main.go")
	command.Dir = "TestProgram"

	return command.Output()

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
