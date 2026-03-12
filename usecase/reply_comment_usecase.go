package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"bentos-backend/domain"
	"bentos-backend/shared/logger/stdlogger"
	uccontracts "bentos-backend/usecase/contracts"
)

// replyCommentUseCase is the concrete ReplyCommentUseCase implementation.
type replyCommentUseCase struct {
	sanitizer    SafetySanitizer
	answerer     ReplyCommentAnswerer
	publisher    ReplyCommentPublisher
	envFactory   uccontracts.CodeEnvironmentFactory
	recipeLoader CustomRecipeLoader
	logger       Logger
}

// NewReplyCommentUseCase constructs a reply comment usecase.
func NewReplyCommentUseCase(
	sanitizer SafetySanitizer,
	answerer ReplyCommentAnswerer,
	publisher ReplyCommentPublisher,
	envFactory uccontracts.CodeEnvironmentFactory,
	recipeLoader CustomRecipeLoader,
	logger Logger,
) (ReplyCommentUseCase, error) {
	if sanitizer == nil || answerer == nil || publisher == nil || envFactory == nil || recipeLoader == nil {
		return nil, errors.New("reply comment usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	return &replyCommentUseCase{
		sanitizer:    sanitizer,
		answerer:     answerer,
		publisher:    publisher,
		envFactory:   envFactory,
		recipeLoader: recipeLoader,
		logger:       logger,
	}, nil
}

// Execute runs the reply comment flow.
func (u *replyCommentUseCase) Execute(ctx context.Context, request ReplyCommentRequest) (ReplyCommentResult, error) {
	startedAt := time.Now()
	target := domain.ChangeRequestTarget{
		Repository:          request.Repository,
		ChangeRequestNumber: request.ChangeRequestNumber,
	}
	logExecution(u.logger, "replycomment", target, "start", startedAt, "")

	if strings.TrimSpace(request.Question) == "" {
		return ReplyCommentResult{}, fmt.Errorf("question is required")
	}

	sanitizeStartedAt := time.Now()
	sanitized, err := u.sanitizer.Sanitize(ctx, request.Question)
	if err != nil {
		logStage(u.logger, "replycomment", "sanitize_question", target, "failure", sanitizeStartedAt, "%v", err)
		return ReplyCommentResult{}, err
	}
	logStage(u.logger, "replycomment", "sanitize_question", target, "success", sanitizeStartedAt, "")

	answerText := ""
	recipe := domain.CustomRecipe{}
	if sanitized.Status == domain.PromptSafetyStatusOK {
		initializeEnvironmentStartedAt := time.Now()
		environment, envErr := u.envFactory.New(ctx, domain.CodeEnvironmentInitOptions{
			RepoURL: request.RepoURL,
		})
		if envErr != nil {
			logStage(u.logger, "replycomment", "initialize_code_environment", target, "failure", initializeEnvironmentStartedAt, "%v", envErr)
			return ReplyCommentResult{}, envErr
		}
		defer func() {
			if cleanupErr := environment.Cleanup(ctx); cleanupErr != nil {
				u.logger.Warnf("Failed to cleanup code environment: %v", cleanupErr)
			}
		}()
		logStage(u.logger, "replycomment", "initialize_code_environment", target, "success", initializeEnvironmentStartedAt, "")

		headRef := strings.TrimSpace(request.Head)
		if headRef == "" {
			headRef = "HEAD"
		}
		loadRecipeStartedAt := time.Now()
		loadedRecipe, err := u.recipeLoader.Load(ctx, environment, headRef)
		if err != nil {
			logStage(u.logger, "replycomment", "load_recipe", target, "failure", loadRecipeStartedAt, "%v", err)
			return ReplyCommentResult{}, err
		}
		recipe = loadedRecipe

		answerStartedAt := time.Now()
		answerText, err = u.answerer.Answer(ctx, ReplyCommentAnswerPayload{
			Input: domain.ChangeRequestInput{
				Target:      target,
				RepoURL:     request.RepoURL,
				Base:        request.Base,
				Head:        request.Head,
				Title:       request.Title,
				Description: request.Description,
				Language:    "English",
				Metadata:    request.Metadata,
			},
			Thread:        request.Thread,
			Question:      sanitized.SanitizedPrompt,
			Environment:   environment,
			ExtraGuidance: strings.TrimSpace(recipe.AutoreplyGuidance),
		})
		if err != nil {
			logStage(u.logger, "replycomment", "answer_question", target, "failure", answerStartedAt, "%v", err)
			return ReplyCommentResult{}, err
		}
		logStage(u.logger, "replycomment", "answer_question", target, "success", answerStartedAt, "")
	} else {
		answerText = strings.TrimSpace(sanitized.RefusalMessage)
		if answerText == "" {
			answerText = "Thanks for the question. I can't safely help with that request."
		}
	}

	replyBody := formatReplyBody(request.Question, answerText)
	publishStartedAt := time.Now()
	if err := u.publisher.Publish(ctx, ReplyCommentPublishResult{
		Target:         target,
		CommentID:      request.CommentID,
		Kind:           request.CommentKind,
		Body:           replyBody,
		ShouldPost:     request.Publish,
		RecipeWarnings: recipe.MissingPaths,
	}); err != nil {
		logStage(u.logger, "replycomment", "publish_reply", target, "failure", publishStartedAt, "%v", err)
		return ReplyCommentResult{}, err
	}
	logStage(u.logger, "replycomment", "publish_reply", target, "success", publishStartedAt, "")

	logExecution(
		u.logger,
		"replycomment",
		target,
		"complete",
		startedAt,
		"Replycomment execution took %d ms.",
		time.Since(startedAt).Milliseconds(),
	)

	return ReplyCommentResult{Answer: answerText}, nil
}

func formatReplyBody(question string, answer string) string {
	question = strings.TrimSpace(question)
	answer = strings.TrimSpace(answer)
	quoted := quoteText(question)
	if quoted == "" {
		return answer
	}
	if answer == "" {
		return quoted
	}
	return fmt.Sprintf("%s\n\n%s", quoted, answer)
}

func quoteText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	for i, line := range lines {
		lines[i] = "> " + strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
