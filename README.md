# contuts

`contuts` is an exploratory code-understanding and program-transformation tool.
It is being built around one question:

> Can a person understand and direct code changes by exploring the code state,
> the relationships around it, and the concrete edits that move it forward?

The project is intentionally being built as a bad, useful version first. The
goal is not to predict the perfect architecture before using the tool. The goal
is to discover an interaction that makes code evolution easier to see.

## The Core Experience

The main UI is an inquiry path made from identical explorer columns:

```text
starting code
    -> imported package
        -> file
            -> declaration
                -> callers, callees, dependents, or related code
```

The path is not limited to two columns. The leftmost column is the starting
point. Every question opens another copy of the same explorer to the right.
Selecting an import, dependent, caller, callee, or affected declaration extends
the path rather than replacing the current context.

Every column should support the same lenses:

- Packages and files.
- Declarations and exported API.
- Source code.
- Imports and references.
- Callers and callees when analysis is available.
- Dependents and affected code when analysis is available.
- Structural AST details and exact edit context.

The user should always be able to tell why a column exists. The originating
item remains highlighted, and the selected item in the next column is
highlighted as the answer to that question.

Loops are expected in real code. A repeated target should be marked as
`Previously opened` instead of silently creating a confusing cycle. Unrelated
questions should start a separate inquiry path or explorer row rather than
polluting the current one.

## Edit Evolution

Dependency exploration and edit exploration are connected, but they are not
the same view.

When edits are applied, the tool should show the relevant evolution as a clear
sequence:

```text
parent edit
    -> child edit
        -> affected declaration
            -> resulting source
                -> next related change
```

The sequence should follow the related parent-to-child chain. Completely
unrelated edits should not appear in the current story; they belong in another
edit lane or inquiry path.

Each edit remains independently controllable. A parent operation is only a
convenient grouping. The materialized child edits are the actual operations and
can be applied, removed, inspected, or changed separately.

## Human Direction, AI Leverage

The human chooses the target state and the next question. AI can locate code,
propose a transformation, or explain an observed relationship, but it must not
silently decide what becomes part of the working state.

```text
Human chooses a target or question
        |
        v
AI proposes a bounded operation or analysis
        |
        v
contuts shows the relationship and concrete edits
        |
        v
Human accepts, rejects, changes, or continues the inquiry
        |
        v
The working program enters a chosen state
```

The product is not primarily an AI summary viewer. A summary is a claim. The
useful artifact is the visible path from one program state to another:

- The source location or structural node.
- The relationship that caused it to be shown.
- The exact edit or proposed edit.
- The resulting working source.
- Diagnostics and verification results.
- Provenance back to the original fragment and operation.

## Current Prototype

The repository currently contains experiments for:

- Go AST export and field-aware structural diffs.
- Low-level insert, update, delete, and reconciliation operations.
- A mutable intermediate working tree.
- Reversible application and removal of edits.
- Best-effort rendering of incomplete states.
- Package, file, declaration, and local-import exploration.
- A Wails desktop UI.
- Lifted views over some low-level AST operations.
- Structural error markers for incomplete representations.
- A multi-package Go fixture for dependency exploration.

This is not yet a dependable multi-file transformation environment. Reference,
call, type, and impact analysis are future capabilities. The current UI is a
prototype and is being reshaped around reusable explorer columns rather than
special-cased left and right panes.

## Structural Transformation Model

The first transformation model should stay concrete:

```text
select(anchor)
capture(subtree)
replace(target, subtree)
move(target, destination)
wrap(target, wrapper)
substitute(edit, explicit bindings)
preview(edit)
apply(edit)
remove(edit)
```

AST pointers and numeric indexes are not durable identities. Operations need
lineage, working-state revisions, structural anchors, and provenance. A copied
subtree receives a new instance identity while retaining its origin.

The system must distinguish:

```text
AST diff        -> syntax changed
References      -> definitions and uses
Call analysis   -> callers and callees
Type analysis   -> type observations
Impact analysis -> code that may observe the change
```

None of these observations should pretend to be the user's architecture or the
true intent of an AI system.

## Why Invalid States Matter

The user may intentionally create an incomplete or wrong state in order to
understand it:

```text
Create the file.
Create the declaration.
Leave the function incomplete.
See what breaks.
Ask for a bounded repair.
Apply only the repair chosen by the human.
```

Parsing, compilation, type checking, and tests are observations about a state.
They should inform the next question instead of erasing the state.

## Running It

Run the Go prototype:

```bash
go run .
go run . --interactive
go test ./...
```

Build or test the desktop application:

```bash
cd desktop
go test ./...
wails build
```

The default example uses `TestProgram/main.go`. `TestProgramCalorieApp/` is a
multi-package fixture for exploring package and import relationships.

## Principles

- The human owns the target state and the starting point of an inquiry.
- Every explorer column uses the same code and behavior.
- A question extends to the right; it does not destroy the question that led to it.
- Previously opened targets must be visible when loops occur.
- Unrelated code belongs in a separate path or lane.
- AI may propose, but does not own execution.
- Every meaningful operation materializes into visible edits.
- Repetition creates independently controllable edit instances.
- Renames and substitutions are explicit.
- Invalid intermediate states are allowed.
- Diagnostics are evidence, not authority.
- Operations are inspectable, reversible, and replayable.
- High-level views remain connected to low-level edits.
- The project must be honest about what it cannot know.

## Product Test

The product is moving in the right direction when a user can say:

```text
Start here.
Why does this depend on that?
Show me the next code in the path.
What does this edit affect?
Show me the edit evolution.
Keep this change, remove that one, and continue from here.
```

If the user can see the code, the relationships, and the chosen working states
well enough to ask better questions, contuts is doing useful work.

The older project notes are preserved in [`oldREADME.md`](oldREADME.md),
[`oldDIRECTION.md`](oldDIRECTION.md), and
[`INTERFACE_IDEAS.md`](INTERFACE_IDEAS.md).
