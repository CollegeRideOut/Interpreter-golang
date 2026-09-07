# Direction

## The Idea

`contuts` is an attempt to combine human agency with the power of AI.

The goal is not to make AI independently rewrite code or to make a passive code-review viewer. The goal is to give a person a structural way to explore code changes, choose what they want, and continue from the code state they created.

```text
AI proposes possibilities
Human explores and chooses
The system preserves those choices
AI continues from the chosen state
```

The user should be able to express intent without manually writing every piece of syntax:

```text
Keep this edit.
Remove that edit.
Show me what is affected.
Continue from the version I chose.
```

## What The App Is

The app is a structural code canvas combining:

- AST and structural diffs.
- Individually selectable edit scripts.
- Apply and remove operations.
- Undo and redo.
- Before and after views for each edit.
- A cumulative working code state.
- Related-code and affected-code context.
- Future OpenCode integration.

Applying an edit is not the same as accepting a code review. It is the user exploring and shaping a possible code state.

Removing an edit is not necessarily rejecting the entire proposal. It means the user wants to continue from a version without that transformation.

## Example Flow

```text
Original code
    |
    v
Structural edit scripts
    |
    +-- apply edit A
    +-- remove edit B
    +-- keep edit C
    v
Current working code
    |
    v
OpenCode continues from this state
```

Example:

```text
Applied:
- change subtraction to addition
- add multiplication
- add modulo

Removed:
- add division

Current state:
- calculator with +, -, *, %
```

The user should be able to hand this state to OpenCode and say:

> I kept these changes, removed those changes, and want to continue from here.

## Review And Exploration

The app should provide both kinds of visibility:

### Per-edit clarity

For the selected edit, show:

- What AST node changed.
- The exact before source.
- The exact after source.
- The file and location.
- The containing function or declaration.

### Cumulative exploration

Keep showing the working source as edits are applied and removed. This is the interactive part of the tool and should remain central, not hidden as a debug feature.

## Affected Code

The selected edit should expose closely related code in an easy-to-reach panel:

- Identifiers inside the changed node.
- Where those identifiers are defined.
- Other uses of those identifiers.
- The containing function.
- Functions called by the containing function.
- Functions that call the containing function.
- Related files.
- Top-level functions and declarations involved.
- Expression, parameter, and return types when available.

The AST diff tells us what syntax changed. Semantic analysis tells us what code may be related or affected.

Keep these claims separate:

```text
AST diff        -> syntax changed
Type analysis   -> types changed or stayed the same
References      -> definitions and uses
Call analysis   -> callers and callees
Impact analysis -> code that may observe the change
```

## Principles

- Human direction comes first.
- AI should provide leverage, not remove agency.
- Every transformation should be inspectable.
- Every applied transformation should be reversible.
- The user should not need to apply an edit to understand it.
- The user should be able to apply an edit to explore its consequences.
- The current working state belongs to the user.
- Preserve provenance: remember where each working change came from.
- Prefer explicit facts over invented summaries.

## Next Steps

### Edit presentation

- [ ] Show exact before and after snippets for the selected edit.
- [ ] Highlight the selected edit in both source views.
- [ ] Keep each edit independently selectable.
- [ ] Show a clear applied/removed/current status.

### Working state

- [ ] Treat the working source as a first-class user state.
- [ ] Preserve original source, target source, and current source separately.
- [ ] Track applied, removed, and unapplied edits explicitly.
- [ ] Add named checkpoints for useful working states.
- [ ] Allow returning to a checkpoint.
- [ ] Preserve edit history and provenance when moving between states.

### Affected code

- [ ] Find the containing function and declaration for each edit.
- [ ] Collect identifiers from the affected AST node.
- [ ] Resolve identifier definitions and uses with `go/types`.
- [ ] Find direct callers and callees.
- [ ] Show related files and top-level functions.
- [ ] Display available parameter, expression, and return types.
- [ ] Add navigation from a related item back to its source location.

### OpenCode integration

- [ ] Define a serializable representation of the current working state.
- [ ] Include applied and removed edit history.
- [ ] Include the selected edit and affected-code context.
- [ ] Let the user send the current state to OpenCode.
- [ ] Support prompts such as: continue, explain, refactor, or generate the next edit.
- [ ] Make OpenCode aware of what the user deliberately kept and removed.

### Generalization

- [ ] Replace hard-coded in-memory files with real files.
- [ ] Support multi-file packages.
- [ ] Improve insert, delete, update, and move edit scripts.
- [ ] Make source ranges robust for interacting edits.
- [ ] Keep the small calculator example as a reliable demonstration.

## The Core Test

The app should make this interaction feel natural:

```text
The AI suggests a change.
I inspect what it changes and what depends on it.
I keep some parts and remove others.
I see the code state I created.
I ask the AI to continue from that exact state.
```

That is the product direction: human agency, made more expressive by structural tools and AI assistance.
