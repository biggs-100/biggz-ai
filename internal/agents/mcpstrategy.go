package agents

import "github.com/biggz-ai/biggz/model"

// MCPStrategy is a type alias for model.MCPStrategy.
type MCPStrategy = model.MCPStrategy

// Known MCP strategy constants — re-exported from model for convenience
// and backward compatibility.
const (
	StrategySeparateMCPFiles  MCPStrategy = model.StrategySeparateMCPFiles
	StrategyMergeIntoSettings MCPStrategy = model.StrategyMergeIntoSettings
	StrategyMCPConfigFile     MCPStrategy = model.StrategyMCPConfigFile
	StrategyTOMLFile          MCPStrategy = model.StrategyTOMLFile
	StrategyMergeIntoYAML     MCPStrategy = model.StrategyMergeIntoYAML
)
