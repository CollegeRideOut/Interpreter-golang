# Direction

## The Idea

`contuts` is an attempt to combine human agency with the power of AI.

The goal is not to make AI independently rewrite code or to make a passive code-review viewer. The goal is to give a person a structural way to explore code changes, choose what they want, and continue from the code state they created.

```text
AI proposes possibilities
Human explores and chooses
The system preserves those choices
AI continues from the chosen state
```

The user should be able to express intent without manually writing every piece of syntax:

```text
Keep this edit.
Remove that edit.
Show me what is affected.
Continue from the version I chose.
```

## What The App Is

The app is a structural code canvas combining:

- AST and structural diffs.
- Individually selectable edit scripts.
- Apply and remove operations.
- Undo and redo.
- Before and after views for each edit.
- A cumulative working code state.
- Related-code and affected-code context.
- Future OpenCode integration.

Applying an edit is not the same as accepting a code review. It is the user exploring and shaping a possible code state.

Removing an edit is not necessarily rejecting the entire proposal. It means the user wants to continue from a version without that transformation.

## Current Position

This project is deliberately very experimental and is still searching for the right representation of an editable program. The current milestone is intentionally smaller than a polished code canvas:

- Read the previous and current versions of a Go program.
- Generate complete, inspectable AST JSON for both versions.
- Generate an explicit structural edit script.
- Test the AST and edit model without depending on a UI.

The next implementation step is an AST-first intermediate program. The working state should be a mutable tree with explicit fields and lists, not a collection of source-text patches. An `ExprStmt`, `CallExpr`, `ImportSpec`, `AssignStmt`, or `IfStmt` should be independently representable even when its children are missing. Invalid intermediate states are acceptable and should remain inspectable.

The UI is not currently the source of truth. Raylib may eventually become a view over the AST state, but the AST model, edit operations, rendering behavior, and invariants must be understandable and testable without it.

The current command surface reflects that priority:

```text
go run .
    -> previousCommitAst.json
    -> currentCommitAst.json
    -> editScript.json

go run . --interactive
    -> numbered field-aware edit tree
    -> apply one AST operation at a time
    -> intermediateAst.json
```

The exported edit script should speak in terms of AST ownership and fields, such as `AssignStmt.Lhs[0]` or `GenDecl.Specs[0]`. Source positions and rendered text may be useful metadata, but they are not the edit model.

## Example Flow

```text
Original code
    |
    v
Structural edit scripts
    |
    +-- apply edit A
    +-- remove edit B
    +-- keep edit C
    v
Current working code
    |
    v
OpenCode continues from this state
```

Example:

```text
Applied:
- change subtraction to addition
- add multiplication
- add modulo

Removed:
- add division

Current state:
- calculator with +, -, *, %
```

The user should be able to hand this state to OpenCode and say:

> I kept these changes, removed those changes, and want to continue from here.

## Review And Exploration

The app should provide both kinds of visibility:

### Per-edit clarity

For the selected edit, show:

- What AST node changed.
- The exact before source.
- The exact after source.
- The file and location.
- The containing function or declaration.

### Cumulative exploration

Keep showing the working source as edits are applied and removed. This is the interactive part of the tool and should remain central, not hidden as a debug feature.

## Affected Code

The selected edit should expose closely related code in an easy-to-reach panel:

- Identifiers inside the changed node.
- Where those identifiers are defined.
- Other uses of those identifiers.
- The containing function.
- Functions called by the containing function.
- Functions that call the containing function.
- Related files.
- Top-level functions and declarations involved.
- Expression, parameter, and return types when available.

The AST diff tells us what syntax changed. Semantic analysis tells us what code may be related or affected.

## Folder Architecture View

The tool should eventually support a folder- and package-level view in addition to the AST and source views. A user may sometimes want to understand or shape the architecture of the codebase rather than inspect one function at a time.

This view could make structural operations such as these explicit:

- Move a function or declaration between files.
- Move files between packages or folders.
- Split a file into smaller files.
- Combine files when that improves cohesion.
- Rename or reorganize packages and directories.
- Show how moves affect imports, package boundaries, references, tests, and callers.

Folder changes are still code changes. The tool should show their consequences across the codebase, preserve the relationship between the architectural operation and the resulting AST edits, and keep the operation reversible. The folder view should be another perspective on the same working state, not a separate untracked editing system.

Keep these claims separate:

```text
AST diff        -> syntax changed
Type analysis   -> types changed or stayed the same
References      -> definitions and uses
Call analysis   -> callers and callees
Impact analysis -> code that may observe the change
```

## Principles

- Human direction comes first.
- AI should provide leverage, not remove agency.
- Every transformation should be inspectable.
- Every applied transformation should be reversible.
- The user should not need to apply an edit to understand it.
- The user should be able to apply an edit to explore its consequences.
- The current working state belongs to the user.
- Preserve provenance: remember where each working change came from.
- Prefer explicit facts over invented summaries.
- Optimize for understanding and control, not speed alone.
- Treat architecture as part of the program, not as irrelevant packaging.
- Do not hide codebase structure behind a black box merely because an AI can often produce a plausible result.

## AST-First Development

The project should earn its interactive interface from a reliable structural core. The development order is:

1. Inspect the complete source and target ASTs.
2. Inspect the generated field-aware edit operations.
3. Apply operations to an intermediate AST.
4. Test parent-only, child-only, deletion, and composition behavior.
5. Render or visualize the resulting state.

Raylib, a web interface, and OpenCode integration are observers and consumers of this state. They should not define the semantics of editing.

## Human Understanding

Human involvement is not automatically valuable just because it is human involvement. It is valuable when it helps a person understand what changed, what the code depends on, and what working state they are deliberately creating.

Fast automation is useful when the task is understood and the consequences are cheap to inspect. It is risky when speed replaces understanding of the codebase, its boundaries, and its architecture. Even a highly capable model can produce a locally plausible change that is globally wrong, misplaced, or difficult to maintain.

The goal is therefore not to force a human to approve every line. The goal is to preserve meaningful human control over structure, intent, and consequences while allowing automation to do the mechanical work. The right balance may change over time, but the product should measure success by the quality of understanding and resulting code state, not only by how quickly a patch appears.

## Next Steps

### Edit presentation

- [ ] Show exact before and after snippets for the selected edit.
- [ ] Highlight the selected edit in both source views.
- [ ] Keep each edit independently selectable.
- [ ] Show a clear applied/removed/current status.

### Working state

- [ ] Treat the working source as a first-class user state.
- [ ] Preserve original source, target source, and current source separately.
- [ ] Track applied, removed, and unapplied edits explicitly.
- [ ] Add named checkpoints for useful working states.
- [ ] Allow returning to a checkpoint.
- [ ] Preserve edit history and provenance when moving between states.

### Affected code

- [ ] Find the containing function and declaration for each edit.
- [ ] Collect identifiers from the affected AST node.
- [ ] Resolve identifier definitions and uses with `go/types`.
- [ ] Find direct callers and callees.
- [ ] Show related files and top-level functions.
- [ ] Display available parameter, expression, and return types.
- [ ] Add navigation from a related item back to its source location.

### OpenCode integration

- [ ] Define a serializable representation of the current working state.
- [ ] Include applied and removed edit history.
- [ ] Include the selected edit and affected-code context.
- [ ] Let the user send the current state to OpenCode.
- [ ] Support prompts such as: continue, explain, refactor, or generate the next edit.
- [ ] Make OpenCode aware of what the user deliberately kept and removed.

### Generalization

- [x] Replace hard-coded in-memory files with a real working-tree file.
- [x] Export the previous and current ASTs as inspectable JSON.
- [x] Export the generated edit script as inspectable JSON.
- [x] Make the AST/edit export the primary development path before returning to UI work.
- [x] Define a field-aware intermediate AST that permits missing children and invalid states.
- [x] Apply edit scripts to the intermediate AST rather than source byte ranges.
- [x] Add a terminal interactive mode for applying numbered AST operations.
- [ ] Render complete intermediate ASTs back to Go source.
- [ ] Provide best-effort rendering and diagnostics for incomplete ASTs.
- [ ] Support multi-file packages.
- [ ] Add a folder- and package-architecture view alongside the AST view.
- [ ] Represent file, folder, package, and declaration moves as inspectable operations.
- [ ] Show the imports, references, tests, and package boundaries affected by architectural moves.
- [ ] Improve insert, delete, update, and move edit scripts.
- [ ] Make source ranges robust for interacting edits.
- [ ] Keep the small calculator example as a reliable demonstration.

## The Core Test

The app should make this interaction feel natural:

```text
The AI suggests a change.
I inspect what it changes and what depends on it.
I keep some parts and remove others.
I see the code state I created.
I ask the AI to continue from that exact state.
```

That is the product direction: human agency, made more expressive by structural tools and AI assistance.
