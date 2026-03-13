# Custom Recipe

Custom recipes let a repository provide extra guidance for review, overview, reply, and autogen flows.

## Example `.autogit/config.toml`

```toml
[review]
enabled = true
ruleset = "rules.md"
suggestions = true

[overview]
enabled = true
extra_guidance = "overview.md"

[overview.issue_alignment]
enabled = true

[autoreply]
enabled = true
extra_guidance = "reply.md"

[autogen]
enabled = true
extra_guidance = "autogen.md"
```

All file paths are resolved relative to `.autogit/`.
