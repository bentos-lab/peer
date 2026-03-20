# Changelog

All notable changes to this project are documented here.

## [0.2.3] - 2026-03-20
### Changed
- Reordered inbound job dependencies to run autogen before overview.

## [0.2.2] - 2026-03-20
### Changed
- Improved reviewer prompt handling and safety checks.

## [0.2.1] - 2026-03-19
### Changed
- Skip VCS resolution and repository clone when the requested ref already exists locally.

## [0.2.0] - 2026-03-19
### Added
- AI-powered commit command with conventional message generation.
- GitHub Actions Go test workflow.
- Apache License 2.0.
### Changed
- Removed auth hint stderr message and added verbosity levels.
- Updated peer skill text for readability.
- Clarified CLI README instructions.
- Updated release note content.
### Fixed
- Install scripts.

## [0.1.0] - 2026-03-18
### Added
- CLI install workflow with safety prompt and GitHub CLI integration to streamline setup.
- Coding-agent LLM generator plus GitHub client support for CLI commands.
- VCS provider abstraction with GitLab webhook flow, line-side support, and scaffolding.
- Custom recipes, issue alignment orchestration, and config-based recipe handling in the agent flow.
### Changed
- Renamed `autogit` to `peer`, reorganized entrypoints, and refreshed install/update UX.
- Separated the overview and review pipelines, split job queue handling, and unified the safety sanitizer architecture.
- Streamlined CLI, webhook, and code environment handling with richer abstractions for adapters and outbound agents.
### Fixed
- Addressed failing tests and documented stability tweaks ahead of the first release.
