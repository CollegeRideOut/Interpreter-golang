# contuts

`contuts` is a code-understanding and structural program-transformation tool.
It is being built for a human who wants to understand a change all the way
from the repository state to the smallest affected declaration.

The product is the thing that matters. The architecture matters because it is
what makes the product trustworthy, understandable, and capable of growing
without losing its mental model.

AI can make this workflow noisy, repetitive, and frustrating. That is not a
reason to give up on thinking carefully about the system. A product builder
should be able to obsess over the architecture, define the model precisely,
and still use AI as leverage. The human owns the model and the target state.

## Desired Workflow

The primary workflow is revision comparison followed by declaration-level
inquiry.

```text
open a program
    -> choose the current revision
    -> choose the revision to compare against
    -> generate edits
    -> inspect the complete canonical edit tree
    -> split edits into declaration inquiries and related columns
    -> apply, remove, or inspect individual structural edits
    -> continue exploring the resulting program state
```

### 1. Open A Program

The initial inquiry opens the program, packages, files, declarations, imports,
and source representation. It is the starting context, not the final answer.

The explorer is a sequence of equivalent columns:

```text
program
    -> package
        -> file
            -> declaration
                -> import, reference, caller, callee, dependent, or affected code
```

Opening a target in the current column answers the current question. Opening it
in a new column preserves the question that led there. Opening it to the left
connects the result to an earlier context.

### 2. Choose Two Revisions

The comparison controls remain explicit because generating edits between two
program states is the central operation.

The current revision may be the working tree or a commit. The comparison
revision may be a branch or commit. Selecting revisions does not silently
rewrite the checked-out working tree.

The user then presses **Generate edits**. Edit data is loaded for every known
file in the inquiry. There is no hidden "show complete file" filter: the
comparison always presents the complete canonical edit set.

### 3. Read The Complete Edit Tree

The comparison view shows the structural tree, not only a lifted or simplified
summary:

```text
file
    -> declaration/specification
        -> changed AST node
            -> changed child
                -> changed descendant
```

Every canonical edit keeps its identity, parent identity, ancestor path, source
location, operation kind, and application status. A structural replacement may
require related child operations; those relationships remain visible.

The complete tree is important because a file can contain several independent
functions, methods, types, literals, fields, or expressions. Showing only one
lifted operation loses the causal structure needed to understand and control
the change.

## Inquiry And Column Rules

The unit of organization is not merely the file. Files are containers; the
meaningful inquiry boundary is the declaration and its structural ancestry.

### Separate Inquiries

Create separate inquiries when edits are independent siblings:

- Two top-level functions changed independently.
- Two methods changed independently.
- Two top-level types changed independently.
- Two unrelated declarations in the same file changed independently.
- Changes in unrelated files have no edited declaration ancestor connecting them.

Each inquiry should be named after the narrowest meaningful declaration when
possible, such as `function Remaining` or `struct Server`, and should retain
the source file as context.

The desired shape is:

```text
inquiry: function First
    controllers/example.go

inquiry: function Second
    controllers/example.go
```

These are not two file-level copies of the same change. They are two declaration
stories in the same file.

### Same Inquiry, Separate Columns

Keep edits in one inquiry when they share an edited ancestor, but use columns
for independent cousin branches below that ancestor.

```text
inquiry: function Build

column 1: edited Build parent -> branch A
column 2: edited Build parent -> branch B
```

This applies recursively. If two edits are siblings and their parent is also
edited, the parent establishes the shared inquiry and the sibling branches are
separate columns. If the shared edited ancestor is a grandparent, cousin edits
under that grandparent remain one inquiry with separate columns.

The rule is based on AST identity and ancestry, not just line ranges or file
names:

```text
same edited declaration or ancestor -> same inquiry
independent sibling declaration    -> separate inquiry
same edited ancestor, cousin branch -> separate columns
no declaration ancestor             -> file-level fallback inquiry
```

### Declaration Boundaries

Grouping must descend at least to declaration-level nodes. The relevant
boundaries include:

- Functions.
- Methods.
- Type declarations.
- Struct and interface declarations.
- Constants and variables.
- Import declarations when no more meaningful declaration owns the edit.
- Nested declarations and their AST descendants.

An edit inside a function body belongs to that function inquiry. An edit inside
a method belongs to that method inquiry, even when the method is attached to a
type. An edit inside a type specification belongs to the type inquiry. Only
edits that cannot be associated with a declaration use the file as their
fallback context.

## Applying Edits

Every edit is individually inspectable and controllable:

```text
unapplied
    -> apply
        -> prepared/applied
    -> remove
        -> unapplied/removed
```

Applying a child may require materializing an edited ancestor. That does not
erase the canonical child edit. Removing an edit must remove only the requested
operation and any temporary projected structure that exists solely to support
it.

The engine distinguishes:

- Canonical edits: the complete source-to-target edit set.
- Lifted edits: a useful projection for compact views.
- Projected application: the concrete operations required to materialize one
  selected canonical edit.
- Rendered working code: the current intermediate program representation.

The UI may offer a convenient replacement action for a delete, but the
underlying operation remains structural and traceable.

## Explorer Workflow

The comparison workflow and dependency workflow are connected but distinct.
The explorer answers questions about code relationships:

```text
start here
    -> why does this depend on that?
    -> where is this declaration used?
    -> what calls this function?
    -> what changed inside this declaration?
    -> what related declaration should I inspect next?
```

Every column should expose the same basic lenses:

- Packages and files.
- All declarations.
- Exported API.
- Source code.
- Imports and references.
- Callers and callees when available.
- Dependents and affected code when available.
- Structural edits and exact edit context.

Repeated targets are marked as previously opened instead of silently creating a
confusing cycle. Unrelated questions start a separate inquiry rather than
polluting the current path.

## Structural AST Model

The engine parses Go into a structural tree and tracks identity across source
and target revisions. Numeric indexes and raw AST pointers are not durable
identities. Edits therefore retain:

- Node identity and global identity.
- Source and target identity where both exist.
- Parent and ancestor identity.
- AST field and position.
- Source line range.
- Operation kind and status.
- Render diagnostics.
- Working-state revision.

The renderer must preserve useful syntax for common Go nodes, including type
specifications, composite literals, key/value elements, arrays, maps, structs,
interfaces, functions, and their nested expressions. Incomplete intermediate
states are allowed, but they should be rendered with explicit diagnostics
rather than silently discarded.

Parsing, rendering, compilation, type checking, and tests are observations of a
program state. They should inform the next inquiry, not erase the state.

## Human Direction And AI Leverage

The human chooses the target state, the revisions, the inquiry boundaries, and
which edits are accepted. AI may locate code, propose an edit, explain a
relationship, or iterate toward a structural contract. AI must not silently
decide what becomes part of the working program.

```text
human defines the goal
    -> AI proposes a bounded operation
    -> contuts exposes the exact AST edits
    -> human inspects declaration and ancestry grouping
    -> human applies, removes, or changes an edit
    -> contuts renders and verifies the resulting state
```

The product is not primarily an AI summary viewer. A summary is a claim. The
valuable artifact is the visible path from one program state to another:

- The source location.
- The declaration that owns the change.
- The relationship that caused it to be shown.
- The complete structural edit tree.
- The resulting working source.
- Diagnostics and verification results.
- Provenance back to the original operation.

The architecture should make that mental model possible. It should be possible
to ask not only whether a change is good, but exactly which declaration changed,
which parent made the change necessary, which cousin branch is independent, and
what state will exist after applying it.

## Current Prototype

The repository currently contains experiments for:

- Go AST parsing, export, and field-aware structural diffs.
- Canonical insert, update, delete, and replacement operations.
- Parent/ancestor identity and reconciliation.
- A mutable intermediate working tree.
- Reversible edit application and removal.
- Best-effort rendering of incomplete states.
- Declaration, package, file, import, and reference exploration.
- Declaration-level comparison inquiries and cousin columns in the React UI.
- A Wails desktop application.
- A multi-package calorie-app fixture for dependency exploration.

The prototype is not yet a complete multi-file transformation environment.
Reference, call, type, and impact analysis are still developing. The important
near-term goal is to make the inquiry and edit model correct and understandable
before adding more automation.

## Running It

Run the Go prototype:

```bash
```

Build or test the desktop application:

```bash
cd desktop
```

The packaged desktop binary is written to:

```text
```

The default example uses `TestProgram/`. `TestProgramCalorieApp/` is a
multi-package fixture for package, import, declaration, and revision workflows.

## Product Test

The product is moving in the right direction when a user can say:

```text
Start here.
Compare these two revisions.
Generate the complete edits.
Show me the functions and types affected.
Separate independent sibling declarations.
Put cousin branches under the same edited parent into columns.
Keep this edit, remove that one, and continue from the resulting state.
```

If the user can see the code, the declaration boundaries, the ancestry, the
chosen edits, and the resulting working states, `contuts` is doing useful work.

The older project notes are preserved in [`oldREADME.md`](oldREADME.md),
[`oldDIRECTION.md`](oldDIRECTION.md), and
[`INTERFACE_IDEAS.md`](INTERFACE_IDEAS.md).
