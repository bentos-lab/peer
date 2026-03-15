# System Prompt Guidelines

## Purpose

These guidelines define how to author system prompts and task prompts for coding agents and LLM tooling in this repository.

## Principles

- Keep prompts scoped to the task and repository rules.
- Use deterministic, unambiguous instructions.
- Avoid toggle-only behavior; author templates with explicit branching but do not expose branching in the rendered prompt.
- Align instructions with existing repository conventions rather than inventing new ones.
- When authoring new system prompts or task prompts, follow existing task.md templates and this document as the baseline; do not introduce new patterns unless explicitly required.
- Coding-agent `task.md` templates must not embed file contents or diff snippets (for example, no “Changed files and contents” sections).

## Conditional Instructions

- Prompts must be opaque to internal logic; only include instructions for active modes.
- Disabled modes must not be mentioned in the rendered prompt.
- Apply this rule to all future prompt features and modes, not only docs/tests.

## Naming
- task for code agent.
- system prompt for llm generator.

## Docs and Tests Guidance

- When prompts request documentation, instruct the agent to follow the repo's existing documentation style and to add to established documentation locations already used in the repo.
- When prompts request tests, instruct the agent to follow the repo's existing test framework, naming conventions, and structure.
- Keep generated work minimal, scoped to the diff, and aligned with repository rules.
