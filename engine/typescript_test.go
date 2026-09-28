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
	if got := strings.TrimSpace(state.Snapshot().RenderedCode); got != strings.TrimSpace(string(target)) {
		t.Fatalf("applied HTML = %q, want %q", got, strings.TrimSpace(string(target)))
	}
}
