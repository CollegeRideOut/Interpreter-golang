package main

import (
	"bytes"
	"fmt"
	goast "go/ast"
	goFormat "go/format"
	goParser "go/parser"
	"go/token"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	gumast "github.com/Xanonymous-GitHub/gumtree-go/ast"
	"github.com/Xanonymous-GitHub/gumtree-go/comparator"
)

type inMemoryDiff struct {
	sourceTree          gumast.AST
	targetTree          gumast.AST
	sourceDraft         *draftNode
	targetDraft         *draftNode
	workingDraft        *draftNode
	focusedRoot         *gumast.Node
	sourceFile          string
	sourceSource        []byte
	targetSource        []byte
	workingSource       []byte
	scripts             []editScript
	scriptsByNode       map[gumast.NodeIdType]editScript
	scriptIndicesByNode map[gumast.NodeIdType][]int
	selected            []bool
	applied             []bool
	selectedRow         int
	expanded            map[int]bool
	parseError          string
	undoStack           []appliedEdit
	redoStack           []appliedEdit
}

func prepareDirectoryDiff(directory string) (*inMemoryDiff, error) {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve directory %q: %w", directory, err)
	}

	filePath := filepath.Join(directory, "main.go")
	target, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read working file %q: %w", filePath, err)
	}

	source, err := readGitFile(directory, "HEAD~1", "main.go")
	if err != nil {
		return nil, fmt.Errorf("read previous file from %q: %w", directory, err)
	}
	if formatted, err := goFormat.Source(source); err == nil {
		source = formatted
	}
	if formatted, err := goFormat.Source(target); err == nil {
		target = formatted
	}

	sourceAST, sourceFileSet, err := parseGoAST(source)
	if err != nil {
		return nil, fmt.Errorf("parse previous %q: %w", filePath, err)
	}
	targetAST, targetFileSet, err := parseGoAST(target)
	if err != nil {
		return nil, fmt.Errorf("parse working file %q: %w", filePath, err)
	}

	sourceTree, sourceNodes, err := goASTToGumTree(sourceAST)
	if err != nil {
		return nil, fmt.Errorf("build source GumTree AST: %w", err)
	}
	targetTree, targetNodes, err := goASTToGumTree(targetAST)
	if err != nil {
		return nil, fmt.Errorf("build target GumTree AST: %w", err)
	}

	comparison := comparator.NewComparator(&sourceTree, &targetTree, 0, 10000, 0.5, *slog.Default())
	mappings := comparison.Compare()
	scripts := buildEditScripts(sourceTree.Root(), targetTree.Root(), sourceNodes, targetNodes, sourceFileSet, targetFileSet, mappings, source, target)

	scriptsByNode := make(map[gumast.NodeIdType]editScript, len(scripts))
	scriptIndicesByNode := make(map[gumast.NodeIdType][]int)
	for index, script := range scripts {
		scriptsByNode[script.source.Id] = script
		scriptIndicesByNode[script.source.Id] = append(scriptIndicesByNode[script.source.Id], index)
	}

	return &inMemoryDiff{
		sourceTree:          sourceTree,
		targetTree:          targetTree,
		sourceDraft:         newDraftTreeFromAST(sourceTree.Root(), sourceNodes, sourceFileSet, source),
		targetDraft:         newDraftTreeFromAST(targetTree.Root(), targetNodes, targetFileSet, target),
		workingDraft:        newDraftTreeFromAST(sourceTree.Root(), sourceNodes, sourceFileSet, source),
		focusedRoot:         focusedASTRoot(scripts),
		sourceFile:          filePath,
		sourceSource:        source,
		targetSource:        target,
		workingSource:       append([]byte(nil), source...),
		scripts:             scripts,
		scriptsByNode:       scriptsByNode,
		scriptIndicesByNode: scriptIndicesByNode,
		selected:            make([]bool, len(scripts)),
		applied:             make([]bool, len(scripts)),
		expanded:            expandedEditNodes(scripts),
	}, nil
}

func expandedEditNodes(scripts []editScript) map[int]bool {
	expanded := make(map[int]bool)
	for index := range scripts {
		for _, script := range scripts {
			if script.parentIndex == index {
				expanded[index] = true
				break
			}
		}
	}
	return expanded
}

func readGitFile(directory, revision, filePath string) ([]byte, error) {
	command := exec.Command("git", "-C", directory, "show", revision+":"+filePath)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w\n%s", revision, filePath, err, output)
	}
	return output, nil
}

func focusedASTRoot(scripts []editScript) *gumast.Node {
	if len(scripts) == 0 {
		return nil
	}

	focused := scripts[0].source
	for _, script := range scripts[1:] {
		focused = commonAncestor(focused, script.source)
	}
	if focused.Parent != nil && focused == scripts[0].source {
		focused = focused.Parent
	}
	return focused
}

func commonAncestor(left, right *gumast.Node) *gumast.Node {
	ancestors := make(map[*gumast.Node]bool)
	for node := left; node != nil; node = node.Parent {
		ancestors[node] = true
	}
	for node := right; node != nil; node = node.Parent {
		if ancestors[node] {
			return node
		}
	}
	return nil
}

type editScript struct {
	kind        editKind
	source      *gumast.Node
	target      *gumast.Node
	parentIndex int
	start       int
	end         int
	baseStart   int
	baseEnd     int
	targetStart int
	targetEnd   int
	original    []byte
	replacement []byte
	description string
}

type editKind string

const (
	editUpdate editKind = "UPDATE"
	editInsert editKind = "INSERT"
)

type appliedEdit struct {
	index        int
	start        int
	end          int
	beforeSource []byte
	afterSource  []byte
	beforeStarts []int
	beforeEnds   []int
	afterStarts  []int
	afterEnds    []int
}

func (script editScript) reverse() editScript {
	reversed := editScript{
		kind:        editKindReverse(script.kind),
		source:      script.target,
		target:      script.source,
		start:       script.start,
		end:         script.start + len(script.replacement),
		baseStart:   script.baseStart,
		baseEnd:     script.baseStart + len(script.replacement),
		original:    append([]byte(nil), script.replacement...),
		replacement: append([]byte(nil), script.original...),
	}
	return reversed
}

func (script editScript) String() string {
	if script.kind == editInsert {
		return "INSERT " + script.description
	}
	if script.kind == editDelete {
		return fmt.Sprintf("DELETE %s:%s", script.source.Label, script.source.Value)
	}
	return fmt.Sprintf("UPDATE %s:%s -> %s:%s", script.source.Label, script.source.Value, script.target.Label, script.target.Value)
}

func editKindReverse(kind editKind) editKind {
	if kind == editInsert {
		return editDelete
	}
	return editUpdate
}

const editDelete editKind = "DELETE"

func parseGoAST(source []byte) (*goast.File, *token.FileSet, error) {
	fileSet := token.NewFileSet()
	file, err := goParser.ParseFile(fileSet, "example.go", source, 0)
	return file, fileSet, err
}

type gumTreeBuilder struct {
	tree  gumast.AST
	stack []*gumast.Node
	nodes map[gumast.NodeIdType]goast.Node
	err   error
}

func goASTToGumTree(root goast.Node) (gumast.AST, map[gumast.NodeIdType]goast.Node, error) {
	builder := &gumTreeBuilder{
		tree:  gumast.NewAST(*slog.Default()),
		nodes: make(map[gumast.NodeIdType]goast.Node),
	}
	goast.Walk(builder, root)
	return builder.tree, builder.nodes, builder.err
}

func (builder *gumTreeBuilder) Visit(node goast.Node) goast.Visitor {
	if builder.err != nil {
		return nil
	}

	if node == nil {
		if len(builder.stack) > 0 {
			builder.stack = builder.stack[:len(builder.stack)-1]
		}
		return nil
	}

	parent := (*gumast.Node)(nil)
	index := -1
	if len(builder.stack) > 0 {
		parent = builder.stack[len(builder.stack)-1]
		index = len(parent.Children)
	}

	created, err := builder.tree.Add(parent, index, gumast.NodeLabelType(goNodeLabel(node)), gumast.NodeValueType(goNodeValue(node)))
	if err != nil {
		builder.err = err
		return nil
	}

	builder.stack = append(builder.stack, created)
	builder.nodes[created.Id] = node
	return builder
}

func goNodeLabel(node goast.Node) string {
	return fmt.Sprintf("%T", node)
}

func goNodeValue(node goast.Node) string {
	switch node := node.(type) {
	case *goast.Ident:
		return node.Name
	case *goast.BasicLit:
		return node.Value
	case *goast.BinaryExpr:
		return node.Op.String()
	case *goast.UnaryExpr:
		return node.Op.String()
	case *goast.AssignStmt:
		return node.Tok.String()
	case *goast.GenDecl:
		return node.Tok.String()
	case *goast.File:
		return node.Name.Name
	default:
		return ""
	}
}

func printGumTree(node *gumast.Node, prefix string) {
	if node == nil {
		return
	}

	value := string(node.Value)
	if value != "" {
		fmt.Printf("%s%s [%s]\n", prefix, node.Label, value)
	} else {
		fmt.Printf("%s%s\n", prefix, node.Label)
	}
	for _, child := range node.OrderedChildren() {
		printGumTree(child, prefix+"  ")
	}
}

func buildEditScripts(source, target *gumast.Node, sourceNodes, targetNodes map[gumast.NodeIdType]goast.Node, sourceFileSet, targetFileSet *token.FileSet, mappings []comparator.Mapping, sourceBytes, targetBytes []byte) []editScript {
	if source == nil || target == nil {
		return nil
	}

	sourceToTarget := make(map[gumast.NodeIdType]gumast.NodeIdType, len(mappings))
	targetToSource := make(map[gumast.NodeIdType]gumast.NodeIdType, len(mappings))
	for _, mapping := range mappings {
		sourceToTarget[mapping.Source.Id] = mapping.Target.Id
		targetToSource[mapping.Target.Id] = mapping.Source.Id
	}
	if source.Label == target.Label && source.Value == target.Value {
		sourceToTarget[source.Id] = target.Id
		targetToSource[target.Id] = source.Id
	}
	augmentStructuralMappings(source, target, sourceToTarget, targetToSource)

	scripts := make([]editScript, 0)
	for sourceID, targetID := range sourceToTarget {
		sourceNode := findGumNode(source, sourceID)
		targetNode := findGumNode(target, targetID)
		if sourceNode == nil || targetNode == nil || sourceNode.Label == targetNode.Label && sourceNode.Value == targetNode.Value {
			continue
		}
		sourceValue, sourceOK := sourceNodes[sourceID]
		targetValue, targetOK := targetNodes[targetID]
		if !sourceOK || !targetOK {
			continue
		}
		start, end := nodeSourceRange(sourceFileSet, sourceValue, sourceBytes)
		targetStart, targetEnd := nodeSourceRange(targetFileSet, targetValue, targetBytes)
		scripts = append(scripts, newEditScript(editUpdate, sourceNode, targetNode, start, end, targetStart, targetEnd, sourceBytes, targetBytes))
	}

	walkGumTree(target, func(targetNode *gumast.Node) {
		if _, mapped := targetToSource[targetNode.Id]; mapped {
			return
		}
		targetValue, ok := targetNodes[targetNode.Id]
		if !ok || !isASTEditNode(targetValue) {
			return
		}
		start := insertionOffset(targetNode, targetToSource, sourceNodes, targetNodes, sourceFileSet, targetFileSet, sourceBytes)
		targetStart, targetEnd := nodeSourceRange(targetFileSet, targetValue, targetBytes)
		sourceNode := source
		if mappedParent := nearestMappedTargetParent(targetNode, targetToSource); mappedParent != nil {
			if sourceID, ok := targetToSource[mappedParent.Id]; ok {
				if mappedNode := findGumNode(source, sourceID); mappedNode != nil {
					sourceNode = mappedNode
				}
			}
		}
		script := newEditScript(editInsert, sourceNode, targetNode, start, start, targetStart, targetEnd, sourceBytes, targetBytes)
		script.replacement = shallowNodeSource(targetValue, targetStart, targetEnd, targetFileSet, targetBytes)
		scripts = append(scripts, script)
	})

	walkGumTree(source, func(sourceNode *gumast.Node) {
		if _, mapped := sourceToTarget[sourceNode.Id]; mapped {
			return
		}
		sourceValue, ok := sourceNodes[sourceNode.Id]
		if !ok || !isASTEditNode(sourceValue) {
			return
		}
		start, end := nodeSourceRange(sourceFileSet, sourceValue, sourceBytes)
		scripts = append(scripts, newEditScript(editDelete, sourceNode, nil, start, end, 0, 0, sourceBytes, targetBytes))
	})

	return organizeEditScripts(scripts, source, target)
}

func isASTEditNode(node goast.Node) bool {
	if node == nil {
		return false
	}
	_, isFile := node.(*goast.File)
	return !isFile
}

func nearestMappedTargetParent(node *gumast.Node, targetToSource map[gumast.NodeIdType]gumast.NodeIdType) *gumast.Node {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if _, ok := targetToSource[parent.Id]; ok {
			return parent
		}
	}
	return nil
}

func organizeEditScripts(scripts []editScript, source, target *gumast.Node) []editScript {
	sort.SliceStable(scripts, func(left, right int) bool {
		if scripts[left].start != scripts[right].start {
			return scripts[left].start < scripts[right].start
		}
		return scripts[left].targetStart < scripts[right].targetStart
	})

	for index := range scripts {
		scripts[index].parentIndex = -1
		candidate := scripts[index].target
		if candidate == nil {
			candidate = scripts[index].source
		}
		for parent := candidate.Parent; parent != nil; parent = parent.Parent {
			for parentIndex := range scripts {
				parentNode := scripts[parentIndex].target
				if parentNode == nil {
					parentNode = scripts[parentIndex].source
				}
				if parentNode == parent {
					scripts[index].parentIndex = parentIndex
					break
				}
			}
			if scripts[index].parentIndex >= 0 {
				break
			}
		}
	}
	return scripts
}

func augmentStructuralMappings(source, target *gumast.Node, sourceToTarget, targetToSource map[gumast.NodeIdType]gumast.NodeIdType) {
	changed := true
	for changed {
		changed = false
		walkGumTree(source, func(sourceNode *gumast.Node) {
			targetID, mapped := sourceToTarget[sourceNode.Id]
			if !mapped {
				return
			}
			targetNode := findGumNode(target, targetID)
			if targetNode == nil {
				return
			}
			sourceChildren := sourceNode.OrderedChildren()
			for _, targetChild := range targetNode.OrderedChildren() {
				if _, alreadyMapped := targetToSource[targetChild.Id]; alreadyMapped {
					continue
				}
				for _, sourceChild := range sourceChildren {
					if _, alreadyMapped := sourceToTarget[sourceChild.Id]; alreadyMapped || sourceChild.Label != targetChild.Label {
						continue
					}
					sourceToTarget[sourceChild.Id] = targetChild.Id
					targetToSource[targetChild.Id] = sourceChild.Id
					changed = true
					break
				}
			}
		})
	}
}

func findGumNode(root *gumast.Node, id gumast.NodeIdType) *gumast.Node {
	var found *gumast.Node
	walkGumTree(root, func(node *gumast.Node) {
		if node.Id == id {
			found = node
		}
	})
	return found
}

func walkGumTree(root *gumast.Node, visit func(*gumast.Node)) {
	if root == nil {
		return
	}
	visit(root)
	for _, child := range root.OrderedChildren() {
		walkGumTree(child, visit)
	}
}

func nodeSourceRange(fileSet *token.FileSet, node goast.Node, source []byte) (int, int) {
	start := nodeOffset(fileSet, node.Pos())
	end := nodeOffset(fileSet, node.End())
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(source) {
		end = len(source)
	}
	if _, ok := node.(goast.Stmt); ok {
		return lineStart(source, start), lineEnd(source, end)
	}
	if _, ok := node.(goast.Decl); ok {
		return lineStart(source, start), lineEnd(source, end)
	}
	return start, end
}

func nodeOffset(fileSet *token.FileSet, position token.Pos) int {
	file := fileSet.File(position)
	if file == nil {
		return 0
	}
	return file.Offset(position)
}

func newEditScript(kind editKind, source, target *gumast.Node, start, end, targetStart, targetEnd int, sourceBytes, targetBytes []byte) editScript {
	return editScript{
		kind:        kind,
		source:      source,
		target:      target,
		start:       start,
		end:         end,
		baseStart:   start,
		baseEnd:     end,
		targetStart: targetStart,
		targetEnd:   targetEnd,
		original:    append([]byte(nil), sourceBytes[start:end]...),
		replacement: append([]byte(nil), targetBytes[targetStart:targetEnd]...),
		description: editDescription(kind, source, target),
	}
}

func shallowNodeSource(node goast.Node, start, end int, fileSet *token.FileSet, source []byte) []byte {
	if node == nil || start < 0 || end < start || end > len(source) {
		return nil
	}

	switch node := node.(type) {
	case *goast.ExprStmt:
		return []byte(";")
	case *goast.CallExpr:
		return []byte("()")
	case *goast.SelectorExpr:
		return []byte(".")
	case *goast.FuncLit:
		return []byte("func() {}")
	case *goast.BlockStmt:
		return []byte("{}")
	case *goast.IfStmt:
		return []byte("if {}")
	case *goast.ForStmt:
		return []byte("for {}")
	case *goast.RangeStmt:
		return []byte("for {}")
	case *goast.ReturnStmt:
		return []byte("return")
	case *goast.AssignStmt:
		return []byte(node.Tok.String())
	case *goast.GenDecl:
		if node.Tok.String() == "import" {
			return []byte("import ()")
		}
		return []byte(node.Tok.String())
	case *goast.FuncDecl:
		return []byte("func() {}")
	case *goast.ImportSpec:
		return []byte("import")
	}

	type blockRange struct{ start, end int }
	blocks := make([]blockRange, 0)
	goast.Inspect(node, func(child goast.Node) bool {
		block, ok := child.(*goast.BlockStmt)
		if !ok {
			return true
		}
		blockStart := nodeOffset(fileSet, block.Pos())
		blockEnd := nodeOffset(fileSet, block.End())
		if blockStart >= start && blockEnd <= end {
			blocks = append(blocks, blockRange{start: blockStart, end: blockEnd})
		}
		return true
	})
	if len(blocks) == 0 {
		return append([]byte(nil), source[start:end]...)
	}

	// Keep braces for every outer block, but leave its child statements for
	// their own edit candidates.
	sort.Slice(blocks, func(left, right int) bool {
		if blocks[left].start != blocks[right].start {
			return blocks[left].start < blocks[right].start
		}
		return blocks[left].end > blocks[right].end
	})
	result := make([]byte, 0, end-start)
	position := start
	for _, block := range blocks {
		if block.start < position || block.end > end {
			continue
		}
		open := bytes.IndexByte(source[block.start:block.end], '{')
		if open < 0 {
			continue
		}
		open += block.start
		close := block.end - 1
		if close <= open || source[close] != '}' {
			continue
		}
		result = append(result, source[position:open+1]...)
		result = append(result, source[close:]...)
		position = block.end
	}
	result = append(result, source[position:end]...)
	return result
}

func editDescription(kind editKind, source, target *gumast.Node) string {
	if kind == editDelete {
		return fmt.Sprintf("subtree %s", source.Label)
	}
	return fmt.Sprintf("subtree %s", target.Label)
}

func insertionOffset(targetNode *gumast.Node, targetToSource map[gumast.NodeIdType]gumast.NodeIdType, sourceNodes, targetNodes map[gumast.NodeIdType]goast.Node, sourceFileSet, targetFileSet *token.FileSet, source []byte) int {
	anchor := targetNode
	for anchor.Parent != nil {
		if _, mapped := targetToSource[anchor.Parent.Id]; mapped {
			break
		}
		anchor = anchor.Parent
	}
	parent := anchor.Parent
	if parent == nil {
		return len(source)
	}
	parentSourceID, ok := targetToSource[parent.Id]
	if !ok {
		return len(source)
	}
	parentSource, ok := sourceNodes[parentSourceID]
	if !ok {
		return len(source)
	}
	if file, ok := parentSource.(*goast.File); ok {
		if len(file.Decls) > 0 {
			start, _ := nodeSourceRange(sourceFileSet, file.Decls[0], source)
			return start
		}
		return len(source)
	}

	children := parent.OrderedChildren()
	childIndex := 0
	for index, child := range children {
		if child == anchor {
			childIndex = index
			break
		}
	}
	for index := childIndex - 1; index >= 0; index-- {
		if sourceID, ok := targetToSource[children[index].Id]; ok {
			if sibling, ok := sourceNodes[sourceID]; ok {
				_, end := nodeSourceRange(sourceFileSet, sibling, source)
				return end
			}
		}
	}
	for index := childIndex + 1; index < len(children); index++ {
		if sourceID, ok := targetToSource[children[index].Id]; ok {
			if sibling, ok := sourceNodes[sourceID]; ok {
				start, _ := nodeSourceRange(sourceFileSet, sibling, source)
				return start
			}
		}
	}

	start, end := nodeSourceRange(sourceFileSet, parentSource, source)
	if block, ok := parentSource.(*goast.BlockStmt); ok {
		_, end = nodeSourceRange(sourceFileSet, block, source)
		return lineStart(source, end-1)
	}
	return lineEnd(source, start)
}

func removeOverlappingScripts(scripts []editScript) []editScript {
	sort.SliceStable(scripts, func(left, right int) bool {
		if scripts[left].start != scripts[right].start {
			return scripts[left].start < scripts[right].start
		}
		return scripts[left].end < scripts[right].end
	})
	result := make([]editScript, 0, len(scripts))
	lastEnd := -1
	for _, script := range scripts {
		if script.start < lastEnd {
			continue
		}
		result = append(result, script)
		lastEnd = script.end
	}
	return result
}

func expandASTInsertionRange(nodes map[gumast.NodeIdType]goast.Node, fileSet *token.FileSet, offset int, source []byte) (int, int) {
	bestStart, bestEnd := offset, offset
	bestSize := len(source) + 1
	for _, node := range nodes {
		nodeStart := nodeOffset(fileSet, node.Pos())
		nodeEnd := nodeOffset(fileSet, node.End())
		if nodeStart > offset || nodeEnd <= offset {
			continue
		}
		if size := nodeEnd - nodeStart; size < bestSize {
			bestStart, bestEnd, bestSize = nodeStart, nodeEnd, size
		}
	}
	return lineStart(source, bestStart), lineEnd(source, bestEnd)
}

func mergeOverlappingEditScripts(scripts []editScript, sourceBytes, targetBytes []byte) []editScript {
	sort.SliceStable(scripts, func(left, right int) bool {
		if scripts[left].start != scripts[right].start {
			return scripts[left].start < scripts[right].start
		}
		return scripts[left].targetStart < scripts[right].targetStart
	})

	merged := make([]editScript, 0, len(scripts))
	for _, script := range scripts {
		if len(merged) == 0 {
			merged = append(merged, script)
			continue
		}

		previous := &merged[len(merged)-1]
		sourceOverlaps := script.start <= previous.end
		targetOverlaps := script.targetStart <= previous.targetEnd
		if !sourceOverlaps || !targetOverlaps {
			merged = append(merged, script)
			continue
		}

		if script.end > previous.end {
			previous.end = script.end
			previous.baseEnd = script.end
		}
		if script.targetEnd > previous.targetEnd {
			previous.targetEnd = script.targetEnd
		}
		if previous.start == previous.end && script.start != script.end {
			previous.start, previous.end = script.start, script.end
			previous.baseStart, previous.baseEnd = previous.start, previous.end
		}
		if previous.targetStart == previous.targetEnd && script.targetStart != script.targetEnd {
			previous.targetStart, previous.targetEnd = script.targetStart, script.targetEnd
		}
		previous.kind = editUpdate
		if previous.start == previous.end {
			previous.kind = editInsert
		} else if previous.targetStart == previous.targetEnd {
			previous.kind = editDelete
		}
		previous.original = append([]byte(nil), sourceBytes[previous.start:previous.end]...)
		previous.replacement = append([]byte(nil), targetBytes[previous.targetStart:previous.targetEnd]...)
	}

	return merged
}

func expandASTRange(nodes map[gumast.NodeIdType]goast.Node, fileSet *token.FileSet, start, end int, source []byte) (int, int) {
	if start == end {
		return start, end
	}
	for end > start && (source[end-1] == ' ' || source[end-1] == '\t' || source[end-1] == '\n' || source[end-1] == '\r') {
		end--
	}

	bestStart, bestEnd := start, end
	bestSize := len(source) + 1
	for _, node := range nodes {
		nodeStart := nodeOffset(fileSet, node.Pos())
		nodeEnd := nodeOffset(fileSet, node.End())
		if nodeStart > start || nodeEnd < end {
			continue
		}
		if size := nodeEnd - nodeStart; size < bestSize {
			bestStart, bestEnd, bestSize = nodeStart, nodeEnd, size
		}
	}

	return lineStart(source, bestStart), lineEnd(source, bestEnd)
}

func lineStart(source []byte, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	lineBreak := bytes.LastIndexByte(source[:offset], '\n')
	if lineBreak < 0 {
		return 0
	}
	return lineBreak + 1
}

func lineEnd(source []byte, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(source) {
		return len(source)
	}
	lineBreak := bytes.IndexByte(source[offset:], '\n')
	if lineBreak < 0 {
		return len(source)
	}
	return offset + lineBreak + 1
}

type sourceLine struct {
	start int
	end   int
}

type lineEdit struct {
	sourceIndex   int
	targetIndex   int
	sourcePresent bool
	targetPresent bool
}

func buildLineEditScripts(source, target *gumast.Node, sourceBytes, targetBytes []byte) []editScript {
	sourceLines := splitSourceLines(sourceBytes)
	targetLines := splitSourceLines(targetBytes)
	common := make([][]int, len(sourceLines)+1)
	for index := range common {
		common[index] = make([]int, len(targetLines)+1)
	}
	for sourceIndex := len(sourceLines) - 1; sourceIndex >= 0; sourceIndex-- {
		for targetIndex := len(targetLines) - 1; targetIndex >= 0; targetIndex-- {
			if bytes.Equal(sourceBytes[sourceLines[sourceIndex].start:sourceLines[sourceIndex].end], targetBytes[targetLines[targetIndex].start:targetLines[targetIndex].end]) {
				common[sourceIndex][targetIndex] = common[sourceIndex+1][targetIndex+1] + 1
				continue
			}
			common[sourceIndex][targetIndex] = max(common[sourceIndex+1][targetIndex], common[sourceIndex][targetIndex+1])
		}
	}

	var edits []lineEdit
	sourceIndex, targetIndex := 0, 0
	for sourceIndex < len(sourceLines) || targetIndex < len(targetLines) {
		if sourceIndex < len(sourceLines) && targetIndex < len(targetLines) && bytes.Equal(
			sourceBytes[sourceLines[sourceIndex].start:sourceLines[sourceIndex].end],
			targetBytes[targetLines[targetIndex].start:targetLines[targetIndex].end],
		) {
			sourceIndex++
			targetIndex++
			continue
		}
		if sourceIndex < len(sourceLines) && (targetIndex == len(targetLines) || common[sourceIndex+1][targetIndex] >= common[sourceIndex][targetIndex+1]) {
			edits = append(edits, lineEdit{sourceIndex: sourceIndex, targetIndex: targetIndex, sourcePresent: true})
			sourceIndex++
			continue
		}
		edits = append(edits, lineEdit{sourceIndex: sourceIndex, targetIndex: targetIndex, targetPresent: true})
		targetIndex++
	}

	scripts := make([]editScript, 0, len(edits))
	for index := 0; index < len(edits); {
		first := edits[index]
		lastSource, lastTarget := first.sourceIndex, first.targetIndex
		sourcePresent, targetPresent := first.sourcePresent, first.targetPresent
		index++
		for index < len(edits) {
			next := edits[index]
			if next.sourceIndex != lastSource+1 || next.targetIndex != lastTarget+1 {
				break
			}
			lastSource, lastTarget = next.sourceIndex, next.targetIndex
			sourcePresent = sourcePresent || next.sourcePresent
			targetPresent = targetPresent || next.targetPresent
			index++
		}

		sourceStart, targetStart := first.sourceIndex, first.targetIndex
		sourceEnd, targetEnd := lastSource+1, lastTarget+1
		if !sourcePresent {
			sourceEnd = sourceStart
		}
		if !targetPresent {
			targetEnd = targetStart
		}

		start := len(sourceBytes)
		end := start
		if sourceStart < len(sourceLines) {
			start = sourceLines[sourceStart].start
		}
		if sourceEnd > 0 && sourceEnd <= len(sourceLines) {
			end = sourceLines[sourceEnd-1].end
		}
		replacement := []byte(nil)
		targetByteStart, targetByteEnd := len(targetBytes), len(targetBytes)
		if targetStart < targetEnd {
			targetByteStart = targetLines[targetStart].start
			targetByteEnd = targetLines[targetEnd-1].end
			replacement = append([]byte(nil), targetBytes[targetByteStart:targetByteEnd]...)
		}

		kind := editUpdate
		if !sourcePresent {
			kind = editInsert
		} else if !targetPresent {
			kind = editDelete
		}
		scripts = append(scripts, editScript{
			kind:        kind,
			source:      source,
			target:      target,
			start:       start,
			end:         end,
			baseStart:   start,
			baseEnd:     end,
			targetStart: targetByteStart,
			targetEnd:   targetByteEnd,
			original:    append([]byte(nil), sourceBytes[start:end]...),
			replacement: replacement,
			description: fmt.Sprintf("lines %d-%d", targetStart+1, targetEnd),
		})
	}
	return scripts
}

func splitSourceLines(source []byte) []sourceLine {
	if len(source) == 0 {
		return nil
	}
	lines := make([]sourceLine, 0, bytes.Count(source, []byte("\n"))+1)
	start := 0
	for start < len(source) {
		end := start + bytes.IndexByte(source[start:], '\n') + 1
		if end == start {
			end = len(source)
		}
		lines = append(lines, sourceLine{start: start, end: end})
		start = end
	}
	return lines
}

func caseOperator(node *gumast.Node) string {
	if node == nil {
		return "?"
	}
	if node.Label == "*ast.BasicLit" {
		return string(node.Value)
	}
	for _, child := range node.OrderedChildren() {
		if operator := caseOperator(child); operator != "?" {
			return operator
		}
	}
	return "?"
}

func applyEdit(source []byte, script editScript) ([]byte, error) {
	if script.start < 0 || script.end < script.start || script.end > len(source) {
		return nil, fmt.Errorf("invalid edit range %d:%d", script.start, script.end)
	}
	if !bytes.Equal(source[script.start:script.end], script.original) {
		return nil, fmt.Errorf("edit precondition failed at %d:%d", script.start, script.end)
	}

	replacement := script.replacement
	updated := make([]byte, 0, len(source)+len(replacement)-(script.end-script.start))
	updated = append(updated, source[:script.start]...)
	updated = append(updated, replacement...)
	updated = append(updated, source[script.end:]...)
	return updated, nil
}
