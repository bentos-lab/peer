# Coding Agent Abstraction

## Purpose

Define a provider-agnostic abstraction for creating and running coding agents without coupling `usecase` to a concrete provider implementation.

## Layer Ownership

- `domain`
  - Shared value types:
    - `CodingAgentSetupOptions`
    - `CodingAgentRunOptions`
    - `CodingAgentRunResult`
- `usecase/contracts`
  - Ports/interfaces:
    - `CodingAgent`
    - `CodingAgentEnvironment`

## Lifecycle

1. Create an environment with `Factory.New`.
2. Prepare an agent with `SetupAgent`.
3. Execute one task with `Run` and per-run options.

Example flow:

```go
env, err := factory.New(ctx, domain.CodeEnvironmentInitOptions{
	RepoURL: "https://github.com/example/repo.git",
})
if err != nil {
	return err
}

agent, err := env.SetupAgent(ctx, domain.CodingAgentSetupOptions{
	Agent: "opencode",
	Ref:   "refs/heads/main",
})
if err != nil {
	return err
}

result, err := agent.Run(ctx, "Task abc", domain.CodingAgentRunOptions{
	Provider: "gemini",
	Model:    "gemini-3-pro-preview",
})
if err != nil {
	return err
}

_ = result.Text
```

## Provider and Model Resolution

- `CodingAgentRunOptions.Provider` and `CodingAgentRunOptions.Model` are optional when using the `opencode` host agent.
- If both are empty, the agent runs without `--model` and opencode uses its default.
- If provider is set and model is empty, the agent queries available models for the provider and selects a default if available.
- If provider is empty and model is set, the agent clears the model and logs a warning.

## Host Environment Behavior

- `Factory.New` with `RepoURL` empty:
  - uses current working directory as workspace for token refs (`@staged`, `@all`); empty ref defaults to `@all`,
  - for non-token refs, reads `remote.origin.url`, clones into `~/.bentos-labtmp`, and operates on that clone.
- `Factory.New` with `RepoURL` non-empty:
  - creates a random temporary workspace under `~/.bentos-labtmp`,
  - runs shallow clone (`git clone --depth 1`),
  - fetches refs (`git fetch --all --prune`).
- `SetupAgent` with `Ref` non-empty:
  - token refs:
    - `@staged`: workspace staged mode (not a git ref), skip checkout/sync.
    - `@all`: workspace full mode (not a git ref), skip checkout/sync.
  - local workspace (token refs only): no checkout/sync beyond token handling.
  - cloned workspace: verify ref availability (with fetch recovery if needed) -> `git checkout <ref>`.
  - token refs are supported only in local workspace mode (`RepoURL` empty). When `RepoURL` is provided, `Ref` must be a real git ref/commit.
- `Agent` is selected in `SetupAgent` by string; current host implementation supports only `opencode`.

## Extending with Adapters

Concrete implementations should be added in outbound adapters and wired through `wiring`. This keeps the contract stable while allowing different providers to implement the same `CodingAgentEnvironment` and `CodingAgent` interfaces.
