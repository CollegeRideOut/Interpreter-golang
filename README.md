# contuts

`contuts` is an experimental AST exploration tool. It compares a previous Go program with the current working-tree program, exposes their structure as JSON, and lets a person apply field-aware AST edits from a terminal.

The project is intentionally back at basics. The AST model and edit operations are being tested before building a dependable graphical interface.

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
- Builds field-aware structural edit operations.
- Writes `previousCommitAst.json` for the `HEAD~1` AST.
- Writes `currentCommitAst.json` for the working-tree AST.
- Writes `editScript.json` for the generated edit script.

The AST JSON preserves relationships such as `AssignStmt.Lhs`, `AssignStmt.Rhs`, `GenDecl.Specs`, `CallExpr.Args`, and `BlockStmt.List`. The edit script describes AST operations, not source-text replacements.

## Interactive AST Mode

Run:

```bash
go run . --interactive
```

The terminal prints a numbered edit tree:

```text
[ ] 1 INSERT *ast.GenDecl at Decls[0]
  [ ] 2 INSERT *ast.ImportSpec at Specs[0]
    [ ] 3 INSERT *ast.BasicLit at Path[0]
```

Enter an edit number to apply it, or `q` to quit. The current mutable AST is written to `intermediateAst.json` after each selection.

Parent edits do not automatically materialize their children. Selecting a child attaches it to its AST field and creates the minimum missing parent path.

## Human Agency, With AI Leverage

The point is not to make AI independently rewrite code, or to make a passive code-review viewer. The point is to let a person inspect structure, choose AST operations, and continue from the state they deliberately created.

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

The default command exports the ASTs and edit script:

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
- `main.go`: command entrypoint and legacy UI code.
- `ast_export.go`: JSON AST and edit-script export.
- `ast_simple_diff.go`: field-aware structural AST comparison.
- `interactive.go`: terminal edit listing and intermediate AST application.
- `ast_draft.go`: experimental mutable AST draft model and renderer.
- `gumtree_diff_test.go`: directory-backed diff and edit-history tests.
- `third_party/gumtree-go`: a local GumTree fork used for AST comparison and mappings.

The local GumTree copy remains available for comparison experiments, but the
current JSON export and terminal workflow use the field-aware structural diff.

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
- Field-aware assignment edits through `BlockStmt.List`, `AssignStmt.Lhs`, and `AssignStmt.Rhs`.
- Parent-only and child-only intermediate AST operations.
- Interactive argument parsing and AST application.

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

- Make the AST JSON and edit-script export the primary testable workflow.
- Build a field-aware intermediate AST for parent-only and child-only edits.
- Allow incomplete and invalid intermediate AST states.
- Apply edits structurally instead of through source byte offsets.
- Support more than one file per directory.
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
