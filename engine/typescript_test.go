package engine_test

import (
	"strings"
	"testing"

	"interpreter/engine"
)

func TestParseTypeScriptBuildsEditableTree(t *testing.T) {
	root, err := engine.ParseTypeScript([]byte("const answer: number = 41;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if root.Kind != "typescript:program" {
		t.Fatalf("root kind = %q, want typescript:program", root.Kind)
	}
	if root.Language != "typescript" {
		t.Fatalf("root language = %q, want typescript", root.Language)
	}
	if got := engine.RenderBestEffort(root).Code; got != "const answer: number = 41;\n" {
		t.Fatalf("rendered source = %q", got)
	}
}

func TestProjectTypeScriptHidesSyntaxShells(t *testing.T) {
	state, err := engine.NewWorkingStateFromTypeScript(
		[]byte("const answer = 41;\n"),
		[]byte("function greet(name: string) { return name; }\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := state.Snapshot()
	hiddenKinds := map[string]bool{
		"typescript:program":           true,
		"typescript:formal_parameters": true,
		"typescript:statement_block":   true,
		"typescript:(":                 true,
		"typescript:)":                 true,
		"typescript:{":                 true,
		"typescript:}":                 true,
		"typescript::":                 true,
		"typescript:;":                 true,
	}
	views := engine.ProjectEdits(snapshot.Edits, engine.LiftOptions{HiddenKinds: hiddenKinds})
	visible := make(map[string]bool)
	for _, view := range views {
		visible[snapshot.Edits[view.EditIndex].NodeKind] = true
	}
	for kind := range hiddenKinds {
		if visible[kind] {
			t.Fatalf("syntax shell %q remained visible", kind)
		}
	}
	if !visible["typescript:function_declaration"] {
		t.Fatalf("semantic function declaration was lifted out of the projection")
	}
}

func TestTypeScriptImportDiffPreservesExistingIdentifiers(t *testing.T) {
	state, err := engine.NewWorkingStateFromTypeScript(
		[]byte("import { Kysely, SqliteDialect } from 'kysely';\n"),
		[]byte("import { Generated, Kysely, Migrator, SqliteDialect } from 'kysely';\n"),
	)
	if err != nil {
		t.Fatal(err)
	}

	updates := make(map[string]int)
	inserts := make(map[string]int)
	for _, edit := range state.Snapshot().Edits {
		if edit.NodeKind != "typescript:identifier" {
			continue
		}
		switch edit.Kind {
		case "UPDATE":
			updates[edit.Value]++
		case "INSERT":
			inserts[edit.Value]++
		}
	}
	if len(updates) != 0 {
		t.Fatalf("existing import identifiers were reported as updates: %v", updates)
	}
	if inserts["Generated"] == 0 || inserts["Migrator"] == 0 {
		t.Fatalf("missing inserted import identifiers: %v", inserts)
	}
}

func TestTypeScriptDiffAndApply(t *testing.T) {
	source := []byte("const answer: number = 41;\n")
	target := []byte("const answer: number = 42;\n")
	state, err := engine.NewWorkingStateFromTypeScript(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Snapshot().Edits) == 0 {
		t.Fatal("expected a TypeScript edit")
	}
	for index := range state.Snapshot().Edits {
		if err := state.Apply(index); err != nil {
			t.Fatalf("apply edit %d: %v", index, err)
		}
	}
	if got := strings.TrimSpace(state.Snapshot().RenderedCode); got != strings.TrimSpace(string(target)) {
		t.Fatalf("applied source = %q, want %q", got, strings.TrimSpace(string(target)))
	}
}

func TestParseTypeScriptSupportsCommonSyntax(t *testing.T) {
	source := []byte("import { readFile } from \"node:fs/promises\";\n\n" +
		"export interface User {\n\tname: string;\n}\n\n" +
		"export class Greeter {\n\tconstructor(private readonly prefix: string) {}\n\n" +
		"\tgreet(user: User): string {\n\t\treturn `${this.prefix}, ${user.name}`;\n\t}\n}\n\n" +
		"export const load = async (path: string): Promise<string> => {\n\treturn readFile(path, \"utf8\");\n};\n")
	root, err := engine.ParseTypeScript(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"typescript:import_statement",
		"typescript:interface_declaration",
		"typescript:class_declaration",
		"typescript:method_definition",
		"typescript:arrow_function",
		"typescript:template_string",
	} {
		if !containsNodeKind(root, want) {
			t.Errorf("parsed tree does not contain %q", want)
		}
	}
	rendered := engine.RenderBestEffort(root).Code
	if !strings.Contains(rendered, "import { readFile } from \"node:fs/promises\";") || !strings.Contains(rendered, "export class Greeter {") {
		t.Fatalf("TypeScript formatting lost structure: %q", rendered)
	}
}

func TestParseTSXBuildsEditableTree(t *testing.T) {
	root, err := engine.ParseTSX([]byte("export function App() { return <main>Hello</main>; }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if root.Kind != "tsx:program" || root.Language != "tsx" {
		t.Fatalf("TSX root = kind %q, language %q", root.Kind, root.Language)
	}
	if got := engine.RenderBestEffort(root).Code; got != "export function App() {\n  return <main>Hello</main>;\n}\n" {
		t.Fatalf("rendered TSX = %q", got)
	}
}

func TestTSXFormattingKeepsInlineTypeObjects(t *testing.T) {
	root, err := engine.ParseTSX([]byte("function App(){const [status,setStatus]=useState('Checking API...');useEffect(()=>{fetch('/api/health').then((response)=>response.json() as Promise<{status:string}>).then((data)=>setStatus(data.status));},[]);return <main><h1>Narativo</h1><p>A minimal React and Express starter.</p><small>Server: {status}</small></main>}\n"))
	if err != nil {
		t.Fatal(err)
	}
	rendered := engine.RenderBestEffort(root).Code
	if !strings.Contains(rendered, "Promise<{ status: string }>") || !strings.Contains(rendered, "  return <main>") {
		t.Fatalf("poorly formatted TSX: %q", rendered)
	}
}

func TestTSXFormattingSeparatesOptionalSemicolonStatements(t *testing.T) {
	root, err := engine.ParseTSX([]byte("function App(){const [status,setStatus]=useState('Checking API...')\nuseEffect(()=>{setStatus('ready')},[])\nreturn <main>{status}</main>}\n"))
	if err != nil {
		t.Fatal(err)
	}
	rendered := engine.RenderBestEffort(root).Code
	if !strings.Contains(rendered, "useState('Checking API...');\n  useEffect") {
		t.Fatalf("rendered statements were not separated: %q", rendered)
	}
	if !strings.Contains(rendered, "}, []);\n  return <main>") {
		t.Fatalf("rendered return statement was not separated: %q", rendered)
	}
}

func TestTSXFormattingSeparatesStatementsInsideCallbackArguments(t *testing.T) {
	root, err := engine.ParseTSX([]byte("app.get('/api/health', async (_request, response) => { await db.selectNoFrom(({ val }) => val(1).as('ok')).execute()\nresponse.json({ status: 'ok' }) })\n"))
	if err != nil {
		t.Fatal(err)
	}
	rendered := engine.RenderBestEffort(root).Code
	if !strings.Contains(rendered, ".execute();\n  response.json") {
		t.Fatalf("nested callback statements were not separated: %q", rendered)
	}
}

func TestTypeScriptDiffLanguageDispatch(t *testing.T) {
	source := []byte("const value = 1;\n")
	target := []byte("const value = 2;\n")
	for _, language := range []string{"typescript", "ts"} {
		_, _, edits, err := engine.DiffLanguage(language, source, target)
		if err != nil {
			t.Fatalf("DiffLanguage(%q): %v", language, err)
		}
		if len(edits) == 0 {
			t.Fatalf("DiffLanguage(%q) returned no edits", language)
		}
	}
	if _, err := engine.ParseLanguage("ruby", source); err == nil {
		t.Fatal("expected unsupported language error")
	}
}

func TestTypeScriptApplyAndRemoveRoundTrip(t *testing.T) {
	source := []byte("const answer: number = 41;\n")
	target := []byte("const answer: number = 42;\n")
	state, err := engine.NewWorkingStateFromLanguage("typescript", source, target)
	if err != nil {
		t.Fatal(err)
	}
	editCount := len(state.Snapshot().Edits)
	for index := 0; index < editCount; index++ {
		if err := state.Apply(index); err != nil {
			t.Fatalf("apply edit %d: %v", index, err)
		}
	}
	if got := state.Snapshot().RenderedCode; got != string(target) {
		t.Fatalf("applied source = %q, want %q", got, target)
	}
	for index := editCount - 1; index >= 0; index-- {
		if err := state.Remove(index); err != nil {
			t.Fatalf("remove edit %d: %v", index, err)
		}
	}
	if got := state.Snapshot().RenderedCode; got != string(source) {
		t.Fatalf("restored source = %q, want %q", got, source)
	}
}

func TestTypeScriptDiffIncludesInsertedDeclaration(t *testing.T) {
	source := []byte("function greet(name: string): string {\n\treturn name;\n}\n")
	target := []byte("function greet(name: string): string {\n\tconst message = `Hello ${name}`;\n\treturn message;\n}\n")
	_, _, edits, err := engine.DiffTypeScript(source, target)
	if err != nil {
		t.Fatal(err)
	}
	inserted := false
	for _, edit := range edits {
		if edit.Kind == "INSERT" && edit.NodeKind == "typescript:lexical_declaration" {
			inserted = true
			break
		}
	}
	if !inserted {
		t.Fatalf("expected inserted lexical declaration, edits = %+v", edits)
	}
}

func TestTypeScriptParameterEditHidesSyntaxAndAppliesCompleteParameter(t *testing.T) {
	source := []byte("function greet(name: string) { return name; }\n")
	target := []byte("function greet(name: string, age: number) { return name; }\n")
	state, err := engine.NewWorkingStateFromTypeScript(source, target)
	if err != nil {
		t.Fatal(err)
	}
	parameterIndex := -1
	for index, edit := range state.Snapshot().Edits {
		if edit.NodeKind == "typescript:required_parameter" {
			parameterIndex = index
		}
		if edit.Hidden {
			if edit.NodeKind != "typescript:," && edit.NodeKind != "typescript::" {
				t.Fatalf("unexpected hidden edit %q", edit.NodeKind)
			}
		}
	}
	if parameterIndex < 0 {
		t.Fatal("expected inserted parameter edit")
	}
	if err := state.ApplyProjected(parameterIndex, engine.ApplyOptions{Reconcile: true}, engine.LiftOptions{}); err != nil {
		t.Fatal(err)
	}
	if report := state.Validate(); !report.Valid {
		t.Fatalf("projected parameter is invalid: %v\n%s", report.Diagnostics, state.Snapshot().RenderedCode)
	}
	if got := state.Snapshot().RenderedCode; !strings.Contains(got, "name: string, age: number") {
		t.Fatalf("complete parameter syntax was not applied:\n%s", got)
	}
}

func TestProposalOperationTransfersParameterToHumanBuild(t *testing.T) {
	base := []byte("function greet(name: string) { return name; }\n")
	proposal := []byte("function greet(name: string, age: number) { return name; }\n")
	proposalState, err := engine.NewWorkingStateFromTypeScript(base, proposal)
	if err != nil {
		t.Fatal(err)
	}
	humanState, err := engine.NewWorkingStateFromTypeScript(base, base)
	if err != nil {
		t.Fatal(err)
	}
	index := -1
	for editIndex, edit := range proposalState.Snapshot().Edits {
		if edit.NodeKind == "typescript:required_parameter" {
			index = editIndex
			break
		}
	}
	if index < 0 {
		t.Fatal("expected proposal parameter edit")
	}
	operations, err := proposalState.ProposalOperationsForSubtree(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := humanState.ApplyProposalOperations(operations); err != nil {
		t.Fatal(err)
	}
	if report := humanState.Validate(); !report.Valid {
		t.Fatalf("human build is invalid: %v\n%s", report.Diagnostics, humanState.Snapshot().RenderedCode)
	}
	if got := humanState.Snapshot().RenderedCode; !strings.Contains(got, "name: string, age: number") {
		t.Fatalf("proposal parameter was not transferred:\n%s", got)
	}
}

func TestTypeScriptAppliedInterfacePropertiesRemainSeparated(t *testing.T) {
	source := []byte("interface Schema {\n  health: { id: number };\n}\n")
	target := []byte("interface Schema {\n  health: { id: number };\n  books: {\n    id: Generated<number>;\n    author: string;\n    title: string;\n    path: string;\n  };\n}\n")
	state, err := engine.NewWorkingStateFromLanguage("typescript", source, target)
	if err != nil {
		t.Fatal(err)
	}
	for index := range state.Snapshot().Edits {
		if err := state.Apply(index); err != nil {
			t.Fatalf("apply edit %d: %v", index, err)
		}
	}
	if report := state.Validate(); !report.Valid {
		t.Fatalf("applied interface properties are invalid: %v\n%s", report.Diagnostics, state.Snapshot().RenderedCode)
	}
	if got := state.Snapshot().RenderedCode; !strings.Contains(got, "Generated<number>;\n    author: string;") {
		t.Fatalf("interface properties were not separated:\n%s", got)
	}
}

func TestValidateTypeScriptWorkingState(t *testing.T) {
	state, err := engine.NewWorkingStateFromTypeScript(
		[]byte("const value = 1;\n"),
		[]byte("const value = 2;\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if report := state.Validate(); !report.Valid {
		t.Fatalf("valid TypeScript reported invalid: %v", report.Diagnostics)
	}
	if _, err := engine.ParseTypeScript([]byte("const = ;\n")); err == nil {
		t.Fatal("expected invalid TypeScript parse error")
	}
}

func containsNodeKind(root *engine.Node, want string) bool {
	if root == nil {
		return false
	}
	if root.Kind == want {
		return true
	}
	for _, child := range root.Children {
		if containsNodeKind(child, want) {
			return true
		}
	}
	return false
}

func TestHTMLDiffAndApply(t *testing.T) {
	source := []byte("<main><h1>Hello</h1></main>\n")
	target := []byte("<main><h1>Welcome</h1></main>\n")
	state, err := engine.NewWorkingStateFromLanguage("html", source, target)
	if err != nil {
		t.Fatal(err)
	}
	for index := range state.Snapshot().Edits {
		if err := state.Apply(index); err != nil {
			t.Fatalf("apply edit %d: %v", index, err)
		}
	}
	want := "<main>\n  <h1>Welcome</h1>\n</main>"
	if got := strings.TrimSpace(state.Snapshot().RenderedCode); got != want {
		t.Fatalf("applied HTML = %q, want %q", got, want)
	}
}

func TestHTMLInsertionsApplyFromEmptySource(t *testing.T) {
	state, err := engine.NewWorkingStateFromLanguage("html", nil, []byte("<html><body><div id=\"root\"></div></body></html>\n"))
	if err != nil {
		t.Fatal(err)
	}
	for index := range state.Snapshot().Edits {
		if err := state.Apply(index); err != nil {
			t.Fatalf("apply HTML edit %d: %v", index, err)
		}
	}
	want := "<html>\n  <body>\n    <div id=\"root\"></div>\n  </body>\n</html>"
	if got := strings.TrimSpace(state.Snapshot().RenderedCode); got != want {
		t.Fatalf("applied HTML insertions = %q", got)
	}
}

func TestHTMLElementEditHidesTagDelimiters(t *testing.T) {
	state, err := engine.NewWorkingStateFromLanguage("html", nil, []byte("<html><body><div id=\"root\"></div></body></html>\n"))
	if err != nil {
		t.Fatal(err)
	}
	elementIndex := -1
	hiddenStartTag := false
	hiddenEndTag := false
	for index, edit := range state.Snapshot().Edits {
		if edit.NodeKind == "html:element" && edit.Node != nil && edit.Node.StartByte == 0 {
			elementIndex = index
		}
		if edit.NodeKind == "html:start_tag" {
			hiddenStartTag = edit.Hidden
		}
		if edit.NodeKind == "html:end_tag" {
			hiddenEndTag = edit.Hidden
		}
		if edit.Hidden && edit.NodeKind != "html:<" && edit.NodeKind != "html:>" && edit.NodeKind != "html:/" && edit.NodeKind != "html:=" && edit.NodeKind != "html:start_tag" && edit.NodeKind != "html:end_tag" && edit.NodeKind != "html:tag_name" {
			t.Fatalf("unexpected hidden HTML edit %q", edit.NodeKind)
		}
	}
	if elementIndex < 0 {
		t.Fatal("expected inserted HTML element edit")
	}
	if !hiddenStartTag || !hiddenEndTag {
		t.Fatal("expected HTML start and end tags to be renderer-owned")
	}
	if err := state.ApplyProjected(elementIndex, engine.ApplyOptions{Reconcile: true}, engine.LiftOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := state.Snapshot().RenderedCode; !strings.Contains(got, "<html>") || !strings.Contains(got, "</html>") {
		t.Fatalf("complete HTML element was not applied:\n%s", got)
	}
}

func TestHTMLSubtreeRemovalSurvivesAppliedParent(t *testing.T) {
	target := []byte("<html lang=\"en\">\n  <head>\n    <title>Narativo</title>\n  </head>\n  <body>\n    <div id=\"root\"></div>\n  </body>\n</html>\n")
	state, err := engine.NewWorkingStateFromLanguage("html", nil, target)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := state.Snapshot()
	var parent, child *engine.Edit
	for index := range snapshot.Edits {
		edit := snapshot.Edits[index]
		if edit.NodeKind == "html:element" && edit.Node != nil && edit.Node.StartLine == 1 {
			copy := edit
			parent = &copy
		}
		if edit.NodeKind == "html:element" && edit.Node != nil && edit.Node.StartLine == 2 && edit.Node.EndLine == 4 {
			copy := edit
			child = &copy
		}
	}
	if parent == nil || child == nil {
		t.Fatalf("could not find HTML parent and child edits")
	}
	if err := state.ApplyProjectedSubtree(parent.Index, engine.ApplyOptions{Reconcile: false}, engine.LiftOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := state.RemoveProjectedSubtree(child.Index, engine.LiftOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, node := range state.Snapshot().Root.Children {
		if node.Kind == "html:element" && len(node.Children) == 0 {
			t.Fatalf("expected remaining HTML body subtree")
		}
	}
	if got := state.Snapshot().RenderedCode; strings.Contains(got, "<head>") {
		t.Fatalf("removed HTML subtree remained rendered: %q", got)
	}
}
