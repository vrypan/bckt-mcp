package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func HandleBcktSetup(id interface{}, params ToolCallParams, globalConfig **Config) *Response {
	var args struct {
		RootPath    string `json:"root_path"`
		Timezone    string `json:"timezone"`
		PathPattern string `json:"path_pattern,omitempty"`
		WrapAt      int    `json:"wrap_at,omitempty"`
		Confirm     bool   `json:"confirm,omitempty"`
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

	// Load defaults
	defaults := GetDefaultConfig()

	// Use provided values or defaults
	rootPath := expandPath(args.RootPath)
	timezone := args.Timezone
	pathPattern := args.PathPattern
	if pathPattern == "" {
		pathPattern = defaults.PathPattern
	}
	wrapAt := args.WrapAt
	if wrapAt == 0 {
		wrapAt = defaults.MarkdownRule.WrapAt
	}

	// If not confirmed, show preview
	if !args.Confirm {
		previewText := fmt.Sprintf(`Configuration Preview:

root_path: %s
  → Where your blog posts will be saved

timezone: %s
  → Used for generating post dates

path_pattern: %s
  → Template for file paths
  → Placeholders: {yyyy} {MM} {DD} {slug}
  → Example: posts/2025/2025-10-07-my-post/my-post.md

wrap_at: %d
  → Maximum line width for text wrapping

To save this configuration, call bckt_setup again with confirm: true
`, rootPath, timezone, pathPattern, wrapAt)

		content := []Content{
			{Type: "text", Text: previewText},
		}

		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Result:  ToolCallResult{Content: content},
		}
	}

	// Confirmed - save configuration
	if *globalConfig == nil {
		cfg := GetDefaultConfig()
		*globalConfig = &cfg
	}

	(*globalConfig).RootPath = rootPath
	(*globalConfig).Timezone = timezone
	(*globalConfig).PathPattern = pathPattern
	(*globalConfig).MarkdownRule.WrapAt = wrapAt

	// Save to file
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".config", "bckt-mcp", "config.toml")
	if err := SaveGlobalConfig(configPath, *globalConfig); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &Error{Code: 1, Message: fmt.Sprintf("Failed to save config: %v", err)},
		}
	}

	resultText := fmt.Sprintf(`✓ Configuration saved to: %s

root_path: %s
timezone: %s
path_pattern: %s
wrap_at: %d

You're all set! You can now use bckt, bckt_preview, and bckt_save.
`, configPath, rootPath, timezone, pathPattern, wrapAt)

	content := []Content{
		{Type: "text", Text: resultText},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  ToolCallResult{Content: content},
	}
}
