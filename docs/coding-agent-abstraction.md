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

1. Create an environment implementation (adapter implementation is out of scope for this version).
2. Prepare an agent with `SetupAgent`.
3. Execute one task with `Run` and per-run options.

Example flow:

```go
env := NewEnvironment()
agent, err := env.SetupAgent(ctx, domain.CodingAgentSetupOptions{
	RepoURL: "https://github.com/example/repo.git",
})
if err != nil {
	return err
}

result, err := agent.Run(ctx, "Task abc", domain.CodingAgentRunOptions{
	Provider: "openai",
	Model:    "o4-mini",
})
if err != nil {
	return err
}

_ = result.Text
```

## Extending with Adapters

Concrete implementations should be added in outbound adapters and wired through `wiring`. This keeps the contract stable while allowing different providers to implement the same `CodingAgentEnvironment` and `CodingAgent` interfaces.
