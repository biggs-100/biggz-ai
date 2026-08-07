package agents

import "github.com/biggs-100/biggz-ai/model"

// SystemPromptStrategy is a type alias for model.SystemPromptStrategy.
type SystemPromptStrategy = model.SystemPromptStrategy

// Known system prompt strategy constants — re-exported from model for
// convenience and backward compatibility.
const (
	StrategyMarkdownSections SystemPromptStrategy = model.StrategyMarkdownSections
	StrategyFileReplace      SystemPromptStrategy = model.StrategyFileReplace
	StrategyAppendToFile     SystemPromptStrategy = model.StrategyAppendToFile
	StrategyInstructionsFile SystemPromptStrategy = model.StrategyInstructionsFile
	StrategyJinjaModules     SystemPromptStrategy = model.StrategyJinjaModules
	StrategySteeringFile     SystemPromptStrategy = model.StrategySteeringFile
)
