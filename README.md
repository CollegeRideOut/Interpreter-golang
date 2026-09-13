# contuts

`contuts` is an exploratory project for directing AI-assisted construction of a codebase.

The human chooses the target state. AI helps find and build the path toward it. The system exposes the concrete transformations along the way so the human can apply, reject, repeat, alter, or undo them.

This is not primarily a code summary tool. It is not an attempt to recover the true intent of an AI-generated change. Intent is not reliably observable. A model's explanation is a claim, not a fact.

The product we are looking for is a way to say:

```text
Create this file.
Add this function here.
Take this if statement and put it in these functions.
Rename this value in that copy.
Let me see the exact edits as they are built.
```

## The Mission

AI can be faster and more capable than the person directing it. That does not make AI the owner of the product.

The human owns the target state and the sequence of decisions. AI is a subordinate planner and operator. It may propose a transformation or locate a likely error, but it should not silently decide what becomes part of the codebase.

```text
Human chooses a target
        |
        v
AI proposes a concrete transformation
        |
        v
Human selects, changes, or rejects it
        |
        v
contuts materializes visible edits
        |
        v
The program enters a new working state
        |
        v
Human chooses the next target
```

The goal is not to make the human approve every token. The goal is to keep the human first in the causal chain of program construction.

## An Honest Status Report

This repository is a gimmick today. It is also a serious exploratory experiment into whether a better interaction is possible.

Much of this codebase was built by AI as a black box. That is ironic because the black-box relationship is the problem this project is trying to fix. The current code proves interesting AST and working-state mechanics, but it is not yet the directed construction product described here.

The current UI exposes too much low-level machinery without a sufficiently useful construction loop. We should not pretend that displaying AST edits automatically gives a human control over architecture, behavior, or AI intent.

The project is valuable only if the mechanics become useful for directing transformations. If they do not, this remains a gimmick and ordinary agents, Git, tests, and an editor are better tools.

## Directed Transformations

The basic unit is a concrete transformation, not a summary.

Suppose the user selects this real piece of code:

```go
if err != nil {
    return err
}
```

The user chooses three destination functions. `contuts` expands the request into three independently visible operations:

```text
1. Insert this IfStmt into foo.Body[2]
2. Insert this IfStmt into bar.Body[4]
3. Insert this IfStmt into baz.Body[1]
```

Each result can be applied, rejected, moved, renamed, or undone independently.

The initial transformation model should be deliberately concrete:

```text
select a real node or subtree
capture that fragment
choose a destination
duplicate, replace, move, wrap, or delete
make substitutions explicitly
preview the resulting edits
apply or reject the operation
```

Generalized templates and pattern rules may come later. The user should not need to design a transformation language just to repeat a piece of code.

## AI As A Subordinate Operator

AI can locate a likely error or propose a repair:

```text
Compiler reports an error at line 18
        |
        v
AI points to a likely AST node
        |
        v
contuts shows the node and its context
        |
        v
AI proposes a bounded transformation
        |
        v
Human says yes, no, or do it differently
```

The proposal must remain inert until the human invokes it. A model saying “I fixed it” is not evidence that the problem was fixed. The system should show:

- The source location or node involved.
- The exact transformation proposed.
- Every low-level edit it would create.
- The resulting working state.
- Diagnostics and verification results.

It must distinguish observed facts, AI hypotheses, and verified outcomes. It must not claim to know what the AI really intended.

## Current Prototype

The codebase currently contains:

- Go AST export and field-aware structural diffs.
- Low-level insert, update, delete, and reconciliation experiments.
- A mutable intermediate working tree.
- Reversible application and removal of edits.
- Best-effort rendering of incomplete states.
- A Wails desktop UI for exploring structural edits.
- Lifted views over some low-level AST operations.
- Structural error markers for incomplete representations.

These are foundations, not proof of the final product. The current implementation is mainly a single-file Go structural editing experiment. It does not yet provide reliable multi-file transformations, AI proposal integration, durable branches, or the full directed workflow.

## Why Invalid States Matter

The user should be allowed to construct a program in a wrong or incomplete state.

```text
Create the file.
Create the declaration.
Leave the function incomplete.
Add the wrong type on purpose.
See what breaks.
Ask AI for a repair.
Apply only the repair chosen by the human.
```

Compilation, parsing, type checking, and tests are observations about a state. They should inform the next decision, not erase the state or prevent exploration unless the user explicitly asks for a constraint.

## Source, Structure, And Edits

The AST is the transformation substrate. Source is the human-facing result. Every high-level operation must expand into concrete, inspectable edits.

The system should preserve:

- The selected source fragment.
- Its structural representation.
- Its original provenance.
- Its destination.
- Explicit substitutions and renames.
- The before and after state.
- The operation that produced each edit.

A high-level row such as `repeat selected IfStmt in 3 functions` is only a convenient parent operation. The three materialized edits underneath it are the real result.

## Future Views

The product may eventually work from:

```text
folders -> packages -> files -> declarations -> AST nodes
```

This is not because a generated graph can tell us what the architecture truly means. It is because users may want to direct a transformation at the folder, package, file, declaration, or node level.

Every view must lead to the same working state and the same concrete edit history. A higher-level view may organize edits, but it must not hide or silently rewrite them.

## Running The Prototype

```bash
go run .
go run . --interactive
go build .
go test ./...
```

Build the desktop application:

```bash
cd desktop
wails build
```

The default example uses `TestProgram/main.go`. A repository directory can be supplied to compare its working-tree `main.go` with `HEAD~1`.

## Principles

- The human owns the target state.
- AI may propose, but does not own execution.
- A transformation is more important than its summary.
- Every meaningful operation must materialize into visible edits.
- Repetition creates independently controllable edit instances.
- Renames and substitutions must be explicit.
- Invalid intermediate states are allowed.
- Diagnostics are evidence, not authority.
- AI intent is a claim, never a fact.
- High-level views must remain connected to low-level edits.
- Operations must be inspectable, reversible, and replayable.
- The project must be honest about what it cannot know.

## The Product Test

The product is moving in the right direction when a user can say:

```text
Put the program in this state.
Use AI to help build the next step.
Show me exactly what it did.
No, keep this part and change that part.
Now continue from the state I chose.
```

That is the product we are searching for. The current repository is only the beginning.
