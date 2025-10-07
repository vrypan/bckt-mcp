package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"bckt-mcp/commands"
)

var version = "dev"

// JSON-RPC 2.0 structures (main protocol only)
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP protocol structures
type InitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	ServerInfo      map[string]string      `json:"serverInfo"`
	Capabilities    map[string]interface{} `json:"capabilities"`
}

type ToolDefinition struct {
	Name         string      `json:"name"`
	Abstract     string      `json:"abstract"`
	InputSchema  interface{} `json:"inputSchema"`
	OutputSchema interface{} `json:"outputSchema,omitempty"`
}

type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

type PromptDefinition struct {
	Name      string           `json:"name"`
	Abstract  string           `json:"abstract,omitempty"`
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

type PromptArgument struct {
	Name     string `json:"name"`
	Abstract string `json:"abstract,omitempty"`
	Required bool   `json:"required,omitempty"`
}

type PromptsListResult struct {
	Prompts []PromptDefinition `json:"prompts"`
}

type PromptGetParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type PromptMessage struct {
	Role    string          `json:"role"`
	Content commands.Content `json:"content"`
}

type PromptGetResult struct {
	Messages []PromptMessage `json:"messages"`
}

type ToolCallParams struct {
	Name      string           `json:"name"`
	Arguments *json.RawMessage `json:"arguments,omitempty"`
}

var globalConfig *commands.Config

func main() {
	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("bckt-mcp version %s\n", version)
		os.Exit(0)
	}

	// Load global config on startup
	globalConfig = commands.LoadGlobalConfig()

	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for {
		// Read incoming message
		request, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		// Handle request
		response := handleRequest(request)

		// Write response (skip if nil for notifications)
		if response != nil {
			if err := writeMessage(writer, response); err != nil {
				return
			}
		}
	}
}

func readMessage(r *bufio.Reader) (*Request, error) {
	// Claude's MCP client uses newline-delimited JSON, not Content-Length headers
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return nil, io.EOF
	}

	// Parse request
	var req Request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return nil, err
	}

	return &req, nil
}

func writeMessage(w *bufio.Writer, response *Response) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	// Write newline-delimited JSON
	if _, err := w.Write(data); err != nil {
		return err
	}

	if _, err := w.WriteString("\n"); err != nil {
		return err
	}

	return w.Flush()
}

func handleRequest(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return handleInitialize(req)
	case "initialized", "notifications/initialized":
		return nil // Notification, no response needed
	case "tools/list":
		return handleToolsList(req)
	case "tools/call":
		return handleToolCall(req)
	case "prompts/list":
		return handlePromptsList(req)
	case "prompts/get":
		return handlePromptsGet(req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32601, Message: "Method not found"},
		}
	}
}

func handleInitialize(req *Request) *Response {
	var params InitializeParams
	if len(req.Params) > 0 {
		json.Unmarshal(req.Params, &params)
	}

	// Negotiate protocol version
	supportedVersions := []string{"2025-06-18", "2024-11-05"}
	selectedVersion := supportedVersions[0]
	if params.ProtocolVersion != "" {
		for _, v := range supportedVersions {
			if v == params.ProtocolVersion {
				selectedVersion = v
				break
			}
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: selectedVersion,
			ServerInfo: map[string]string{
				"name":    "bckt-mcp",
				"version": version,
			},
			Capabilities: map[string]interface{}{
				"tools":   map[string]interface{}{},
				"prompts": map[string]interface{}{},
			},
		},
	}
}

func handleToolsList(req *Request) *Response {
	tools := []ToolDefinition{
		{
			Name:     "bckt",
			Abstract: "Format raw text and metadata into bckt-compatible Markdown with YAML front matter and compute file path. IMPORTANT: Before calling this tool, you MUST ask the user to provide: title, tags, abstract, and optionally slug, language, and excerpt. Never auto-generate metadata without explicit user confirmation.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"raw": map[string]interface{}{
						"type":     "string",
						"abstract": "Raw markdown content",
					},
					"meta": map[string]interface{}{
						"type":     "object",
						"abstract": "Metadata for front matter",
						"properties": map[string]interface{}{
							"title":    map[string]interface{}{"type": "string"},
							"slug":     map[string]interface{}{"type": "string"},
							"date":     map[string]interface{}{"type": "string"},
							"tags":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
							"abstract": map[string]interface{}{"type": "string"},
							"lang":     map[string]interface{}{"type": "string"},
						},
						"required": []string{"title"},
					},
					"config": map[string]interface{}{
						"type":     "string",
						"abstract": "Optional TOML configuration",
					},
					"strategy": map[string]interface{}{
						"type":     "string",
						"enum":     []string{"strict", "lenient"},
						"abstract": "Validation strategy",
					},
				},
				"required": []string{"raw", "meta"},
			},
			OutputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":     map[string]interface{}{"type": "string"},
					"markdown": map[string]interface{}{"type": "string"},
					"warnings": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				},
			},
		},
		{
			Name:     "bckt_preview",
			Abstract: "Preview the formatted output without saving. Shows the generated YAML front matter, markdown, and computed file path.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"raw": map[string]interface{}{
						"type":     "string",
						"abstract": "Raw markdown content",
					},
					"meta": map[string]interface{}{
						"type":     "object",
						"abstract": "Metadata for front matter",
						"properties": map[string]interface{}{
							"title":    map[string]interface{}{"type": "string"},
							"slug":     map[string]interface{}{"type": "string"},
							"date":     map[string]interface{}{"type": "string"},
							"tags":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
							"abstract": map[string]interface{}{"type": "string"},
							"lang":     map[string]interface{}{"type": "string"},
						},
						"required": []string{"title"},
					},
					"config":   map[string]interface{}{"type": "string", "abstract": "Optional TOML configuration"},
					"strategy": map[string]interface{}{"type": "string", "enum": []string{"strict", "lenient"}, "abstract": "Validation strategy"},
				},
				"required": []string{"raw", "meta"},
			},
		},
		{
			Name:     "bckt_save",
			Abstract: "Save the formatted markdown to the computed file path. Creates directories if needed. On first use, asks for root_path (e.g., /Users/yourname/blog) and saves it to config.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"markdown": map[string]interface{}{
						"type":     "string",
						"abstract": "The complete formatted markdown with front matter",
					},
					"path": map[string]interface{}{
						"type":     "string",
						"abstract": "The file path where to save (from bckt or bckt_preview output)",
					},
					"root_path": map[string]interface{}{
						"type":     "string",
						"abstract": "Root directory for blog posts (required on first save, then saved to config)",
					},
				},
				"required": []string{"markdown", "path"},
			},
		},
		{
			Name:     "bckt_config",
			Abstract: "View or update the bckt-mcp configuration. If no parameters provided, returns current config. If parameters provided, updates config and saves it.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"root_path": map[string]interface{}{
						"type":     "string",
						"abstract": "Root directory for blog posts",
					},
					"timezone": map[string]interface{}{
						"type":     "string",
						"abstract": "Timezone for dates (e.g., 'America/New_York', 'Europe/London', 'UTC')",
					},
					"path_pattern": map[string]interface{}{
						"type":     "string",
						"abstract": "Path pattern with placeholders: {yyyy}, {MM}, {DD}, {slug}",
					},
					"wrap_at": map[string]interface{}{
						"type":     "integer",
						"abstract": "Line width for text wrapping",
					},
				},
			},
		},
		{
			Name:     "bckt_setup",
			Abstract: "Interactive setup wizard for first-time configuration. Shows current values and suggestions, then saves all settings at once when confirmed.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"root_path": map[string]interface{}{
						"type":     "string",
						"abstract": "Root directory for blog posts",
					},
					"timezone": map[string]interface{}{
						"type":     "string",
						"abstract": "Timezone for dates",
					},
					"path_pattern": map[string]interface{}{
						"type":     "string",
						"abstract": "Path pattern (optional, uses default if not provided)",
					},
					"wrap_at": map[string]interface{}{
						"type":     "integer",
						"abstract": "Line width for text wrapping (optional, uses default if not provided)",
					},
					"confirm": map[string]interface{}{
						"type":     "boolean",
						"abstract": "Set to true to save the configuration",
					},
				},
				"required": []string{"root_path", "timezone"},
			},
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolsListResult{Tools: tools},
	}
}

func handleToolCall(req *Request) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		}
	}

	cmdParams := commands.ToolCallParams{
		Name:      params.Name,
		Arguments: params.Arguments,
	}

	switch params.Name {
	case "bckt", "bckt_preview":
		cmdResp := commands.HandleBcktFormat(req.ID, cmdParams, params.Name == "bckt_preview", globalConfig)
		return convertResponse(cmdResp)
	case "bckt_save":
		cmdResp := commands.HandleBcktSave(req.ID, cmdParams, globalConfig)
		return convertResponse(cmdResp)
	case "bckt_config":
		cmdResp := commands.HandleBcktConfig(req.ID, cmdParams, globalConfig)
		return convertResponse(cmdResp)
	case "bckt_setup":
		cmdResp := commands.HandleBcktSetup(req.ID, cmdParams, &globalConfig)
		return convertResponse(cmdResp)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32601, Message: "Unknown tool"},
		}
	}
}

func handlePromptsList(req *Request) *Response {
	prompts := []PromptDefinition{
		{
			Name:     "format_blog_post",
			Abstract: "Interactive workflow to format a blog post with user input for metadata",
			Arguments: []PromptArgument{
				{
					Name:     "content",
					Abstract: "The raw blog post content",
					Required: true,
				},
			},
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  PromptsListResult{Prompts: prompts},
	}
}

func convertResponse(cmdResp *commands.Response) *Response {
	if cmdResp == nil {
		return nil
	}

	var err *Error
	if cmdResp.Error != nil {
		err = &Error{
			Code:    cmdResp.Error.Code,
			Message: cmdResp.Error.Message,
		}
	}

	return &Response{
		JSONRPC: cmdResp.JSONRPC,
		ID:      cmdResp.ID,
		Result:  cmdResp.Result,
		Error:   err,
	}
}

func handlePromptsGet(req *Request) *Response {
	var params PromptGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		}
	}

	if params.Name != "format_blog_post" {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: "Unknown prompt"},
		}
	}

	content := ""
	if params.Arguments != nil {
		if c, ok := params.Arguments["content"].(string); ok {
			content = c
		}
	}

	instructions := `You are helping the user format a blog post for their bckt static site.

IMPORTANT: Your FIRST action must be to call the bckt_config tool (with no parameters) to check current configuration.

If root_path is empty or not set in the config, you MUST guide the user through bckt_setup:
1. Call bckt_setup with only root_path and timezone to show a preview
2. The tool will display actual default values for path_pattern and wrap_at
3. If user approves, call bckt_setup again with confirm: true to save

DO NOT assume or make up default values. ALWAYS use bckt_setup to show the real defaults from the system.

Once configuration is confirmed, follow these steps to format the blog post:
1. Ask the user for the blog post title
2. Ask for tags (comma-separated list)
3. Ask for a brief abstract (SEO meta description)
4. Ask if they want to specify a custom slug (or auto-generate from title)
5. Ask for the language code (default: en)

For each step, provide a suggested value based on the content.

Once you have all the information, use the bckt or bckt_preview tool with:
- raw: the content provided below
- meta: object with title, tags (array), abstract, slug (optional), lang

Content to format:
` + content

	messages := []PromptMessage{
		{
			Role:    "user",
			Content: commands.Content{Type: "text", Text: instructions},
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  PromptGetResult{Messages: messages},
	}
}
