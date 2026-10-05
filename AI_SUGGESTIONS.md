# AI Suggestions For Human Review

This document records a product thesis and a set of suggestions for humans to
review. It is intentionally not a specification or a claim that every idea is
already proven.

## The Concern

Modern AI systems are becoming powerful enough that people can trust them too
quickly. A person can ask an agent to change a large codebase, accept the
result, and never meaningfully inspect the code that changed.

That creates a gap between **software production speed** and **human ownership
of the software**.

The question is no longer only:

> Can AI produce working code faster?

It is also:

> Can a human still understand, question, and take responsibility for the
> structure of the code that AI produces?

## The Human Ownership Thesis

AI is powerful and valuable. Faster generation is useful, but infinite speed
is not automatically a better engineering process if human understanding is
removed from it.

Software engineering may already have received much of what raw generation
speed can provide, especially for ordinary syntax and boilerplate. The next
important problem may be preserving human structural ownership while using AI
to increase capability and reduce cost.

Structural ownership means that a person can see:

- Which declaration owns a change.
- Which parent and child structures were affected.
- Why an operation was proposed.
- What related code may be affected.
- What the program looks like before and after the operation.
- Which parts the person accepted, rejected, or still does not understand.

The purpose is not to make humans manually write every line. The purpose is to
keep humans involved in the important decisions about the shape and evolution
of the program.

## The Speed Question

It is reasonable to look at an AI agent rewriting a very large codebase in a
short time and think:

> That is extremely fast. Is becoming even faster still the main problem?

This project should treat that as a serious design question, not as opposition
to AI. Faster AI may still be valuable, but once generation is already faster
than a person can responsibly review, the limiting factor becomes:

- Understanding.
- Verification.
- Decision quality.
- Ability to recover from a wrong change.
- Confidence that the resulting structure is still owned by the team.

The goal is therefore not to slow AI down for its own sake. The goal is to add
guardrails that let humans use powerful AI without surrendering judgment.

## Guardrails With AI

AI should be able to propose large and fast changes. The human should receive
an inspectable transformation rather than an opaque final answer.

Useful guardrails include:

- Structural proposals instead of only text patches.
- Declaration-level grouping of changes.
- Explicit parent and child edit relationships.
- Reversible application and removal.
- Before and after source views.
- Diagnostics after every projected state.
- Tests and validation connected to the resulting state.
- A durable record of what was proposed and what the human accepted.
- Clear separation between facts, analysis, hypotheses, and human judgment.

The system should help a powerful AI move quickly while keeping the human in
the loop at the points where architecture, behavior, and responsibility are
decided.

## Evidence Categories

The interface should never present every kind of result as equally certain.
Every explanation should distinguish these categories:

### AST Fact

Syntax changed in a specific structural location.

Example:

```text
The body of function App changed.
```

### Reference Fact

An identifier or declaration resolves to another known declaration according
to the available analysis.

Example:

```text
This call references function loadUser.
```

### Call-Analysis Fact

A caller or callee relationship was identified by call analysis.

Example:

```text
App calls loadUser.
```

This should only be stated when the analysis is strong enough to support it.

### Impact Hypothesis

The system believes that a change may affect another file, declaration, or
behavior, but the relationship is not proven.

Example:

```text
This change may affect the API response path.
```

### Human Interpretation

An explicit conclusion or label chosen by the person reviewing the change.

Example:

```text
This change is intentional because it removes the legacy response format.
```

Keeping these categories separate is part of the guardrail. The system should
not turn an AST observation into an architectural claim without showing the
steps in between.

## Suggested First Workflow

Build one complete vertical workflow for one language before expanding the
surface area:

1. Load a real AI-generated patch.
2. Show a familiar before/after source view first.
3. Show the AST explanation collapsed underneath.
4. Group changes by declaration.
5. Let the user accept or reject one structural operation.
6. Show the resulting source immediately.
7. Run validation and tests.
8. Export the resulting patch.

The AST should explain the source change, not replace the source change as the
first thing a person sees.

## What This Should Prove

The project should not be judged by how impressive its tree rendering looks.
It should be judged by whether a person can make a better engineering decision
with it.

A useful evaluation would compare this workflow with a normal Git diff and IDE
workflow on realistic AI-generated patches. Measure whether people can:

- Find the relevant declaration faster.
- Reject an intentionally incorrect change more accurately.
- Understand which changes belong together.
- Accept only the intended structural operation.
- Avoid collateral changes.
- Explain the resulting program state.
- State which conclusions are facts and which are hypotheses.

If people only use the tool as a more complicated diff viewer, the product is
gimmicky. If people use it to understand and safely control changes they would
otherwise blindly accept, the human-ownership thesis has evidence behind it.

## Working Principle

The intended balance is:

```text
AI proposes quickly.
The system exposes structure and evidence.
The human decides deliberately.
Tests and guardrails verify the result.
```

The aim is not to reject powerful AI. The aim is to make powerful AI compatible
with human understanding and responsibility.
