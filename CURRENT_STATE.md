# Current State

Last updated: 2026-10-10

## Product

Contuts is currently a context saver/build for AI-assisted development. Its
working path is Neovim plus nvim-dap:

```text
base commit + current code
    -> AST change pointers
    -> configurable targets
    -> normal DAP session
    -> saved experiment
    -> rerun
```

The human uses the debugger to understand the code. Contuts prepares and saves
the context; it does not replace debugging.

## Implemented

- Go AST and language-aware structural comparison.
- TypeScript, TSX, HTML, and Go change JSON for editor integrations where
  supported by the current engine.
- Configurable base commit and current comparison.
- Current-branch commit picker for `:Contuts` and `:ContutsDebug`.
- Composable `.contuts.json` target definitions.
- Node and browser target startup through normal DAP configuration.
- Automatic breakpoints for meaningful changed executable locations.
- DAP stop notifications and changed-location hit counts.
- `:ContutsCoverage` for reached changed locations.
- Experiment directories created by `:ContutsDebug`.
- Git, target, breakpoint, and DAP event metadata.
- Configurable database snapshot and restore commands.
- Optional environment-file capture.
- Playwright browser action, network, trace, and console capture for projects
  that configure a recorder command.
- `:ContutsDebugRerun` with saved breakpoint and experiment context.
- A watcher that records source checkpoints and process output.

## Narrativo Example

The Narrativo project is configured with:

- An API Node target.
- A Vite/browser target.
- A local PostgreSQL snapshot using `pg_dump`.
- A PostgreSQL restore command using `pg_restore`.
- A Playwright browser recorder.

The current working example compares an explicitly selected earlier commit with
the working tree. Choosing `main` while already on `main` correctly produces no
diff for a clean committed tree; choose an earlier commit to inspect its later
changes.

## Known Limits

- Changed-location coverage is not branch or full path coverage.
- DAP values are captured only where adapters or selected watches provide them.
- Browser recording uses a dedicated Playwright browser rather than a personal
  Chrome profile.
- Backend network and database query adapters are configuration points; the
  browser cannot see internal backend traffic by itself.
- Reruns do not automatically checkout a branch.
- Environment files are not copied unless explicitly enabled.
- Database restore commands can overwrite data and should target disposable
  databases.

## Verification

```sh
go test ./...
cd /home/tuts/Work/personal/narrativo && pnpm build
```

Lua syntax is checked with `luac -p`, and the Narrativo debug flow is exercised
through headless Neovim/DAP runs when changing the integration.
