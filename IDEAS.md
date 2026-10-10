# Product Ideas

Ideas here extend the landed model. They are not promises.

## Core Idea

The useful unit is a saved experiment:

```text
Git context
  + AST change pointers
  + runtime targets
  + adapter artifacts
  + debugger events
  = replayable context
```

The person does not need to read every line an AI generated before running the
program. They need a way to ask focused questions, exercise meaningful behavior,
and return to the exact context that produced an observation.

## Adapter Ideas

- HTTP request/response drivers for backend-only experiments.
- PostgreSQL query and transaction capture around a database snapshot.
- Outbound HTTP capture for API dependencies.
- Queue and worker adapters.
- Playwright browser action and network capture.
- Test-runner adapters that attach an existing Vitest, Go, or Playwright run to
  an experiment.
- OpenTelemetry-compatible event IDs for joining browser, API, database, and
  external-service events.

## Evidence Ideas

An artifact should distinguish:

```text
changed: AST says this source changed
reached: runtime stopped at this location
observed: an adapter captured this input/output/state
replayed: the saved experiment ran again
expected: the human chose this result as desired
```

Do not call one breakpoint hit “verified behavior.” The report should say exactly
what was observed and what was restored.

## Test Generation

Test generation may come later. A useful generated test would be based on a
named, replayed, human-understood scenario rather than every local variable:

```text
saved experiment
    -> selected inputs and outputs
        -> generated API or browser test
```

The generated test should use existing runners rather than becoming a new test
framework inside Contuts.

## Human Context

The project should optimize for active understanding:

```text
ask a question
  -> run an experiment
    -> inspect the changed path
      -> save what mattered
        -> return later
```

That is more useful than forcing a person to approve every AST node or consume a
large AI explanation after the fact.
