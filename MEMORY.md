# Project Memory

## Current Direction

`contuts` is a structural code-understanding and transformation tool. The
engine owns normalized trees, identity-aware edits, lifting, reconciliation,
working-state transitions, and reversible application. Language adapters own
parsing and source representation.

## Supported Adapters

- Go uses the existing `go/ast` adapter and Go-specific renderer/validation.
- TypeScript uses the Tree-sitter TypeScript grammar for syntax-focused
  parsing, structural diffing, applying, removing, and best-effort rendering.
- HTML uses the Tree-sitter HTML grammar through the same structural engine.

The public entry points are `ParseLanguage`, `DiffLanguage`, and
`NewWorkingStateFromLanguage`. The desktop comparison path selects Go,
TypeScript, or HTML from the file extension for `.go`, `.ts`, `.html`, and
`.htm` files.

## Important Boundaries

- TypeScript support is syntax-focused. It does not yet resolve types,
  references, `tsconfig` paths, project references, or cross-file symbols.
- TSX/JSX is not enabled yet.
- Workspace discovery and declaration exploration remain Go-specific, so the
  desktop workspace does not automatically list TypeScript or HTML files yet.
- Do not replace the generic structural engine with language-specific edit
  logic. Add language behavior at the adapter/parser/render/validation seam.

## Verification

Run the root tests with `go test ./...` and the desktop tests with
`cd desktop && go test ./...`. The TypeScript test suite covers common syntax,
language dispatch, applying/removing edits, inserted declarations, validation,
and invalid source handling. The HTML suite covers parsing through the shared
language dispatch and applying a text edit.
