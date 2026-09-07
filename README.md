# contuts

> A tiny structural code canvas for people who looked at a four-line diff and decided they needed an AST editor.

`contuts` is an experimental Go project for exploring whether a program can explain and transform a code change using deterministic evidence instead of immediately asking an AI to summarize or rewrite it.

It is also, currently, a Raylib window containing one hard-coded Go calculator, four edit scripts, and a frankly unreasonable amount of machinery for changing `-` to `+`.

## What It Does Today

The program compares two in-memory Go source strings:

```go
// before
case '+':
    return left - right
case '-':
    return left - right
```

```go
// after
case '+':
    return left + right
case '-':
    return left - right
case '*':
    return left * right
case '/':
    return left / right
case '%':
    return left % right
```

It then:

- Parses both versions into Go ASTs.
- Converts those ASTs into GumTree-style trees.
- Builds a small structural edit script containing one update and three insertions.
- Displays the affected original AST nodes in a Raylib UI.
- Displays the current working source beside the AST.
- Lets each edit be selected and applied independently.
- Lets a selected AST subtree apply all of its edits at once.
- Supports undo and redo for individual edits.
- Keeps everything in memory. Nothing is written to disk.

The UI is intentionally focused on the original AST. Inserted cases do not magically become nodes in the original tree; they appear as separate edit rows attached to the affected source node instead.

## Human Agency, With AI Leverage

The point is not to make AI independently rewrite code, or to make a passive code-review viewer. The point is to let a person explore structural changes, choose what they want, and continue from the code state they created.

```text
AI proposes possibilities
Human explores and chooses
The system preserves those choices
AI continues from the chosen state
```

Applying and removing edits are therefore intentional parts of the interaction. They are not accept/reject buttons for a review; they are a way to express intent without manually rewriting syntax. The user can keep one transformation, remove another, inspect the consequences, and treat the resulting working source as their current state.

The longer-term flow is:

```text
structural diff
    -> user applies and removes edits
    -> current working code
    -> OpenCode continues from that exact state
```

See [`DIRECTION.md`](DIRECTION.md) for the working product direction and next-step list.

## Running It

This is a Go project using Raylib through `raylib-go`.

```bash
go run .
```

Build it with:

```bash
go build .
```

The current UI expects this font to exist:

```text
/usr/share/fonts/liberation/LiberationSans-Regular.ttf
```

That is not a portable application packaging strategy. It is a prototype on a Linux machine that currently has that font installed.

## Controls

- `Up` / `Down`: move through affected AST/edit rows.
- `Enter`: apply the selected edit.
- `Shift+Enter`: apply all edits in the selected subtree.
- Click an edit in the footer: select that individual edit.
- `Ctrl+Z`: undo the most recent applied edit.
- `Ctrl+Y`: redo the most recently undone edit.

The three inserted cases are separate edits. Applying `case '*'` does not also apply `/` or `%`, despite all three initially sharing the same insertion anchor. The project has tests specifically for applying, undoing, redoing, and repeating that cycle.

## Architecture, Such As It Is

The interesting prototype code currently lives in:

- `gumtree_diff.go`: in-memory examples, Go AST conversion, edit-script generation, source ranges, and edit history.
- `main.go`: Raylib window, AST rows, source display, selection, and keyboard/mouse handling.
- `gumtree_diff_test.go`: calculator diff and edit-history tests.
- `third_party/gumtree-go`: a local GumTree fork used for AST comparison and mappings.

The local GumTree copy is used through this module replacement:

```go
replace github.com/Xanonymous-GitHub/gumtree-go => ./third_party/gumtree-go
```

The fork currently supplies mappings and comparison support. The edit script shown by the UI is still custom code. This distinction matters because saying "GumTree generated the edit script" would be more impressive than accurate.

## Tests

Run all tests with:

```bash
go test ./...
```

The tests currently cover:

- The expected calculator edit count and edit kinds.
- The generated source matching the target after all edits.
- Independent AST rows for each edit.
- Basic update undo/redo.
- Repeated apply, undo, and redo cycles for all calculator edits.

## Related Code

The selected edit should eventually show more than the changed syntax. A nearby context panel should make it easy to inspect:

- The containing function or declaration.
- Identifiers inside the changed node.
- Where those identifiers are defined and used.
- Direct callers and callees.
- Related files and top-level functions.
- Available expression, parameter, and return types.

The AST diff tells us what syntax changed. Semantic analysis tells us what code may be related or affected. Those claims should remain separate:

```text
AST diff        -> syntax changed
Type analysis   -> types changed or stayed the same
References      -> definitions and uses
Call analysis   -> callers and callees
Impact analysis -> code that may observe the change
```

## Why This Exists

A text diff can tell us that this happened:

```diff
- return left - right
+ return left + right
```

An AST-oriented tool can describe it as an update to a binary expression, plus three inserted case clauses. That structural description is useful evidence. It is not intent, and it does not prove that the calculator is now correct.

That boundary is the point of the experiment:

```text
text diff       -> bytes changed
AST diff        -> syntax changed
symbol analysis -> relationships changed or affected
control flow    -> paths that may change
human judgment  -> what the change means
```

The project wants to explore the first four layers before pretending the fifth can be automated by putting a chatbot in a panel. Later, OpenCode should be able to use the user-created working state, including which edits were kept and removed, rather than starting over and guessing intent.

## The Roadmap, In The Most Technically Honest Order

- Replace the hard-coded source strings with real files.
- Generate more complete insert, delete, update, and move scripts.
- Improve mappings when AST children are inserted or reordered.
- Make source ranges robust for overlapping and interacting edits.
- Add proper structural visualization instead of colored text rows.
- Add symbols, references, call graphs, control-flow, and data-flow analysis.
- Add affected-code context for selected edits.
- Preserve named working states and edit provenance.
- Let OpenCode continue from the user's current code state.
- Eventually become useful.

The last item is aspirational.

## Current Limitations

This is not yet:

- A general-purpose Go diff tool.
- A conventional code editor.
- A persistent review application.
- A complete GumTree implementation.
- An interpreter-driven semantic analyzer.
- An AI replacement.

It is a deliberately small laboratory for finding out how much expressive power and complexity appears when you let a user independently apply and reverse three inserted `case` clauses.

That complexity is the feature. It is the bug we are studying.

## The Larger Rabbit Hole

The long-term idea is to move from:

```text
source change
    -> AST mapping
    -> edit script
    -> affected symbols
    -> callers and callees
    -> control-flow impact
    -> data-flow impact
```

Until then, `contuts` is mostly an excuse to get unreasonably nerdy about trees, source offsets, and undo stacks.
