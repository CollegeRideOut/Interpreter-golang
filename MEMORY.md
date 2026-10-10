# Project Memory

## Current Mental Model

Contuts is a context saver/build for a human working with AI-generated code.
The user wants to remain involved in the system by running it, seeing changed
locations, debugging real behavior, and returning to the same setup later.

```text
AI changes code
    -> Contuts points to the changed code
        -> the human runs and debugs it
            -> Contuts saves the context
                -> the human can rerun it
```

## Current Layers

### Change Layer

The Go engine compares revisions and emits structural change data. The Neovim
integration turns meaningful executable changes into breakpoint locations.

### Debug Layer

nvim-dap remains the debugger. Contuts observes DAP stop events, maps stops to
changed locations, and records hit counts and experiment events.

### Adapter Layer

Projects define targets and capture commands in `.contuts.json`. Adapters may
provide DAP, HTTP, browser, database, process, or external-service evidence.

### Experiment Layer

An experiment stores the Git context, targets, breakpoints, database snapshot,
environment choices, browser artifacts, and runtime events needed for a later
rerun.

## Security Rules

- Do not commit `.env` files, database dumps, HAR files, cookies, or tokens.
- Environment-file capture must be explicitly enabled.
- Database snapshots should target disposable local databases.
- Network and runtime artifacts may contain personal or secret data.
- A rerun must not silently switch branches or overwrite the working tree.

## Language Boundaries

TypeScript support is currently syntax-focused. It does not claim complete type,
symbol, call, or cross-project analysis. AST pointers are useful context, not a
semantic proof of impact.

The generic structural engine remains separate from language adapters. Add
language-specific behavior at parsing, source, rendering, validation, or runtime
adapter boundaries.

## Useful Commands

```sh
go test ./...
go build -o "$HOME/bin/contuts" .
```

In a configured project:

```vim
:Contuts
:ContutsDebug
:ContutsDebugStart
:ContutsCoverage
:ContutsDebugRerun
```
