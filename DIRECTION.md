# Direction

## Mission

`contuts` should return meaningful ownership and agency to the human in AI-assisted software development.

AI is powerful at generating possibilities, exploring implementation details, and performing mechanical work. The human is responsible for the direction of the system: what it should become, how its boundaries should evolve, and which tradeoffs are acceptable.

The product should maximize human involvement where involvement creates understanding and direction, while allowing AI to do the work that machines are good at.

## The Honest Starting Point

The current codebase is an ironic experiment. Much of it was built by AI as a black box, with the human observing and correcting behavior after the fact. That is the failure mode this project is intended to address.

The current prototype proves that structural edits and an intermediate working state are possible. It does not yet provide a genuinely useful directed workflow. The UI can feel like a gimmick because it exposes many mechanical operations without yet giving the human enough meaningful control over the larger change.

This is not a reason to hide the mechanics. It is a reason to put the mechanics in service of a better interaction.

## Product Thesis

The human should not have to choose between:

- Manually controlling every token and losing the speed of AI.
- Accepting a black-box rewrite and losing understanding of the system.

`contuts` should provide a third option:

```text
AI does substantial work.
Human controls the direction and consequences.
The system preserves the decisions.
```

## Core Workflow

```text
Human selects a meaningful context
        |
        v
AI proposes multiple possible changes
        |
        v
Human inspects intent, structure, and impact
        |
        v
Human keeps, removes, or combines edits
        |
        v
The system records the resulting working state
        |
        v
AI continues from that state
```

The human should be able to direct the process with statements such as:

```text
Continue from this working state.
Keep the API change but not the implementation detail.
Show me three architectural alternatives.
Explain the intention of this group of edits.
Move this responsibility into another package.
```

## Current Product Position

The project has moved from pure engine exploration into UI workflow exploration.

The engine work remains foundational:

- Field-aware AST representation.
- Structural edit provenance.
- Intermediate incomplete states.
- Reversible application and removal.
- Best-effort rendering.

But the engine is not the final user experience. A list of low-level AST edits is not, by itself, a meaningful way to direct an AI. The UI must progressively expose higher-value choices while preserving the underlying detail for inspection.

## Progressive Lifting

The API should remain low-level and complete. The UI should add views above it.

A lifted view is a projection of the same edit state. It is not a new edit system and must not silently merge or destroy edits.

The progression should be cautious:

1. Hide mechanical wrappers while preserving their children.
2. Group edits around meaningful constructs such as functions, conditions, calls, and declarations.
3. Expose intent and consequences for those groups.
4. Let the human move between lifted and low-level detail at any time.
5. Add folder and package operations above file and AST operations.

Examples of possible presentation-only shells include:

- Required function or control-flow body blocks.
- Expression-statement wrappers.
- Declaration-statement wrappers.
- Import-spec wrappers.
- Field-list wrappers.

The UI must remain context-sensitive. An explicit nested block that creates a scope is meaningful and should not be hidden merely because it is a `BlockStmt`.

## Meaningful Structure

Not every AST node deserves equal attention in the primary workflow.

The system should distinguish:

- Mechanical shells needed to assemble valid syntax.
- Meaningful constructs that express behavior or architecture.
- Values and identifiers that carry concrete content.
- Structural errors that prevent a legal representation.
- Differences from an AI proposal that are not errors.

An edit being different from what the AI proposed is not a structural error. A valid `if` placed in a different valid block is still structurally valid.

## Comments And Intent

AI-generated rationale should be independently toggleable.

Intent comments should:

- Explain why a group of changes was proposed.
- Be attached to edits, groups, or architectural operations.
- Remain distinct from source comments.
- Be hidden when the user wants a clean code view.
- Be visible when the user is evaluating alternatives.

The system should never force speculative AI explanation into the code itself.

## Folder-First View

The human should be able to begin at the level where architecture is usually understood:

```text
folders -> packages -> files -> declarations -> syntax
```

The folder-first view should support human-directed operations such as:

- Moving a function or declaration between files.
- Splitting a file by responsibility.
- Combining files when cohesion improves.
- Reorganizing packages and directories.
- Reviewing import, reference, test, and dependency consequences.
- Comparing alternative layouts proposed by AI.

The folder view must be another projection of the same working state. It must not create a second untracked source of truth.

## Human Mental Models And Graphs

Graphs, dependency maps, call graphs, and architecture diagrams are useful instruments. They are not the architecture itself.

The architecture a person builds in their head is shaped by experience, goals, constraints, and evolving understanding. It is not fully captured by automatically generated nodes and edges. The product should help the human develop and test that mental model rather than pretending that a graph can replace it.

The system can show facts:

```text
AST diff        -> syntax changed
Type analysis   -> types changed or stayed the same
References      -> definitions and uses
Call analysis   -> callers and callees
Impact analysis -> code that may observe a change
```

The human still decides what those facts mean for the architecture.

## Multiple Versions

AI should be able to propose multiple versions of a change.

Each proposal should retain its own:

- Working-state basis.
- Structural edits.
- Intent explanation.
- Affected context.
- Provenance.

The human should be able to:

- Compare proposals.
- Apply one proposal completely.
- Apply selected edits from several proposals.
- Reject edits without losing the alternatives.
- Ask for another version based on the chosen combination.

This is more useful than asking the human to approve one opaque patch.

## OpenCode Integration

OpenCode should eventually live inside the workflow rather than beside it as an unrelated chat panel.

The AI context should include:

- The selected folder, package, file, or AST context.
- Current source and working state.
- Applied edits.
- Removed edits.
- Unapplied alternatives.
- User-selected proposal versions.
- Intent comments.
- Relevant provenance and affected-code information.

OpenCode should be able to explain, propose, revise, and continue. It should not reset the user's decisions or silently replace their working state.

## Principles

- Human direction comes first.
- AI provides leverage, not authority.
- The human owns the current working state.
- A proposal is not a command.
- A target difference is not automatically an error.
- Structural errors must be distinguished from preference differences.
- Every meaningful transformation should be inspectable.
- Every applied transformation should be reversible.
- Low-level provenance must remain available.
- Lifted views simplify presentation, not semantics.
- Comments about intent are optional and separate from source.
- Architecture is shaped by human understanding, not only by generated graphs.
- Facts, interpretations, and suggestions must be distinguishable.
- The project must document its own uncertainty honestly.

## Roadmap

### 1. Make The Current UI Coherent

- Make low-level AST edits understandable.
- Make lifted views preserve all meaningful children.
- Make apply, remove, sibling, and subtree operations predictable.
- Make structural errors visible in source without replacing source content.
- Show exactly why an edit is structurally invalid.

### 2. Continue Lifting Carefully

- Lift additional mechanical wrappers.
- Add construct-level groups without hiding low-level edits.
- Make every lifted feature independently toggleable.
- Preserve a clear path from a high-level presentation to raw AST edits.

### 3. Make Working State First-Class

- Add named checkpoints.
- Preserve edit history and provenance.
- Compare working states.
- Allow returning to previous decisions.
- Make combinations of AI proposals explicit.

### 4. Add Intent Comments

- Attach rationale to edits and groups.
- Toggle intent comments independently from source.
- Distinguish AI explanation from verified facts.

### 5. Build The Folder-First View

- Support multi-file packages.
- Show folder, package, file, and declaration structure.
- Represent moves and reorganizations as reversible operations.
- Show architectural consequences across imports, references, and tests.

### 6. Support Multiple AI Proposals

- Request several versions.
- Compare versions structurally and architecturally.
- Combine selected edits.
- Preserve rejected alternatives.

### 7. Integrate OpenCode

- Send the real working state to OpenCode.
- Continue from human decisions.
- Ask for explanations and alternatives in context.
- Keep the user in control of which proposal becomes active.

### 8. Add Semantic And Impact Analysis

- Resolve definitions and uses.
- Show callers and callees.
- Show type information when available.
- Show affected tests and packages.
- Keep analysis claims separate from human judgment.

## The Product Test

The product is moving in the right direction when this interaction feels natural:

```text
The AI offers possibilities.
I understand what they mean.
I choose the architectural direction.
I keep and remove specific changes.
I see the working state I created.
I ask the AI to continue from there.
```

The goal is not human approval of every token. The goal is meaningful human ownership of structure, intent, and direction while AI performs substantial work.
