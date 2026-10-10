# Contuts Direction

Contuts is a context saver and build surface for AI-assisted software work.
It helps a person stay mentally involved by connecting four things:

```text
Git change
    -> AST change pointers
        -> configurable runtime adapters
            -> saved experiment context
```

Contuts does not replace the editor, the debugger, the test runner, or the
application. It assembles the context needed to use those tools deliberately.

## The Landed Workflow

```text
choose a base commit
    -> compare it with the current revision or working tree
    -> generate pointers to changed executable locations
    -> choose configured runtime targets
    -> start a normal debug session
    -> run a real scenario
    -> save the experiment context
    -> rerun the experiment later
```

The human remains in the debugger. Contuts does not add a second stepping model.

## Context Build

An experiment can preserve:

- Base commit, current revision, branch, and target configuration.
- Generated and manually retained breakpoints.
- Changed-code hit counts and DAP stop events.
- Database snapshots and optional restore commands.
- Configured environment-file references, with copying opt-in.
- Browser actions, network captures, traces, console output, and screenshots.
- Backend/API artifacts supplied by their adapters.

An experiment is a saved context build, not a claim that the whole program has
been proven correct.

## AST Change Pointers

The AST engine compares two source states and identifies meaningful changed
locations. For debugging, executable locations become DAP breakpoints. The
coverage view reports which generated locations were reached and how often.

This is intentionally narrower than full branch or path coverage:

```text
changed location -> breakpoint -> runtime stop -> hit count
```

The AST points the human back to the code that changed. The debugger shows what
that code did when it ran.

## Runtime Adapters

Targets and capture mechanisms are configured in the project’s `.contuts.json`.
Contuts should not assume that every project has a frontend and backend.

Useful adapters include:

- Node or another DAP target for runtime stops and stack state.
- HTTP/API drivers for backend-only scenarios.
- PostgreSQL or another database adapter for snapshots and query evidence.
- Browser/Playwright adapters for frontend actions and browser network traffic.
- Process adapters for services, workers, and custom scenario commands.

Each adapter contributes artifacts to the same experiment directory. The common
experiment ID is the connection between a browser action, an API request, a
database snapshot, and a debugger stop.

## Reruns

`:ContutsDebugRerun` selects a saved experiment and creates a child run. It can
restore the saved database, load the saved breakpoints, reuse the selected
targets, and preserve the Git comparison. It does not switch the current branch
automatically.

Replaying an experiment is strongest when the starting database, environment,
inputs, dependencies, and external responses are controlled. The system should
state what was restored and what remained live rather than pretending that a
single replay is universal proof.

## Boundaries

Contuts should not:

- Replace nvim-dap’s normal debugger controls.
- Pretend that a breakpoint hit proves behavior is correct.
- Capture secrets by default.
- Treat a browser as the only way to exercise a backend.
- Generate tests from every observed value without human context.
- Switch branches or overwrite a working tree during a rerun.

The practical question is:

> After an AI-assisted change, can the human quickly see what changed, run it,
> understand the important behavior, and return to the same context later?
