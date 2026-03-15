# Custom Recipe Env

These environment variables provide default custom-recipe values when `.autogit/config.toml` does not set them. CLI flags take precedence over `config.toml`, and `config.toml` takes precedence over env defaults.

Boolean values accept `true`/`false`. List values are comma-separated and are case-insensitive.

## Review
- `REVIEW_ENABLED`
- `REVIEW_SUGGESTED_CHANGES`
- `REVIEW_EVENTS` (pull_request actions)

## Overview
- `OVERVIEW_ENABLED`
- `OVERVIEW_EVENTS` (pull_request actions)
- `OVERVIEW_ISSUE_ALIGNMENT_ENABLED`

## Autoreply
- `AUTOREPLY_ENABLED`
- `AUTOREPLY_EVENTS` (`issue_comment`, `pull_request_review_comment`)
- `AUTOREPLY_ACTIONS` (comment actions like `created`)

## Autogen
- `AUTOGEN_ENABLED`
- `AUTOGEN_EVENTS`
- `AUTOGEN_DOCS`
- `AUTOGEN_TESTS`
