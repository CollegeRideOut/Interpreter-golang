# Direction

## Mission

`contuts` is a directed program-transformation environment.

The human chooses the target state of the codebase. AI helps construct transformations toward that state. `contuts` makes those transformations concrete, visible, reversible, and controllable.

The product is not primarily about summarizing AI output or recovering AI intent. Neither is reliably possible. The product is about controlling the transition from one program state to another.

## The Core Distinction

There is a major difference between these two workflows:

```text
Ask AI for a program
    -> receive a large patch
    -> receive a summary
    -> try to understand what happened
```

and:

```text
Choose a target state
    -> direct the next transformation
    -> inspect the edits being materialized
    -> accept, reject, or alter them
    -> observe the resulting state
```

The second workflow is the direction of the project.

The human does not need to understand the AI's private reasoning. The human needs to control what transformation is attempted and what state becomes current.

## Human Authority

AI may be more capable than the human at implementation. It is still below the human in authority.

```text
Human: chooses the target and constraints
AI: proposes or performs bounded implementation work
contuts: materializes and exposes the transformation
Human: decides whether the next state is accepted
```

The AI must not silently mutate the working state. “Fix this” should produce a proposed transformation. The user must be able to say yes, no, or do it differently.

## The Transformation Model

The first model should be concrete rather than a general-purpose rule language.

```text
select(anchor)
capture(subtree)
duplicate(subtree, destination)
replace(target, subtree)
move(target, destination)
wrap(target, wrapper)
delete(target)
substitute(edit, explicit bindings)
preview(edit)
apply(edit)
remove(edit)
```

The input and output of an operation are working-state revisions. An operation should record:

- Input working-state revision.
- Selected node or subtree.
- Destination and insertion mode.
- Explicit substitutions or renames.
- Materialized low-level edits.
- Output working-state revision.
- Diagnostics produced by the new state.

The operation makes no claim that behavior or intent is correct. It records what was requested and what was structurally produced.

## Concrete Replication

Replication is a central use case.

The user selects:

```go
if err != nil {
    return err
}
```

Then selects target locations in several functions. The system expands the request into separate instances:

```text
Transform: repeat selected IfStmt

Instance 1: insert into foo.Body[2]
Instance 2: insert into bar.Body[4]
Instance 3: insert into baz.Body[1]
```

The transform is a convenient parent. Each instance is an ordinary concrete edit with its own:

- Target.
- Substitutions.
- Before and after source.
- Diagnostics.
- Applied or rejected status.
- Provenance back to the selected fragment.

The user can apply instances 1 and 3, reject instance 2, and continue. A bulk operation must never hide partial success.

## Explicit Substitution And Renaming

Copied code must not be silently adapted.

When a selected fragment contains `err`, the target may contain `err`, `parseErr`, another value, or no suitable value. The system may detect collisions and suggest choices, but the mapping must be visible and explicit:

```text
source binding: err
target function: ParseFile
available candidates: err, parseErr
selected mapping: err -> parseErr
```

The initial system should support concrete substitutions and capture holes without requiring the user to author a formal template:

```text
selected fragment
    -> mark this expression as a hole
    -> fill it with a selected target node
    -> preview the concrete result
```

Semantic equivalence must not be assumed. A type-compatible rename is still a human decision.

## Placement

A destination is a concrete structural location, not a vague pattern.

The user should be able to choose:

- Insert before a node.
- Insert after a node.
- Prepend or append to a list.
- Replace a node.
- Wrap a node.
- Move a node to another parent.

Target queries may eventually select many locations, but the complete target set must be shown before application. The first implementation should prioritize manually selected destinations because they are easier to reason about and test.

## AI As A Subordinate Operator

AI is useful for finding and proposing transformations:

```text
compiler error or user request
    -> likely line or node
    -> user opens the relevant structure
    -> AI proposes a bounded transform
    -> user invokes or changes it
    -> contuts materializes the edits
```

For example, AI may identify a likely error node and suggest replacing one expression. It must not silently repair unrelated code or claim that its interpretation of the problem is true.

All AI-generated work must enter the same transformation system as human-created work. There should be no hidden AI edit path.

## Intermediate And Wrong States

Invalid states are part of directed construction.

The user may intentionally create:

- An incomplete declaration.
- A missing expression.
- A type mismatch.
- An unresolved identifier.
- A function in the wrong package.
- A broken import relationship.

The system should preserve and display these states. It should attach observations rather than erase the state:

```text
Structural state: incomplete
Parser state: failed at line 18
Type state: unavailable
Tests: not run
AI suggestion: replace this expression
```

These categories must remain separate:

- The transformation could not be materialized.
- The resulting structure is incomplete or invalid.
- The code parses but fails type checking.
- The code builds but tests fail.
- The AI proposal differs from the user's preference.

## Identity And Repetition

AST pointers and numeric indexes are not durable identities. Mutations can rebuild parents and shift list positions.

The system needs explicit lineage and working-tree instance identity:

```text
origin: where the selected fragment came from
instance: this occurrence in the current working state
```

A copied subtree receives a new instance identity while retaining provenance to its source. An edit also records its working-state revision and structural anchor.

If an anchor becomes stale or ambiguous, the edit becomes pending or conflicted. It must not be silently moved to a location that merely appears similar.

## Source Fidelity

The AST should be the structural transformation surface, but source remains the user-facing artifact.

The system should preserve untouched source bytes where possible and render only changed structural regions. Comments, formatting, and generated code need explicit policies. A source-text fallback may exist, but it must be labeled as a text operation rather than presented as a structurally safe transformation.

## Views

The user should be able to move between levels without losing the underlying edits:

```text
folder
    -> package
        -> file
            -> declaration
                -> AST node
                    -> token or field
```

Higher-level views are navigation and targeting surfaces. They are not authoritative summaries of what a change means. Every high-level operation must expand to visible low-level edits.

## What Not To Build First

Do not begin with:

- AI-generated summaries as the primary experience.
- A formal transformation language.
- Automatic intent inference.
- Automatic application to every similar location.
- Silent binding or rename decisions.
- Semantic-preservation claims.
- Hidden AI mutations.
- Mandatory validity gates.
- A graph that pretends to be the user's architecture.

These may become useful later, but they should earn their place by reducing the cost of directed transformation.

## Minimal Useful Experiment

Build one complete vertical slice:

1. Select a real AST subtree.
2. Capture it without losing provenance.
3. Select one or more concrete destinations.
4. Duplicate it into those destinations.
5. Show one independent materialized edit per destination.
6. Allow explicit substitutions and renames.
7. Preview before and after source.
8. Apply or reject each instance independently.
9. Permit invalid intermediate states.
10. Undo and continue from the chosen state.

The first demonstration should be a user copying a newly created `if` statement into several functions, changing the relevant identifier in each copy, and watching the program move through visible working states.

## Product Test

The product is working when a user can say:

```text
Put the program in this state.
Use AI to help construct the next transformation.
Show me every edit it creates.
No. Keep this part and change that part.
Now continue from the state I chose.
```

The product is not working if it only produces a persuasive summary after an opaque rewrite.

## Current Reality

The current repository is not this product yet. It is an AI-built, partially understood AST and working-state experiment. That makes it an appropriate place to explore the problem, but not evidence that the problem has been solved.

The project should keep the mechanics only if they lead to a better directed construction loop. Otherwise, we should be willing to call the experiment a gimmick and stop pretending that more AST controls alone will create value.
