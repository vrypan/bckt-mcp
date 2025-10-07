package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func HandleBcktSave(id interface{}, params ToolCallParams, globalConfig *Config) *Response {
	var args struct {
		Markdown string `json:"markdown"`
		Path     string `json:"path"`
		RootPath string `json:"root_path,omitempty"`
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

	if args.Markdown == "" || args.Path == "" {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &Error{Code: -32602, Message: "markdown and path are required"},
		}
	}

	// Determine the final path
	var finalPath string
	pathIsRelative := !filepath.IsAbs(args.Path)

	// If path is relative, we need root_path
	if pathIsRelative {
		currentRootPath := ""
		if globalConfig != nil {
			currentRootPath = globalConfig.RootPath
		}

		// Check if root_path is configured
		if currentRootPath == "" {
			// Root path not configured
			if args.RootPath == "" {
				// Not provided in arguments either - suggest setup
				return &Response{
					JSONRPC: "2.0",
					ID:      id,
					Error:   &Error{Code: -32602, Message: "Configuration not set up. Please run bckt_setup first to configure root_path, timezone, and other settings interactively."},
				}
			}

			// Save the provided root_path to config
			if globalConfig == nil {
				cfg := GetDefaultConfig()
				globalConfig = &cfg
			}
			globalConfig.RootPath = args.RootPath
			homeDir, _ := os.UserHomeDir()
			configPath := filepath.Join(homeDir, ".config", "bckt-mcp", "config.toml")
			if err := SaveGlobalConfig(configPath, globalConfig); err != nil {
				return &Response{
					JSONRPC: "2.0",
					ID:      id,
					Error:   &Error{Code: 1, Message: fmt.Sprintf("Failed to save root_path to config: %v", err)},
				}
			}
			currentRootPath = args.RootPath
		}

		// Build absolute path
		finalPath = filepath.Join(currentRootPath, args.Path)
	} else {
		// Path is already absolute
		finalPath = args.Path
	}

	// Create directories if needed
	dir := filepath.Dir(finalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &Error{Code: 1, Message: fmt.Sprintf("Failed to create directories: %v", err)},
		}
	}

	// Write file
	if err := os.WriteFile(finalPath, []byte(args.Markdown), 0644); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &Error{Code: 1, Message: fmt.Sprintf("Failed to write file: %v", err)},
		}
	}

	content := []Content{
		{Type: "text", Text: fmt.Sprintf("✓ Saved to: %s", finalPath)},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  ToolCallResult{Content: content},
	}
}
