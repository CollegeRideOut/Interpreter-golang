# Current State

Last updated: 2026-09-14

## Product Direction

contuts is exploring a headless, evidence-preserving code inquiry and
transformation engine. The intended UI is an unlimited left-to-right path of
reusable explorer columns. The user starts at the left and asks questions into
the right: imports, same-package references, types, callers, callees,
dependents, and edit consequences.

The important product idea is a causal line of sight through code evolution:

```text
AI proposal -> parent edit -> child edit -> affected code -> working revision
```

The user can accept every edit and still gain understanding by walking through
the transitions. The user can also reject or alter an edit and continue from a
new branch of the working state.

## What Exists

- Go AST export and field-aware structural diffs.
- Mutable working AST and reversible edit operations.
- Reconciliation and lifted edit views.
- Wails desktop application.
- Package, file, declaration, import, and source exploration.
- Parameter, result, variable, and named-type display metadata.
- Direct same-package reference discovery across sibling files.
- Same-package reference and type links that open related files.
- Calorie-counter multi-package fixture in `TestProgramCalorieApp/`.
- Structural contract and agent-loop direction documented, but not implemented.

## Current UI Reality

The Angular UI is still a hard-coded primary/secondary explorer prototype. It
does not yet render an arbitrary array of identical columns. Imported files and
same-package references currently open in the secondary context rather than a
true unbounded path.

The next UI architecture should extract the explorer into a reusable
`ExplorerColumnComponent` and store independent column state in an array.

## Current Engine Reality

The engine is useful but only partially understood. AST parsing and structural
editing are the current foundation. Name-based reference discovery is only a
prototype and is not equivalent to full semantic resolution.

Not yet implemented reliably:

- Full symbol identity through `go/types` or `gopls`.
- Imported type links from signatures.
- Complete same-package, reference, call, caller, and callee analysis.
- Dependents and impact analysis.
- Structural contract verification API.
- Agent/ACP edit loops.
- Durable inquiry paths and cycle handling.
- Edit-evolution timeline connected to affected-code relationships.

## Recommended Next Session

Start in UI land rather than redesigning the engine:

1. Extract the current explorer markup into one reusable column component.
2. Replace primary/secondary state with an array of column targets.
3. Append a new identical column when a relationship is selected.
4. Preserve the originating selection and label the relationship.
5. Add close/back behavior for the rightmost path.
6. Add previously-opened detection for cycles.
7. Keep unrelated questions in separate rows.
8. Render a first edit-evolution strip below the inquiry path.

Only after that should the analysis boundary be upgraded with `go/types`,
`go/packages`, or LSP support.

## Verification Last Run

These checks passed during the last implementation session:

```text
go test ./...
```

The frontend test runner reports 2 tests passing. Generated frontend artifacts
may appear dirty after frontend checks.

## Worktree Notes

The repository contains uncommitted implementation and generated changes from
the exploratory UI and engine work. Do not reset or discard them without
reviewing them first. Documentation commits have been kept separate.

Historical ideas remain in:

- `oldREADME.md`
- `oldDIRECTION.md`
- `INTERFACE_IDEAS.md`
