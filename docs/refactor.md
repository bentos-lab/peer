## General

The agent is allowed to aggressively refactor Go code to improve readability, maintainability, and architectural consistency. The goal is to simplify logic, reduce duplication, and keep the codebase clean and idiomatic.

### General Principles

- Prefer **clarity and simplicity** over preserving existing structure.
- Remove unnecessary abstractions, wrappers, or helper functions.
- Reduce cognitive complexity of functions and files.
- Avoid premature generalization.
- Favor idiomatic Go patterns.
- Always aggressively cleanup any unnecessary components (tests, functions, methods, structs, fields, etc.).
- Change file name for consistent if necessary.

### Function Refactoring

The agent MAY:

- **Inline small functions** that:
  - are used only once
  - only wrap another function
  - do trivial transformations
  - only pass parameters through

- **Delete unnecessary components**.

- **Merge related functions** if:
  - they are tightly coupled
  - they are always called together
  - they represent a single logical operation

- **Split large functions** when:
  - they exceed reasonable logical complexity
  - they mix multiple responsibilities
  - they contain long nested logic

- **Rename functions** to better reflect behavior.

- Replace deeply nested logic with:
  - early returns
  - guard clauses

### Struct and Type Refactoring

The agent MAY:

- Remove unused structs or fields.
- Merge structs that represent the same concept.
- Remove unnecessary interfaces when only one implementation exists.
- Replace interfaces with concrete types if abstraction has no value.

### Package Structure

The agent MAY:

- Move files between packages to better reflect domain boundaries.
- Merge small packages that contain very little logic.
- Remove unnecessary layers or pass-through code.

### Code Duplication

The agent MUST:

- Detect duplicated logic and extract it into a shared helper if reused multiple times.
- Alternatively inline duplicated code if abstraction would harm readability.

### Error Handling

- Simplify repetitive error handling.
- Avoid wrapping errors multiple times unnecessarily.
- Remove redundant error propagation layers.

### Control Flow

Prefer:

- early returns
- flat logic
- minimal nesting

Avoid:

- deeply nested `if`
- unnecessary `else` blocks
- complex switch logic that can be simplified

### Dead Code

The agent MUST remove:

- unused variables
- unused constants
- unused functions
- unreachable code
- commented-out code blocks

### Imports

- Remove unused imports.
- Group imports according to Go conventions.

### Logging

- Remove redundant logging layers.
- Avoid logging the same error multiple times across layers.

### Refactor Safety

The agent MUST ensure:

- public API behavior remains unchanged unless explicitly instructed.
- exported function signatures remain compatible if used externally.
- tests continue to pass after refactoring.

### When Aggressive Refactoring is Allowed

The agent MAY perform larger structural refactors if it significantly improves:

- readability
- maintainability
- architectural consistency

This includes:

- deleting unnecessary layers
- collapsing indirection
- merging thin abstractions
- simplifying data flow

### Refactor Verification Tools

After performing refactoring, the agent MUST verify that the code still builds, passes tests, and maintains basic code quality.

The following commands MUST be executed:

Build check:

    go build ./...

Run tests:

    go test ./...

Static analysis:

    go vet ./...

Formatting check (must not introduce formatting issues):

    gofmt -s -w .

Dependency consistency check:

    go mod tidy

The agent MUST ensure:

- the project builds successfully
- all tests pass
- no new vet errors are introduced
- formatting follows standard Go style

If any verification step fails, the agent MUST fix the issue before completing the refactor.
