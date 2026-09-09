# contuts

> A tiny structural code canvas for people who looked at a four-line diff and decided they needed an AST editor.

`contuts` is an experimental Go project for exploring whether a program can explain and transform a code change using deterministic evidence instead of immediately asking an AI to summarize or rewrite it.

It currently exports the compared Go ASTs and their edit script as JSON so the structural model can be tested independently of a UI.

This is intentionally an experimental foundation. The project is currently searching for the right AST and edit representation before building a dependable interactive interface. The JSON artifacts are the source of inspection for now; Raylib is not the source of truth.

## What It Does Today

The program compares two versions of `main.go` in a directory:

```go
// before
case '+':
    return left - right
case '-':
    return left - right
```

```go
// after
case '+':
    return left + right
case '-':
    return left - right
case '*':
    return left * right
case '/':
    return left / right
case '%':
    return left % right
```

It then:

- Parses both versions into Go ASTs.
- Converts those ASTs into GumTree-style trees.
- Builds a small structural edit script containing one update and three insertions.
- Writes `previousCommitAst.json` for the `HEAD~1` AST.
- Writes `currentCommitAst.json` for the working-tree AST.
- Writes `editScript.json` for the generated edit script.

The current output is intentionally structural rather than a finished program editor. The next goal is an intermediate AST that can represent parent nodes without their children, child nodes without all surrounding syntax, and invalid exploratory states without losing information.

The UI is intentionally focused on the original AST. Inserted cases do not magically become nodes in the original tree; they appear as separate edit rows attached to the affected source node instead.

## Human Agency, With AI Leverage

The point is not to make AI independently rewrite code, or to make a passive code-review viewer. The point is to let a person explore structural changes, choose what they want, and continue from the code state they created.

```text
AI proposes possibilities
Human explores and chooses
The system preserves those choices
AI continues from the chosen state
```

Applying and removing edits are therefore intentional parts of the interaction. They are not accept/reject buttons for a review; they are a way to express intent without manually rewriting syntax. The user can keep one transformation, remove another, inspect the consequences, and treat the resulting working source as their current state.

The longer-term flow is:

```text
structural diff
    -> user applies and removes edits
    -> current working code
    -> OpenCode continues from that exact state
```

See [`DIRECTION.md`](DIRECTION.md) for the working product direction and next-step list.

## Running It

This is a Go project. The current command is an AST export step.

```bash
go run .
```

By default this reads `TestProgram/main.go`. Pass another directory to compare
that directory's working-tree `main.go` with its `HEAD~1` version:

```bash
go run . ./path/to/repository
```

The command writes `previousCommitAst.json`, `currentCommitAst.json`, and
`editScript.json` in the current directory. The working-tree file is the
target. The previous AST comes from `HEAD~1`.

Build it with:

```bash
go build .
```

## Architecture, Such As It Is

The interesting prototype code currently lives in:

- `gumtree_diff.go`: directory loading, Git revision reading, Go AST conversion, edit-script generation, source ranges, and edit history.
- `main.go`: command entrypoint and the legacy UI implementation.
- `ast_export.go`: JSON AST and edit-script export.
- `ast_draft.go`: experimental mutable AST draft model and renderer.
- `gumtree_diff_test.go`: directory-backed diff and edit-history tests.
- `third_party/gumtree-go`: a local GumTree fork used for AST comparison and mappings.

The local GumTree copy is used through this module replacement:

```go
replace github.com/Xanonymous-GitHub/gumtree-go => ./third_party/gumtree-go
```

The fork currently supplies mappings and comparison support. The edit script shown by the UI is still custom code. This distinction matters because saying "GumTree generated the edit script" would be more impressive than accurate.

## Tests

Run all tests with:

```bash
go test ./...
```

The tests currently cover:

- The expected directory-backed diff and edit replay.
- The generated source matching the target after all edits.
- Independent AST rows for each edit.
- Basic update undo/redo.
- Repeated apply, undo, and redo cycles for all generated edits.

## Related Code

The selected edit should eventually show more than the changed syntax. A nearby context panel should make it easy to inspect:

- The containing function or declaration.
- Identifiers inside the changed node.
- Where those identifiers are defined and used.
- Direct callers and callees.
- Related files and top-level functions.
- Available expression, parameter, and return types.

The AST diff tells us what syntax changed. Semantic analysis tells us what code may be related or affected. Those claims should remain separate:

```text
AST diff        -> syntax changed
Type analysis   -> types changed or stayed the same
References      -> definitions and uses
Call analysis   -> callers and callees
Impact analysis -> code that may observe the change
```

## Why This Exists

A text diff can tell us that this happened:

```diff
- return left - right
+ return left + right
```

An AST-oriented tool can describe syntax updates and insertions in a way that is useful evidence. It is not intent, and it does not prove that the resulting program is correct.

That boundary is the point of the experiment:

```text
text diff       -> bytes changed
AST diff        -> syntax changed
symbol analysis -> relationships changed or affected
control flow    -> paths that may change
human judgment  -> what the change means
```

The project wants to explore the first four layers before pretending the fifth can be automated by putting a chatbot in a panel. Later, OpenCode should be able to use the user-created working state, including which edits were kept and removed, rather than starting over and guessing intent.

## Why Human Understanding Matters

Speed is useful, but speed alone is not the product. A fast change is not necessarily a good change if it leaves the person with less understanding of the codebase, its dependencies, or its architecture.

Architecture is not decoration. Folder boundaries, package boundaries, declarations, references, and call relationships affect how code behaves and how safely it can evolve. Treating the codebase as a black box is risky even when the system producing changes is highly capable.

The project therefore keeps a human in the loop where human judgment adds real value: understanding structure, choosing consequences, and deciding what state to continue from. This is not a demand that a person manually approve every token. It is an attempt to make automated changes inspectable, reversible, and grounded in the structure of the program.

## The Roadmap, In The Most Technically Honest Order

- Support more than one file per directory.
- Make the AST JSON and edit-script export the primary testable workflow.
- Build a field-aware intermediate AST for parent-only and child-only edits.
- Allow incomplete and invalid intermediate AST states.
- Apply edits structurally instead of through source byte offsets.
- Render valid intermediate ASTs back to Go.
- Provide best-effort source and diagnostics for invalid intermediate ASTs.
- Generate more complete insert, delete, update, and move scripts.
- Improve mappings when AST children are inserted or reordered.
- Make source ranges robust for overlapping and interacting edits.
- Add proper structural visualization instead of colored text rows.
- Add a folder and package architecture view.
- Represent file, folder, package, and declaration moves explicitly.
- Show the imports, references, tests, callers, and package boundaries affected by architectural changes.
- Integrate OpenCode only after the structural state and provenance model are reliable.
- Add symbols, references, call graphs, control-flow, and data-flow analysis.
- Add affected-code context for selected edits.
- Preserve named working states and edit provenance.
- Let OpenCode continue from the user's current code state.
- Eventually become useful.

The last item is aspirational.

## Current Limitations

This is not yet:

- A general-purpose Go diff tool.
- A conventional code editor.
- A persistent review application.
- A complete GumTree implementation.
- An interpreter-driven semantic analyzer.
- An AI replacement.

It is a deliberately small laboratory for finding out how much expressive power and complexity appears when you let a user independently apply and reverse structural edits.

That complexity is the feature. It is the bug we are studying.

## The Larger Rabbit Hole

The long-term idea is to move from:

```text
source change
    -> AST mapping
    -> edit script
    -> affected symbols
    -> callers and callees
    -> control-flow impact
    -> data-flow impact
```

Until then, `contuts` is mostly an excuse to get unreasonably nerdy about trees, source offsets, and undo stacks.
