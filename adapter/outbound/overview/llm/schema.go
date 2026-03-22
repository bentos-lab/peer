package llm

import "github.com/bentos-lab/peer/domain"

func overviewResponseSchema() map[string]any {
	categoryEnumValues := []string{
		string(domain.OverviewCategoryLogicUpdates),
		string(domain.OverviewCategoryRefactoring),
		string(domain.OverviewCategorySecurityFixes),
		string(domain.OverviewCategoryTestChanges),
		string(domain.OverviewCategoryDocumentation),
		string(domain.OverviewCategoryInfrastructureConfig),
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"categories", "walkthroughs"},
		"properties": map[string]any{
			"categories": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"category", "summary"},
					"properties": map[string]any{
						"category": map[string]any{
							"type": "string",
							"enum": categoryEnumValues,
						},
						"summary": map[string]any{
							"type": "string",
						},
					},
				},
			},
			"walkthroughs": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"groupName", "files", "summary"},
					"properties": map[string]any{
						"groupName": map[string]any{
							"type": "string",
						},
						"files": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
						"summary": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
	}
}
