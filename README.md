# contuts

> How much can we understand about a code change without asking an AI what happened?

`contuts` is an experimental code-analysis project exploring deterministic ways to understand how source code changes.

The project currently starts at **AST differencing**: parse two versions of a program, match their syntax trees, and reconstruct the structural operations that transform one into the other.

The longer-term rabbit hole goes further:

**tree edit distance → AST matching → structural differencing → static analysis → control flow → code understanding**

This is not an AI code summarizer.

The interesting question is what we can derive from the program itself.

## Why

A normal diff tells us what text changed:

```diff
- return x * 0.18
+ return x * 0.20
```

An AST gives us structure:

```text
FunctionDeclaration
└── ReturnStatement
    └── InfixExpression
        ├── Identifier(x)
        ├── Operator(*)
        └── Number(0.20)
```

An AST differ can potentially tell us:

```text
UPDATE InfixExpression
└── Number
    0.18 → 0.20
```

And static analysis can eventually tell us more:

```text
changed function
    ↓
references
    ↓
callers / callees
    ↓
control-flow impact
    ↓
data-flow relationships
```

These are different from statements like:

> "This change fixes the tax calculation."

That is an interpretation of the change.

`contuts` is interested in the layer underneath it: **mechanically derived evidence about what changed and how the affected program is structured.**

## The experiment

Given:

```text
before.go
    ↓
  parser
    ↓
  AST A ─────────┐
                 │
              matcher
                 │
  AST B ─────────┘
    ↑
  parser
    ↑
after.go
```

Can we produce a useful structural description such as:

```text
MATCH   FunctionDeclaration calculate → calculate
MATCH   ReturnStatement → ReturnStatement

UPDATE  NumericLiteral
        0.18 → 0.20
```

For simple changes, yes.

The interesting problems start when the trees stop lining up nicely.

```text
BEFORE                  AFTER

Program                 Program
├── let x = 1           ├── let x = 1
├── let y = 2           └── let z = 3
└── let z = 3
```

A naive positional matcher might decide:

```text
MATCH   x → x
UPDATE  y → z
DELETE  z
```

A human would probably describe it as:

```text
MATCH   x → x
DELETE  y
MATCH   z → z
```

Determining those mappings is where AST differencing gets interesting.

## Current status

Very early.

This repository is intentionally being built from simple implementations upward rather than starting by wrapping an existing AST-diff library.

Current exploration:

- [x] Lexer/parser/AST fundamentals
- [x] Basic tree edit distance
- [ ] Naive AST differ for the Monkey language
- [ ] Node mapping experiments
- [ ] Insert / delete / update edit scripts
- [ ] Moves and reordered subtrees
- [ ] Subtree similarity
- [ ] GumTree-style matching
- [ ] Compare against existing AST differs

Later:

- [ ] Symbol and reference analysis
- [ ] Call graphs
- [ ] Control-flow graphs
- [ ] Data-flow analysis
- [ ] Structural change visualization
- [ ] Editor experiments

None of that roadmap is sacred. This is a research project; following interesting failures is part of the point.

## Why Monkey?

The first AST differ is being implemented against the small language from *Writing an Interpreter in Go*.

That's deliberate.

Real languages have enormous syntax trees and years of edge cases. Monkey is small enough that both ASTs can be understood by hand, making bad mappings obvious.

The goal isn't to build the world's greatest Monkey differ.

The goal is to understand **why building a good differ is hard**.

## Principles

### Deterministic first

If information can be derived mechanically from source code, prefer that over generating an explanation.

### Facts and interpretation are different

```text
Git diff       → text changed
AST diff       → structure changed
Types          → symbol relationships
Call graph     → possible calls
CFG            → possible execution paths
Data flow      → possible value propagation

────────────────────────────────────

Human / AI     → interpretation and intent
```

Both layers can be useful. They should not be confused.

### Build the dumb version first

This repository will contain bad algorithms. That's intentional.

A naive implementation that fails on:

```text
[a, b, c]
    ↓
[a, c]
```

teaches more about the matching problem than immediately importing a mature implementation.

Break it. Understand why. Then improve it.

## Research rabbit holes

Some of the work informing this project includes:

- Tree Edit Distance
- Zhang–Shasha
- GumTree
- Diff/AST
- RefactoringMiner
- AST mapping and edit-script generation
- static program analysis
- control-flow and data-flow analysis

The project isn't attempting to reproduce all of them. They're increasingly sophisticated answers to questions this repository is trying to encounter naturally.

## Eventually

The vague destination is an environment where a developer can explore a change structurally:

```text
                CODE CHANGE
                     │
              ┌──────┴──────┐
              │             │
           Git Diff      AST Diff
                            │
                      Changed Symbol
                            │
                 ┌──────────┴──────────┐
                 │                     │
              Callers                 CFG
                 │                     │
              Types                Data Flow
                 └──────────┬──────────┘
                            │
                         HUMAN
```

Instead of immediately asking:

> "AI, explain this diff to me."

the developer should be able to ask the program itself:

> **What changed?**

> **Where is it?**

> **What does it connect to?**

> **Where can execution go from here?**

Then the human can decide what it means.

---

Mostly, though, this repository is an excuse to get unreasonably nerdy about trees.
