# Direction

## Mission

`contuts` is a directed code-exploration and program-transformation
environment. It should help a person understand how code relates, ask focused
questions about it, and deliberately move the working program from one state
to another.

The product is not a fixed tree, a two-pane dependency viewer, or an opaque AI
patch generator. It is a sequence of visible questions over a working codebase.

## The Inquiry Path

The primary interaction is an unbounded path of identical explorer columns:

```text
column 0: starting program or selected edit
    -> column 1: imported package
        -> column 2: selected file
            -> column 3: declaration
                -> column 4: callers, callees, dependents, or impact
```

The first column is the user's starting point. Each later column exists because
the user asked a question from the previous column. The rightmost column is
always available for the next question.

A column is a reusable explorer, not a special-purpose result panel. It should
be able to show the same package, file, declaration, source, import, API, and
relationship views regardless of how it was opened.

The path must preserve context:

- Highlight the source item that caused the next column to open.
- Highlight the selected answer in the new column.
- Show the relationship type, such as `imports`, `calls`, `used by`, or
  `affected by`.
- Keep the starting column visible while exploring to the right.
- Allow returning to an earlier column without losing the path.

## Cycles And Separate Questions

Code relationships are not guaranteed to form a tree. They can contain cycles,
shared dependencies, and repeated references.

The first useful behavior is not to invent a complex graph UI. It is to make
the path honest:

```text
target already exists earlier in this path
Previously opened at column 2
Open again anyway | Return to existing column
```

If the user asks an unrelated question, create a separate inquiry path or row.
Do not make unrelated files appear to be part of the current explanation.

## Relationship Facts

The engine should expose relationships as facts with explicit confidence and
origin where needed:

```text
import      -> this file imports that package
reference   -> this identifier resolves to that declaration
call        -> this function calls that function
dependent   -> this package or file uses the selected declaration
impact      -> this code may observe the selected change
```

These claims must stay separate from interpretation:

```text
AST diff        -> syntax changed
References      -> definitions and uses
Call analysis   -> callers and callees
Type analysis   -> type observations
Impact analysis -> possible affected code
Human label     -> an explicitly chosen interpretation
```

The tool may help the user explore relationships. It must not claim that a
generated graph is the true architecture of the system.

## Edit Evolution And Line Of Sight

The dependency path and edit history should meet at the selected code. When a
user explores an edit, show the relevant line of sight from broad operation to
concrete consequence:

```text
operation
    -> parent edit
        -> child edit
            -> affected declaration
                -> related file or package
                    -> resulting source and diagnostics
```

This is a linear story only for related work. Unrelated edits are excluded from
the story and shown in another lane or explorer.

An edit operation records:

- Input working-state revision.
- Selected node or subtree.
- Origin and current instance identity.
- Destination and insertion mode.
- Explicit substitutions or renames.
- Materialized low-level edits.
- Output working-state revision.
- Diagnostics produced by the new state.
- Relationships discovered from the changed code.

A high-level operation is only a grouping. Each materialized edit can be
applied, removed, inspected, or changed independently. Applying an edit should
update the working state and refresh the visible related-code path.

The point is not merely to catch bad AI output. Even when every proposed edit
is accepted, the user should be able to walk through how the program evolved:

```text
AI proposal
    -> parent AST edit
        -> child edit
            -> changed declaration
                -> affected type or reference
                    -> next working revision
```

Rejecting or changing a child creates a new branch from the current working
state. Applying an edit is both a user authorization and an observation of the
next state. The system should preserve a causal thread for each step:

- Edit and parent edit identity.
- Source and destination AST identities.
- Before and after state.
- Affected declarations and relationships.
- Working-state revision.
- Diagnostics and verification results.

The UI should distinguish `changed directly`, `referenced by`, `potentially
affected`, and `verified by tests`. A line of sight is an evidence chain, not a
claim that every nearby part of the codebase is impacted.

## Human Authority And AI

The human chooses the starting point, question, constraints, and accepted
working state. AI can help locate a node, propose an edit, or explain an
observation.

```text
human chooses a target or question
    -> AI proposes bounded work
        -> contuts exposes relationships and concrete edits
            -> human accepts, rejects, or changes the work
                -> continue from the chosen state
```

There must be no hidden AI edit path. A model's explanation is not evidence of
intent. The system should distinguish observed facts, AI hypotheses, requested
transformations, and verified results.

## Structural Core

The UI is a view over an authoritative structural engine. The initial operations
remain deliberately concrete:

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

The engine should preserve source fragment, AST structure, provenance,
destination, substitutions, before state, after state, and the operation that
produced each edit.

AST pointers and list indexes are not durable identities. Mutations can rebuild
parents and shift positions. Every working occurrence needs an instance
identity, while copied code retains provenance to its origin. Stale or
ambiguous anchors become pending or conflicted; they are never silently moved
to a merely similar location.

## Intermediate States

Wrong and incomplete states are part of exploration:

- An incomplete declaration.
- A missing expression.
- An unresolved identifier.
- A type mismatch.
- A broken import relationship.
- A declaration in the wrong package.

The tool should preserve these states and attach observations:

```text
Structural state: incomplete
Parser state: failed at line 18
Type state: unavailable
Tests: not run
AI suggestion: replace this expression
```

A failed transformation, invalid structure, type failure, build failure, test
failure, and disagreement with an AI proposal are different events.

## Implementation Order

The next UI architecture should be small and composable:

1. Extract one `ExplorerColumnComponent` from the current explorer markup.
2. Give a column its own target, selection, lens, source, and history state.
3. Render an array of columns instead of hard-coding primary and secondary panes.
4. Append a column when the user selects a relationship.
5. Preserve the originating selection and label the relationship edge.
6. Detect repeated targets and offer the existing column or an explicit reopen.
7. Add separate inquiry rows for unrelated questions.
8. Connect selected edits to affected-code relationships.
9. Render related parent-to-child edit evolution below the inquiry path.
10. Add deeper reference, call, type, and impact analysis only when it improves
    a real question.

The immediate development focus is the UI. The engine work already in the
repository is foundational and will take time to understand. The interface
should first become a useful instrument for steering and observing the existing
engine rather than requiring the whole engine to be redesigned at once.

Do not start by building a formal graph editor. The path should earn graph
features through use.

## What Not To Build First

- A two-pane limit.
- A special imported-source screen that duplicates explorer behavior.
- A graph that claims to know the user's architecture.
- Automatic intent inference.
- Silent substitutions or renames.
- Automatic edits at every similar location.
- AI-generated summaries as the primary experience.
- Hidden AI mutations.
- Mandatory validity gates.
- A formal transformation language before concrete transformations work.

## Product Test

The product is working when a user can start from a changed declaration and
ask:

```text
What imports this?
What does it call?
Who depends on it?
What else is affected by this edit?
Show me the related edit evolution.
Keep this part, remove that part, and continue from the resulting state.
```

The answer should be an explorable, honest path through code and concrete edits,
not a persuasive paragraph disconnected from the working program. The user
should be able to follow the evolution even when they agree with the AI, and
intervene at any point when they do not.

## Current Reality

The repository is still an experimental AST and working-state prototype. It
already has useful parsing, structural edit, package, file, and import
mechanics, but its Angular UI is being reorganized from special-cased panes to
reusable columns. Reference, call, type, impact, and durable branching support
remain future work.

The project should keep mechanics that help a person understand and direct a
working state. If more controls do not improve that loop, the project should be
willing to call the experiment a gimmick and change direction. There are no
promises yet; the UI is the place to steer the experiment and discover whether
the line of sight through code evolution is genuinely useful.
