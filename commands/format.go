package commands

import (
	"encoding/json"
	"fmt"
	"strings"
)

func HandleBcktFormat(id interface{}, params ToolCallParams, previewMode bool, globalConfig *Config) *Response {
	var input FormatInput
	if params.Arguments != nil {
		if err := json.Unmarshal(*params.Arguments, &input); err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      id,
				Error:   &Error{Code: -32602, Message: "Invalid arguments"},
			}
		}
	}

	output, err := FormatContent(input, globalConfig)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &Error{Code: 1, Message: err.Error()},
		}
	}

	// Format result with multiple content blocks for better display
	content := []Content{}

	if len(output.Warnings) > 0 {
		warningText := "Warnings:\n- " + strings.Join(output.Warnings, "\n- ")
		content = append(content, Content{Type: "text", Text: warningText})
	}

	if previewMode {
		content = append(content, Content{Type: "text", Text: "PREVIEW MODE - Not saved"})
	}

	content = append(content, Content{Type: "text", Text: fmt.Sprintf("Path: %s", output.Path)})
	content = append(content, Content{Type: "text", Text: output.Markdown})

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  ToolCallResult{Content: content},
	}
}
