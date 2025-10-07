package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func HandleBcktConfig(id interface{}, params ToolCallParams, globalConfig *Config) *Response {
	var args struct {
		RootPath    string `json:"root_path,omitempty"`
		Timezone    string `json:"timezone,omitempty"`
		PathPattern string `json:"path_pattern,omitempty"`
		WrapAt      int    `json:"wrap_at,omitempty"`
	}

	if params.Arguments != nil {
		if err := json.Unmarshal(*params.Arguments, &args); err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error:   &Error{Code: -32602, Message: "Invalid arguments"},
			}
		}
	}

	// Ensure global config is loaded
	if globalConfig == nil {
		cfg := GetDefaultConfig()
		globalConfig = &cfg
	}

	// Check if this is a view or update operation
	isUpdate := args.RootPath != "" || args.Timezone != "" || args.PathPattern != "" || args.WrapAt != 0

	if isUpdate {
		// Update config
		if args.RootPath != "" {
			globalConfig.RootPath = expandPath(args.RootPath)
		}
		if args.Timezone != "" {
			globalConfig.Timezone = args.Timezone
		}
		if args.PathPattern != "" {
			globalConfig.PathPattern = args.PathPattern
		}
		if args.WrapAt != 0 {
			globalConfig.MarkdownRule.WrapAt = args.WrapAt
		}

		// Save to file
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".config", "bckt-mcp", "config.toml")
		if err := SaveGlobalConfig(configPath, globalConfig); err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error:   &Error{Code: 1, Message: fmt.Sprintf("Failed to save config: %v", err)},
			}
		}

		resultText := "✓ Configuration updated:\n"
		if args.RootPath != "" {
			resultText += fmt.Sprintf("  root_path: %s\n", args.RootPath)
		}
		if args.Timezone != "" {
			resultText += fmt.Sprintf("  timezone: %s\n", args.Timezone)
		}
		if args.PathPattern != "" {
			resultText += fmt.Sprintf("  path_pattern: %s\n", args.PathPattern)
		}
		if args.WrapAt != 0 {
			resultText += fmt.Sprintf("  wrap_at: %d\n", args.WrapAt)
		}

		content := []Content{
			{Type: "text", Text: resultText},
		}

		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Result:  ToolCallResult{Content: content},
		}
	}

	// View current config
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".config", "bckt-mcp", "config.toml")

	configText := fmt.Sprintf(`Current Configuration:
Config file: %s

root_path: %s
timezone: %s
path_pattern: %s
wrap_at: %d

Front Matter:
  required: %v
  defaults: %v
`,
		configPath,
		globalConfig.RootPath,
		globalConfig.Timezone,
		globalConfig.PathPattern,
		globalConfig.MarkdownRule.WrapAt,
		globalConfig.FrontMatter.Required,
		globalConfig.FrontMatter.Defaults,
	)

	content := []Content{
		{Type: "text", Text: configText},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  ToolCallResult{Content: content},
	}
}
