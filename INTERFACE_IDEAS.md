# Interface Ideas

The current interface is Neovim plus nvim-dap. The interface should keep the
debugger familiar and make Contuts context visible around it.

## Main Surfaces

```text
:Contuts
    choose base commit
    show changed files and AST pointers

:ContutsDebug
    choose base commit
    choose configured targets
    generate breakpoints and experiment context

:ContutsDebugStart
    launch the selected targets

:ContutsCoverage
    show changed locations reached by DAP

:ContutsDebugRerun
    choose a saved experiment and restore it
```

## Useful Views

- Source view with changed-line highlighting.
- Breakpoint list from nvim-dap-ui.
- Normal DAP stack, scopes, console, and source frame.
- Changed-location coverage report.
- Experiment directory and artifact links.
- Rerun history under the original experiment.

The UI should always show the comparison context:

```text
base: 573727b
current: working-tree
branch: main
targets: api, web
experiment: 20261010-...
```

## Do Not Hide The Boundaries

The interface should distinguish:

```text
source changed
breakpoint generated
breakpoint reached
network captured
database restored
experiment replayed
```

It should not imply that an AST pointer is a test result or that a saved trace
proves every possible behavior.
