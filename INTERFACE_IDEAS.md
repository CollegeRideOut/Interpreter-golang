# Exploratory Interface Ideas

This document records ideas for the interface. It is intentionally not a
complete implementation plan. The project should move through these ideas one
step at a time instead of attempting to build the entire interface at once.

## The Basic Idea

The interface should make a codebase explorable at different levels of detail.
The user can work at a high, human-friendly level and descend into internals
whenever they want.

```text
human-facing view
    -> structural view
        -> AST view
            -> field and token internals
```

Changing the view must not change the underlying working state. Views are
lenses over the same authoritative engine model.

## Observable Structure And Human Meaning

The engine can expose facts about the code:

```text
workspace
    -> files
        -> packages or modules
            -> declarations
                -> types and class-like structures
                    -> functions and methods
                        -> parameters and return values
                            -> statements
                                -> expressions
                                    -> values and tokens
```

These are observable code structures, though some relationships may require
analysis.

The engine must not pretend that human interpretations are facts. Names such
as these are useful, but they are interpretations:

```text
auth service
user repository
factory
billing domain
```

A user may define such a grouping or label it explicitly. The engine can then
use that grouping as an interface aid, but it should not silently infer or
assert the meaning.

## Files At Human Eye Level

Files should be high in the interface because they are often the most tangible
architectural unit. People open files, name files, move declarations between
files, and form an initial understanding of a codebase through its files.

Packages and modules remain important real boundaries, but they do not always
need to dominate navigation. They can be shown as context and as constraints
around files:

```text
package parser
    parser.go
    errors.go
    tokens.go
```

The interface can show package consequences without claiming to know the
human purpose of the package:

```text
this file belongs to package parser
this declaration is exported
this move changes package membership
these imports cross the package boundary
```

## Multiple Exploratory Lenses

The code should not be forced into one permanent tree. A declaration can be
viewed through several valid paths:

```text
parser.go
parser package
Parser type
Parse method
```

The interface should support questions such as:

```text
Show me the code grouped by file.
Show me the public surface grouped by package.
Show me methods grouped by type.
Show me this function's contract and implementation.
Show me the raw AST for this item.
```

Possible views include:

### File View

```text
file
    imports
    types
    functions
    variables
```

### Package Interface View

```text
package
    exported types
    exported functions
    exported variables
```

This is an interface view of the package, not a claim about what the package
means.

### Type Or Class-Like View

For languages with classes, show fields, constructors, and methods. For Go,
the equivalent view can group a named type with its fields and methods, even
when those methods are declared in different files.

```text
named type
    fields
    methods
    constructor-like functions
```

### Function View

```text
function
    parameters
    return values
    branches
    calls
    implementation
```

Return values are part of the function contract. They are not necessarily a
higher architectural level than a function, but they are an important lens
for understanding how behavior connects.

### Structural And AST Views

Any higher-level item should be expandable into its structural details:

```text
function
    -> if statement
        -> condition
            -> identifier
```

At the lowest level, the user should still be able to inspect exact fields,
node identities, provenance, and materialized edits.

## Directional Exploration

The interface should support moving in either direction:

```text
file
    -> function
        -> condition
            -> identifier
```

and also:

```text
identifier
    -> containing condition
        -> containing function
            -> containing file
                -> containing package
```

This makes the interface exploratory rather than a fixed drill-down menu.

## Transformations At Every Level

The same underlying edits may be expressed at different levels:

```text
file: create, move, split, combine, rename
package: expose, reorganize, change boundary
type: create, move, add behavior, change fields
function: create, duplicate, move, change contract
statement: copy, move, wrap, replace
expression: substitute, rename, replace
```

A high-level action must remain connected to its concrete engine edits. The
interface may show:

```text
copy this behavior into three functions
```

while the engine retains:

```text
insert IfStmt at destination A
insert IfStmt at destination B
insert IfStmt at destination C
```

The user can accept the high-level result or open it to inspect every detail.

## Progressive Disclosure

Every displayed item should make it possible to answer two questions:

```text
What useful thing am I looking at?
What exact code and edits are underneath it?
```

Useful controls may include:

```text
show internals
show raw AST
show affected files
show package boundaries
show exports only
show exact edits
```

The UI can start simple. The important design constraint is that higher-level
views must preserve links to the lower-level items they represent.

## Engine Boundary

The engine should provide reusable projections or lenses over its canonical
model. A view may specify a grouping and a level of detail:

```text
group by file
group by package
group by type
group by function
show exports
show declarations
show contracts
show implementation
show internals
```

The engine should return observable entities with stable identities and links
back to source locations and edit indexes. A UI should render the projection,
manage navigation state, and request deeper views.

Lifting or projection must not mutate the working state. Applying or removing
a projected item must operate on the canonical edits through the engine.

## Step-By-Step Development

This idea should be built incrementally:

1. Keep the canonical structural edit model stable.
2. Make raw and lifted edit projections reusable in the engine.
3. Add file-level grouping and navigation.
4. Add declaration, type, and function views.
5. Add package boundary and export views.
6. Preserve expansion from every view into exact internals.
7. Add user-defined labels and groupings only when the observable views are
   dependable.
8. Add higher-level directional transformations after the views make their
   destinations clear.

Do not build all levels in one pass. Each new view should prove that it can
show useful structure, preserve identity, and lead back to the same working
state and concrete edit history.

## Guiding Principle

The engine exposes structure and evidence. The human chooses the grouping,
interpretation, and direction.

The interface should let a person say:

```text
Show me this by file.
Now show me the package surface.
Now show me the type-level view.
Now show me the exact internals.
```

All of those should be different ways of exploring the same codebase, not
different representations that lose one another.
