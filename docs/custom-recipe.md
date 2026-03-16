# Custom Recipe

Custom recipes let a repository provide extra guidance for review, overview, reply, and autogen flows.

Precedence (highest to lowest): CLI flags > `.autogit/config.toml` > environment defaults. See [Configuration](/docs/configuration.md#precedence). When a key is present in `config.toml`, it overrides env defaults even if empty or false.

## Example `.autogit/config.toml`

```toml
[review]
enabled = true
ruleset = "rules.md"
suggestions = true
events = ["opened", "reopened", "synchronize"]

[overview]
enabled = true
extra_guidance = "overview.md"
events = ["opened"]

[overview.issue_alignment]
enabled = true
extra_guidance = "issue_alignment.md"

[replycomment]
enabled = true
extra_guidance = "reply.md"
events = ["issue_comment", "pull_request_review_comment"]
actions = ["created"]

[autogen]
enabled = true
extra_guidance = "autogen.md"
events = ["opened"]
docs = true
tests = true
```

All file paths are resolved relative to `.autogit/`.
