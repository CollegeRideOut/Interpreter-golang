# Contuts

Contuts is an experiment in keeping a human mentally involved while AI changes
the codebase.

AI can make a large implementation quickly. The risk is not only that the code
is wrong. The risk is that the code is correct and the human no longer knows
where behavior lives, how the system evolved, or what a change actually did.

Contuts explores a simple loop:

```text
inspect a change
    -> apply a meaningful structural group
    -> run a known scenario
    -> observe what happened
    -> inspect or investigate
    -> continue from the new state
```

The goal is not to make humans approve every line. The goal is to let AI do the
typing while giving the human a way to stay oriented.

## Two Tools

Contuts has two related responsibilities.

### AST Evolution

The engine compares two repository states and produces structural AST edits.
The AST is used to make controlled, valid changes and to show what changed
before and after an edit.

The AST is not intended to be the primary human abstraction. Syntax shells such
as punctuation, delimiters, import containers, and block wrappers are applied
automatically. Human-facing review should focus on top-level semantic edit
groups.

For example, a human should normally see:

```text
Add books persistence model
Register the migration
Load books from the repository
Expose books from the API
```

not a list of every comma, identifier, and syntax node required to render those
changes.

Each group can be expanded when structural detail is useful. Applying a group
applies the complete AST subtree, including the syntax edits that support it.

### Execution History

The debugger side records what happened when a source state ran.

An execution record can contain:

- The source checkpoint.
- The command, input, environment, and fixtures.
- Breakpoints and debugger locations.
- Stack frames and observed values.
- Output, errors, and the final result.
- Paths reached during the run.
- The first observable difference from another run.

Normal debuggers remain responsible for stepping through a live process. Contuts
organizes repeatable scenarios and keeps the history connected to source
checkpoints. Going back initially means restoring a source checkpoint and
replaying the scenario. For the interpreter, complete runtime snapshots may
eventually allow direct state restoration.

## Neovim First

Neovim is the primary editing and debugging surface. Contuts does not try to
replace it with another code editor or debugger.

The first Neovim integration displays the files changed between two revisions
and their structural edits. It is intentionally read-only for now; application
and checkpoint commands will use the same engine next.

Build the command-line tool:

```sh
go build -o "$HOME/bin/contuts" .
```

Add the plugin to your Neovim configuration:

```lua
local contuts = require("contuts")

contuts.setup({
  command = vim.fn.expand("~/bin/contuts"),
  directory = vim.fn.getcwd(),
  base = "HEAD~1",
  compare = "working-tree",
})
```

Open the edit overview:

```vim
:Contuts
```

The overview lists changed files and their AST edits. Press `<CR>` on an edit
to open its file and source location. Press `r` to refresh and `q` to close.

The command-line JSON interface can also be used directly:

```sh
contuts --nvim-json /path/to/repository HEAD~1 working-tree
```

The JSON response is deliberately small and stable enough for editor clients:

```json
{
  "schema": "contuts.nvim.v1",
  "directory": "/path/to/repository",
  "base": "HEAD~1",
  "compare": "working-tree",
  "files": [
    {
      "path": "server/src/db.ts",
      "edits": []
    }
  ]
}
```

## Human Build And Git

Proposal targets remain immutable. The Human build is the mutable state where
selected structural groups from one or more proposals are combined.

The workflow is:

```text
choose a current baseline
    -> choose one or more proposal sources
    -> inspect semantic edit groups
    -> apply selected groups to Human build
    -> run or debug the Human build
    -> commit Human build on the current Git branch
```

The tool does not move `main`. If the repository is on `feature/login`, the
Human build commit is made on `feature/login`. The interface should always make
these contexts explicit:

```text
branch: feature/login
base: <baseline revision>
compare to: <selected branch or commit>
proposal: <proposal source>
```

## Architecture View

The most useful overview is not a giant edit tree. It is a map of the system:

```text
project
  -> package or folder
      -> file
          -> declaration
              -> imports, callers, callees, references
```

The architecture view should answer:

- Where does this behavior live?
- What files and packages are involved?
- What depends on this declaration?
- Which files changed together?
- Which execution entry point can exercise the change?

This is the part that is difficult to get from a normal Neovim session. The
Neovim plugin can remain focused on editing and debugging while Contuts provides
the repository-level map.

## Debugger Integration

Contuts should use existing debugger technology rather than become a universal
debugger.

- Go: Delve through DAP.
- JavaScript and TypeScript: Node inspector or `vscode-js-debug`.
- Other languages: their existing DAP adapter where practical.
- Interpreter execution: Contuts instrumentation, because the runtime is under
  our control.

Neovim and `nvim-dap` are well suited for breakpoints, stepping, stack frames,
locals, watches, and test debugging. Contuts adds the missing history around
those sessions: which source checkpoint ran, with which input, and how its
observed execution compared with another checkpoint.

## Interpreter Experiment

The interpreter is the first execution target because its complete path is
observable:

```text
source
  -> tokens
  -> AST
  -> environment
  -> evaluation
  -> function calls
  -> values
  -> result
```

A useful first experiment is:

```text
run historical input against state A
    -> save trace A
apply one semantic edit group
    -> run the same input against state B
    -> save trace B
compare the traces and locate the first divergence
```

If a path is not reached, the tool should say so. It should distinguish:

- Not reached: the scenario did not execute the path.
- Blocked: the path needs different input, fixtures, or environment setup.
- Observed: the path executed and produced evidence.

Creating the setup required to reach a path should remain separate from the code
change being investigated.

## Current Direction

The immediate direction is deliberately narrow:

1. Keep the AST engine as the trusted structural application layer.
2. Show top-level semantic edit groups by default.
3. Use Neovim for source editing and debugger interaction.
4. Use Contuts for architecture, source checkpoints, scenarios, and history.
5. Build one replayable before/after execution experiment for the interpreter.

The project is an experiment, not a claim that every engineer needs this
workflow. The result should be judged by one question:

> After an AI-driven change, do I understand the codebase and its behavior
> better because I could observe the transition?

## Development

Run the root tests:

```sh
go test ./...
go test -race ./...
```

Run the desktop tests:

```sh
cd desktop
go test .
go test -race .
```

Run the React frontend tests and build:

```sh
cd desktop/frontend-react
npm test
npm run build
```

The repository contains experimental desktop, terminal, AST, and debugger
prototypes. They are implementation material for testing the mental-model
hypothesis, not separate product commitments.
