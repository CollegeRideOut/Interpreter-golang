# Product Ideas

This file keeps the useful ideas discovered while exploring the project. They
are hypotheses, not promises or a fixed specification.

## Central Idea

contuts should help a person understand code evolution rather than receive an
opaque AI patch or an AI-generated architecture story.

```text
observable code facts
    -> human question
        -> related code and evidence
            -> proposed or applied edit
                -> resulting working state
                    -> next question
```

## Two Directions Meet

Exploration moves top-down:

```text
workspace -> package -> file -> declaration -> type/reference/call/dependent
```

Editing moves bottom-up:

```text
AST node -> parent -> declaration -> file -> working program
```

They meet at stable code identity, source evidence, and working-state revision.
The same declaration or AST node should connect navigation, source, edits,
relationships, and verification.

## Inquiry Paths

The UI should use unlimited reusable explorer columns, not a two-pane limit.
The leftmost column is the starting point. Each question opens an identical
explorer to the right.

```text
function
    -> return type
        -> type declaration
            -> fields
                -> field types
```

This applies equally to:

- Package imports.
- Same-package symbols in another file.
- Parameters and return types.
- Variables and fields.
- References.
- Callers and callees.
- Dependents and possible impact.

The originating item stays highlighted. The selected answer is highlighted in
the next column. A repeated target should say `Previously opened` and offer the
existing column or an explicit reopen. Unrelated questions should start another
path or row.

## Faithful Facts And Human Nuance

The canonical model must not begin with an agent inventing components,
boundaries, or architecture. It should expose facts:

```text
file belongs to package X
function returns type Y
identifier resolves to declaration Z
file imports package Q
same-package symbol is declared in another file
edit changed this AST node
```

Human meaning and AI hypotheses remain separate overlays:

```text
fact: Function A returns Type B
question: why does Function A return Type B?
hypothesis: this may be request validation
annotation: important checkout path
verified edit: Edit E changed Function A
```

Nuance is preserved without corrupting the facts. User labels, selected paths,
questions, assumptions, and AI hypotheses should be carried forward explicitly.

## Line Of Sight Through Edits

The key value may be walking through an edit even when the AI's proposal is
accepted exactly:

```text
AI proposal
    -> parent AST edit
        -> child edit
            -> changed declaration
                -> affected type or reference
                    -> next working revision
```

Applying an edit is both authorization and observation. Rejecting or changing a
child creates a branch from the chosen state. Every edit should retain a causal
thread containing its parent, source and destination identities, before and
after state, affected entities, revision, diagnostics, and verification.

Show evidence categories separately:

```text
changed directly
referenced by
potentially affected
verified by tests
not yet checked
```

Do not claim that every nearby file or an entire subsystem is affected without
evidence.

## Structural Contracts And Agent Loops

The engine could let a human state the desired shape of a result:

```text
Explore X and Y.
The top-level AST must export four functions.
Those functions must satisfy interface Z.
The implementation must call package Q from file R.
```

An AI or ACP-connected agent proposes bounded edits. The engine applies them to a
working revision and reports exact structural failures:

```text
required exports: 3 of 4
interface: missing Close
package call: wrong target
```

The agent loops until the contract is satisfied or the human stops it. The
engine judges observable conditions, not intent or semantic behavior. The
human owns the contract and accepts the resulting revision.

## Headless Boundary

The core should be usable without Angular:

```text
Angular / CLI / MCP / ACP / editor plugin
              |
      inquiry and transformation API
              |
 facts + analysis + working-state engine
```

Possible operations include:

```text
inspect(target)
related(target, relation)
source(target)
edits(revision, target)
apply(editID)
remove(editID)
verify(contract, revision)
```

Clients choose whether to render columns, graphs, timelines, trees, tables, or
editor decorations. The client must not define code semantics.

## Analysis Providers

- Go AST provides syntax, declarations, ranges, and incomplete-state handling.
- `go/types` or `golang.org/x/tools/go/packages` should provide symbol identity,
  types, references, calls, and same-package resolution.
- gopls/LSP can provide richer workspace analysis later.
- Tree-sitter may help incremental parsing or multi-language support later, but
  it does not replace semantic resolution.
- The existing AST edit engine remains the transformation substrate.

## Archify Comparison

Archify is a useful reference for typed IR, deterministic validation, source
evidence, stable IDs, and multiple viewer modes. Its repository workflow is
still fundamentally:

```text
agent analyzes or receives a description
    -> agent authors diagram IR
        -> deterministic validation and rendering
```

That is appropriate for a communication artifact, but the authored boundaries
are still an interpretation. contuts should not use an agent-authored diagram
as the canonical truth.

Archify asks:

```text
How should this system be communicated?
```

contuts asks:

```text
What can be observed here, what led me here, what does this edit affect,
and how do I want to continue from the resulting state?
```

## Product Test

The idea is earning its place when this can happen headlessly and visually:

```text
select a function
    -> open its return type
        -> open the type declaration
            -> inspect same-package references
                -> inspect callers
                    -> inspect related edits
                        -> reject or apply one edit
                            -> continue from the new revision
```

If the tool only produces a beautiful graph or a persuasive summary, it has
not solved the problem.

## Honest Status

This remains exploratory. The UI is the near-term steering surface. The engine
work is foundational and will take time to understand. No promise is made that
the final representation, analysis provider, or client architecture has been
settled.
