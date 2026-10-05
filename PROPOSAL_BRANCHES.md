# Proposal Branches

This document describes the next product phases for exploring multiple AI
proposals without giving up human control of the resulting program.

## Base Model

The visible program is the `Compare to` revision. It is the base program that
the person explores.

The `Current revision` is only the comparison baseline. Changing it changes the
diff source; it must not replace the visible program explorer.

```text
visible workspace = Compare to revision
comparison source = Current revision
proposal branches = future changes forked from the visible workspace
```

The existing AST edit machinery remains important. It supplies declaration
ownership, parent/child relationships, reversible operations, and provenance.
Proposal branches should be built around those operations rather than replacing
them with opaque text patches.

## Phases

### Phase 1: Establish The Base

Make the revision roles explicit and stable.

- Keep the normal explorer on `Compare to`.
- Treat `Current revision` only as the diff baseline.
- Show structural edits below the same Program inquiry.
- Keep source exploration available while edits are visible.
- Do not create proposal branches yet.

This is the phase to try manually. The question is whether exploring the target
program with its structural edits attached already feels useful.

### Phase 2: Create Proposal Branches

Add first-class working branches forked from the base program.

```text
base
├── proposal-a
├── proposal-b
└── proposal-c
```

Each branch records its parent branch, base revision, structural operations,
diagnostics, tests, and proposal provenance.

### Phase 3: Use A Proposal Contract

Let an AI return separate proposals instead of silently mutating one workspace.
Each proposal should describe its intended behavior as an interpretation, not
as proven truth, plus its affected declarations, operations, risks, and tests.

For consequential work, a useful request is:

```text
Generate three independent proposals from this base:
1. Conservative.
2. Balanced.
3. Broad or alternative.

Do not modify the shared workspace. Keep each proposal separate.
```

Three proposals are not necessary for trivial changes.

### Phase 4: Explore Proposals

Show normal source first, then progressively reveal the structural explanation.
Each proposal should support package, file, declaration, source, reference, and
edit exploration.

### Phase 5: Compose Proposals

Allow a person to copy a declaration group or structural operation from one
proposal into another branch.

Automatically combine only clearly independent operations, such as changes to
different declarations or disjoint children of a common parent.

### Phase 6: Surface Conflicts

When proposals modify the same declaration, field, list position, or parent,
show an explicit conflict instead of guessing.

```text
Both proposals modify function App.

Keep proposal A
Keep proposal B
Attempt composition
Inspect manually
```

### Phase 7: Add Relationship Evidence

Callers, references, and affected code are evidence attached to a branch, not
automatic merge decisions.

Keep these categories separate:

- AST fact.
- Reference fact.
- Call-analysis fact.
- Impact hypothesis.
- Human interpretation.

### Phase 8: Validate And Export

For the final human-selected branch:

- Render the resulting source.
- Run AST, type, and project validation.
- Run tests and runtime checks where available.
- Preserve provenance and decisions.
- Export a patch, branch, or commit.

## First Vertical Slice

Do not begin with arbitrary multi-language merging. First prove the model with
one language, one base program, and two or three proposals:

1. Load a real AI-generated patch.
2. Fork two or three proposal branches from one base.
3. Group operations by declaration.
4. Apply one declaration group from proposal A.
5. Apply one compatible group from proposal B.
6. Detect a conflict when both modify the same declaration.
7. Show before/after source immediately.
8. Run tests and diagnostics.
9. Export the resulting patch.

The goal is to let a person play with code structure deliberately while keeping
the construction technically bounded and explainable.
