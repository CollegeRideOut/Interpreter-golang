# AI And Human Context

This is the project thesis, not a claim that every idea is implemented.

## The Problem

AI can change a codebase faster than a person can remain mentally immersed in
the implementation. A common loop becomes:

```text
prompt -> wait -> generated code -> summary -> reconstruct context -> prompt again
```

The problem is not that the human must type every line. The problem is losing the
continuity of building, running, questioning, and improving the system.

## The Contuts Response

Contuts keeps the human in the working system without pretending to be the author
of every edit:

```text
AI proposes or writes
    -> Git identifies the source context
        -> AST points to changed code
            -> adapters run a real experiment
                -> DAP and artifacts show what happened
                    -> the human decides what matters
```

The human owns the scenario, the questions, and the decision to accept an
observation. AI can help implement, summarize, or propose a replayable test, but
an AI explanation is not evidence by itself.

## Evidence Categories

- **AST fact**: these source nodes changed.
- **Runtime fact**: execution reached this file and line.
- **Adapter fact**: a request, response, query, or browser action was captured.
- **Restoration fact**: a database, environment, or breakpoint set was restored.
- **Human expectation**: the person says this result is desired.
- **Hypothesis**: a possible explanation that still needs investigation.

Contuts should not collapse these categories into “the feature is correct.”

## Practical Standard

The project succeeds when a person can answer:

```text
What changed?
Which configured target exercised it?
What did the runtime receive and return?
What database or environment was used?
Where did execution stop?
Can I run the same experiment again?
```

That is enough. It does not require a universal AI reviewer or a replacement IDE.
