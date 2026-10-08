# Contuts

Contuts automates the repetitive parts of debugging code changes without
replacing the debugger.

Its current job is simple:

```text
compare the current code with a previous revision
    -> find meaningful changed locations
    -> place DAP breakpoints
    -> start the selected debug sessions
    -> let the human inspect, step, and continue
```

Neovim and `nvim-dap` remain responsible for the debugging experience: stopping
at breakpoints, showing source and stack frames, displaying locals, and stepping
through code.

## What Contuts Does

Contuts has two connected parts:

1. **Change context**: compare a source revision with the current working tree
   and show the affected semantic regions.
2. **Debug setup**: turn useful changed TypeScript locations into DAP
   breakpoints and launch the frontend, backend, or both.

It does not try to become a new editor or a new debugger. It does not replace
normal Neovim editing, DAP stepping, or the existing JavaScript debugger.

Historical execution records are the reason for connecting source comparison to
debugging. They are a later layer for answering, "What changed, and what did
that change do when it ran?" They are not required to use the current debugger
workflow.

## Current Neovim Workflow

Build the command-line tool:

```sh
go build -o "$HOME/bin/contuts" .
```

The Neovim plugin is configured with the path to that binary. The project must
provide a `.contuts.json` file describing its debug targets:

```json
{
  "frontend": {
    "command": "pnpm dev:web",
    "url": "http://localhost:5173"
  },
  "backend": {
    "entry": "packages/functions/src/server.ts"
  }
}
```

From the project directory:

```vim
:ContutsDebug
```

Contuts compares `HEAD~1` with the working tree, prepares breakpoints, and asks
which target to debug: `frontend`, `backend`, or `both`.

Review the generated breakpoints in `nvim-dap-ui`. Remove a breakpoint with `d`
when it is only startup or module-initialization noise. Then launch:

```vim
:ContutsDebugStart
```

The sessions behave like normal DAP sessions:

- A breakpoint hit pauses execution automatically.
- Neovim jumps to the stopped source line.
- DAP UI shows the stack, scopes, and locals.
- `<leader>dc` continues.
- `<leader>dn` steps over.
- `<leader>di` steps into.
- `<leader>do` steps out.

The Contuts-specific breakpoint commands are:

```vim
:ContutsBreakpoints
:ContutsDebugReload
:Contuts2Debug
```

`<leader>dp` opens the breakpoint list. `:ContutsDebugReload` refreshes the
breakpoint plan and restarts the active session after a source change.
`:Contuts2Debug` refreshes the `HEAD~1` comparison before preparing the plan.

## Frontend And Backend

When debugging `both`:

- The backend is launched through the Node DAP adapter.
- The frontend development command starts normally, such as Vite.
- A browser DAP session launches against the configured frontend URL.
- Vite's logs are process output; they are not the backend DAP session.

The backend DAP session is visible in the DAP stack with its Node process and
source frame. A request breakpoint is only hit when the browser or another
client actually calls that backend route.

Automatic breakpoints come from changed files. To add a breakpoint that is not
part of the current diff, open the source, place the cursor on the line, and
press:

```text
<leader>db
```

Add manual breakpoints after `:ContutsDebug` prepares the plan and before
`:ContutsDebugStart` launches it.

## Source Comparison

The command-line comparison is also available directly:

```sh
contuts --nvim-json /path/to/project HEAD~1 working-tree
```

The JSON output gives editor integrations a stable list of changed files and
structural edits. Contuts filters syntax-only nodes so the default breakpoint
plan focuses on executable locations rather than every punctuation or delimiter
in the AST.

The comparison is context, not a second checkout. Debugging runs the current
working tree. `HEAD~1` is shown to explain what changed.

## Execution History

The longer-term purpose is to connect a source change to observed behavior. A
future execution record can capture:

- The source revision and comparison.
- The scenario, input, environment, and fixtures.
- Breakpoint locations and stack frames.
- Selected locals or watched values.
- Output, errors, and the final result.
- The first observable difference between two runs.

The intended loop is:

```text
change code
    -> compare the change
    -> run the same scenario
    -> inspect the stopped state
    -> compare the observed result with the previous state
```

This is historical context around ordinary debugging. It should help restore a
mental model after an AI-assisted change, not force a new debugging language on
the user.

## Watch Sessions

The experimental watcher records source checkpoints and process output:

```sh
contuts watch \
  --directory /path/to/project \
  --command "pnpm dev:web"
```

Use `--once` to run a command once:

```sh
contuts watch \
  --directory /path/to/project \
  --command "pnpm build" \
  --once
```

Watch records are experimental. They are separate from the live DAP session.

## Development

Run the Go tests:

```sh
go test ./...
go test -race ./...
```

The repository contains AST, terminal, desktop, watcher, and debugger
prototypes. The practical product boundary today is the Neovim workflow above:
automate breakpoint placement and debug-session startup, then let the existing
debugger show what happened.
