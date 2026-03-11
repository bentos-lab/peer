package wiring

import (
	"os"

	cliinbound "bentos-backend/adapter/inbound/cli"
	autogencodingagent "bentos-backend/adapter/outbound/autogen/codingagent"
	codeenvhost "bentos-backend/adapter/outbound/codeenv/host"
	llmtracing "bentos-backend/adapter/outbound/llm/tracing"
	overviewcodingagent "bentos-backend/adapter/outbound/overview/codingagent"
	clipublisher "bentos-backend/adapter/outbound/publisher/cli"
	githubpublisher "bentos-backend/adapter/outbound/publisher/github"
	routerpublisher "bentos-backend/adapter/outbound/publisher/router"
	replycommentcodingagent "bentos-backend/adapter/outbound/replycomment/codingagent"
	replycommentsanitizer "bentos-backend/adapter/outbound/replycomment/sanitizer"
	reviewercodingagent "bentos-backend/adapter/outbound/reviewer/codingagent"
	githubvcs "bentos-backend/adapter/outbound/vcs/github"
	"bentos-backend/config"
	"bentos-backend/usecase"
	"bentos-backend/usecase/contracts"
	"bentos-backend/usecase/rulepack"
)

// BuildCLIReviewCommand wires dependencies for a single CLI review mode.
func BuildCLIReviewCommand(cfg config.Config, opts CLILLMOptions, logLevelOverride string) (*cliinbound.Command, error) {
	logger, err := buildLogger(cfg, logLevelOverride)
	if err != nil {
		return nil, err
	}

	llmSelection, err := ResolveLLMSelection(cfg, opts)
	if err != nil {
		return nil, err
	}
	var formatter contracts.LLMGenerator
	if llmSelection.UseOpenAI {
		formatter = buildOpenAIGenerator(llmSelection)
	} else {
		formatter, err = buildCodingAgentGenerator(cfg, logger)
		if err != nil {
			return nil, err
		}
	}
	tracedLLMClient := llmtracing.NewGenerator(formatter, logger)
	codeEnvironmentFactory := codeenvhost.NewFactory(codeenvhost.FactoryConfig{
		Logger: logger,
	})
	codingAgentConfig := resolveServerCodingAgentConfig(cfg)
	codingReviewer, err := reviewercodingagent.NewReviewer(tracedLLMClient, reviewercodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}
	codingOverview, err := overviewcodingagent.NewOverviewGenerator(tracedLLMClient, overviewcodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}
	ruleProvider := rulepack.NewCoreRulePackProvider()
	githubClient := githubvcs.NewCLIClient()

	reviewPublisher := routerpublisher.NewReviewPublisher(
		clipublisher.NewPublisher(os.Stdout),
		githubpublisher.NewPublisher(githubClient, logger),
	)
	overviewPublisher := routerpublisher.NewOverviewPublisher(
		clipublisher.NewOverviewPublisher(os.Stdout),
		githubpublisher.NewOverviewPublisher(githubClient, logger),
	)

	reviewUseCase, err := usecase.NewReviewUseCase(
		ruleProvider,
		codingReviewer,
		reviewPublisher,
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}
	overviewUseCase, err := usecase.NewOverviewUseCase(
		codingOverview,
		overviewPublisher,
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}
	changeRequestUseCase, err := usecase.NewChangeRequestUseCase(
		reviewUseCase,
		overviewUseCase,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return cliinbound.NewCommand(changeRequestUseCase, githubClient, logger), nil
}

// BuildCLIAutogenCommand wires dependencies for a single CLI autogen mode.
func BuildCLIAutogenCommand(cfg config.Config, _ CLILLMOptions, logLevelOverride string) (*cliinbound.AutogenCommand, error) {
	logger, err := buildLogger(cfg, logLevelOverride)
	if err != nil {
		return nil, err
	}

	codeEnvironmentFactory := codeenvhost.NewFactory(codeenvhost.FactoryConfig{
		Logger: logger,
	})
	codingAgentConfig := resolveServerCodingAgentConfig(cfg)
	autogenGenerator, err := autogencodingagent.NewGenerator(autogencodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}

	githubClient := githubvcs.NewCLIClient()
	autogenPublisher := routerpublisher.NewAutogenPublisher(
		clipublisher.NewAutogenPublisher(os.Stdout),
		githubpublisher.NewAutogenPublisher(githubClient, logger),
	)

	autogenUseCase, err := usecase.NewAutogenUseCase(
		autogenGenerator,
		autogenPublisher,
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return cliinbound.NewAutogenCommand(autogenUseCase, githubClient, logger), nil
}

// BuildCLIReplyCommentCommand wires dependencies for CLI replycomment mode.
func BuildCLIReplyCommentCommand(cfg config.Config, opts CLILLMOptions, logLevelOverride string) (*cliinbound.ReplyCommentCommand, error) {
	logger, err := buildLogger(cfg, logLevelOverride)
	if err != nil {
		return nil, err
	}

	llmSelection, err := ResolveLLMSelection(cfg, opts)
	if err != nil {
		return nil, err
	}
	var formatter contracts.LLMGenerator
	if llmSelection.UseOpenAI {
		formatter = buildOpenAIGenerator(llmSelection)
	} else {
		formatter, err = buildCodingAgentGenerator(cfg, logger)
		if err != nil {
			return nil, err
		}
	}
	tracedLLMClient := llmtracing.NewGenerator(formatter, logger)

	codeEnvironmentFactory := codeenvhost.NewFactory(codeenvhost.FactoryConfig{
		Logger: logger,
	})
	codingAgentConfig := resolveServerCodingAgentConfig(cfg)
	answerer, err := replycommentcodingagent.NewAnswerer(replycommentcodingagent.Config{
		Agent:    codingAgentConfig.Agent,
		Provider: codingAgentConfig.Provider,
		Model:    codingAgentConfig.Model,
	}, logger)
	if err != nil {
		return nil, err
	}
	sanitizer, err := replycommentsanitizer.NewSanitizer(tracedLLMClient)
	if err != nil {
		return nil, err
	}

	githubClient := githubvcs.NewCLIClient()
	publisher := routerpublisher.NewReplyCommentPublisher(
		clipublisher.NewReplyCommentPublisher(os.Stdout),
		githubpublisher.NewReplyCommentPublisher(githubClient, logger),
	)
	replyCommentUseCase, err := usecase.NewReplyCommentUseCase(
		sanitizer,
		answerer,
		publisher,
		codeEnvironmentFactory,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return cliinbound.NewReplyCommentCommand(replyCommentUseCase, githubClient, cfg.Server.GitHub.ReplyCommentTriggerName), nil
}
