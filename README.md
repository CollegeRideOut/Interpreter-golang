# Contuts

Contuts is a context saver for debugging AI-assisted code changes. It connects
source changes to real runtime behavior without replacing the editor, debugger,
or test runner.

```text
choose a Git base
    -> identify meaningful AST changes
    -> create changed-code DAP breakpoints
    -> run configured targets and scenarios
    -> save runtime context and artifacts
    -> inspect, compare, and rerun
```

Code review remains code review. Production monitoring remains production
monitoring. Contuts owns the local build-and-understand loop between them.

## What It Captures

An experiment can preserve:

- Git base, current revision, branch, and target configuration.
- AST-derived changed locations and generated breakpoints.
- Normal DAP stops, stack events, and changed-location hit counts.
- Browser actions, network requests, HAR, traces, console output, and screenshots.
- API or process output supplied by project adapters.
- Database snapshots and restore commands.
- Rerun metadata and saved breakpoint context.

An experiment is evidence and context, not a correctness proof.

## Neovim Workflow

Build the command-line tool:

```sh
go build -o "$HOME/bin/contuts" .
```

The Neovim plugin reads `.contuts.json` from the project directory. A project
can define any number of targets and optional experiment adapters:

```json
{
  "comparison": {
    "base": "main",
    "current": "working-tree"
  },
  "targets": [
    {
      "name": "api",
      "kind": "node",
      "entry": "packages/functions/src/server.ts"
    },
    {
      "name": "web",
      "kind": "browser",
      "command": "pnpm dev:web",
      "url": "http://localhost:5173",
      "startDelayMs": 1500
    }
  ],
  "experiment": {
    "database": {
      "enabled": true,
      "snapshotCommand": "pg_dump --format=custom \"$DATABASE_URL\" --file {experiment}/database.dump",
      "restoreCommand": "pg_restore --clean --if-exists --no-owner --dbname \"$DATABASE_URL\" {snapshot}"
    },
    "envFiles": [".env"],
    "saveEnvFiles": false,
    "browser": {
      "command": "pnpm run contuts:browser -- --url http://localhost:5173 --output {experiment}/browser"
    }
  }
}
```

Use these commands from the project directory:

```vim
:Contuts
:ContutsDebug
:ContutsDebugStart
:ContutsCoverage
:ContutsDebugRerun
```

`:Contuts` opens the source comparison. `:ContutsDebug` selects a base commit
and target set, generates breakpoints for meaningful changed executable
locations, and creates an experiment. Review or edit the breakpoints, then use
`:ContutsDebugStart` to launch the targets.

The sessions remain normal nvim-dap sessions:

- A breakpoint pauses execution.
- DAP UI shows the source, stack, scopes, and locals.
- `<leader>dc` continues.
- `<leader>dn` steps over.
- `<leader>di` steps into.
- `<leader>do` steps out.
- `<leader>db` toggles a manual breakpoint.

Contuts observes the session and records context. It does not introduce a second
debugging language.

## Comparison And Coverage

`base` and `current` accept Git revisions understood by `git diff`, including
`working-tree`. When no base is supplied interactively, Contuts lists commits
reachable from the current branch.

The command-line comparison is also available directly:

```sh
contuts --nvim-json /path/to/project HEAD~1 working-tree
```

The output contains changed files and structural edits. The Neovim integration
filters syntax-only edits so breakpoints focus on executable locations. This is
change context, not a second checkout and not full branch or path coverage.

`:ContutsCoverage` reports which generated changed locations were reached and
how often. A reached location means execution got there; it does not mean the
behavior was correct.

## Experiments And Reruns

Experiments are stored under:

```text
.contuts/experiments/<id>/
```

`:ContutsDebugRerun` loads a saved experiment, restores configured environment
and database state when enabled, reloads saved breakpoints, and starts a child
run under `reruns/`. It does not switch the current Git branch automatically.

Browser recording uses a dedicated Playwright browser. The recorder writes
actions, a generated Playwright spec, network JSON, HAR, trace, console output,
and a completion marker. Its shutdown protocol is designed to flush artifacts
when a debug session or Neovim exits.

Treat experiment artifacts as potentially sensitive. Do not commit database
dumps, HAR files, cookies, tokens, or environment contents.

## Target Model

Targets are intentionally configurable:

- `node` targets launch through the Node DAP adapter.
- `browser` targets can start a web process and use a browser URL.
- Browser recording is an adapter and does not replace backend DAP.
- API, database, worker, and custom process adapters can be added through
  project commands without changing the core plugin.

The same experiment ID connects a scenario, runtime stop, database state, and
adapter artifacts.

## Development

Run the Go tests:

```sh
go test ./...
go test -race ./...
```

Check the Neovim plugin and Narrativo browser recorder:

```sh
luac -p nvim/lua/contuts.lua
node --check /path/to/narrativo/scripts/contuts-browser-experiment.mjs
```

The repository also contains AST, terminal, desktop, watcher, and debugger
prototypes. The practical product boundary is the experiment workflow above:
point to the changed code, run it with normal tools, save what mattered, and
return to the same context later.
