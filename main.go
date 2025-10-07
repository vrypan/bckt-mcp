package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// JSON-RPC 2.0 structures
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
	Description  string      `json:"description"`
	InputSchema  interface{} `json:"inputSchema"`
	OutputSchema interface{} `json:"outputSchema,omitempty"`
}

type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

type ToolCallParams struct {
	Name      string           `json:"name"`
	Arguments *json.RawMessage `json:"arguments,omitempty"`
}

type ToolCallResult struct {
	Content []Content `json:"content"`
}

type Content struct {
	Type string      `json:"type"`
	Text string      `json:"text,omitempty"`
	JSON interface{} `json:"json,omitempty"`
}

// bckt.format tool structures
type FormatInput struct {
	Raw      string                 `json:"raw"`
	Meta     map[string]interface{} `json:"meta"`
	Config   string                 `json:"config,omitempty"`
	Strategy string                 `json:"strategy,omitempty"`
}

type FormatOutput struct {
	Path     string   `json:"path"`
	Markdown string   `json:"markdown"`
	Warnings []string `json:"warnings"`
}

type Config struct {
	Timezone    string `toml:"timezone"`
	PathPattern string `toml:"path_pattern"`
	FrontMatter struct {
		Required []string               `toml:"required"`
		Defaults map[string]interface{} `toml:"defaults"`
	} `toml:"front_matter"`
	MarkdownRule struct {
		WrapAt int `toml:"wrap_at"`
	} `toml:"markdown_rules"`
}

func main() {
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
				"version": "1.0.0",
			},
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		},
	}
}

func handleToolsList(req *Request) *Response {
	tool := ToolDefinition{
		Name:        "bckt_format",
		Description: "Format raw text and metadata into bckt-compatible Markdown with YAML front matter and compute file path",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"raw": map[string]interface{}{
					"type":        "string",
					"description": "Raw markdown content",
				},
				"meta": map[string]interface{}{
					"type":        "object",
					"description": "Metadata for front matter",
					"properties": map[string]interface{}{
						"title":       map[string]interface{}{"type": "string"},
						"slug":        map[string]interface{}{"type": "string"},
						"date":        map[string]interface{}{"type": "string"},
						"tags":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"description": map[string]interface{}{"type": "string"},
						"excerpt":     map[string]interface{}{"type": "string"},
						"draft":       map[string]interface{}{"type": "boolean"},
						"lang":        map[string]interface{}{"type": "string"},
					},
					"required": []string{"title"},
				},
				"config": map[string]interface{}{
					"type":        "string",
					"description": "Optional TOML configuration",
				},
				"strategy": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"strict", "lenient"},
					"description": "Validation strategy",
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
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolsListResult{Tools: []ToolDefinition{tool}},
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

	if params.Name != "bckt_format" {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32601, Message: "Unknown tool"},
		}
	}

	var input FormatInput
	if params.Arguments != nil {
		if err := json.Unmarshal(*params.Arguments, &input); err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &Error{Code: -32602, Message: "Invalid arguments"},
			}
		}
	}

	output, err := formatContent(input)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: 1, Message: err.Error()},
		}
	}

	// Format result as text
	resultText := fmt.Sprintf("Path: %s\n\n%s", output.Path, output.Markdown)
	if len(output.Warnings) > 0 {
		resultText = fmt.Sprintf("Warnings:\n- %s\n\n%s", strings.Join(output.Warnings, "\n- "), resultText)
	}

	content := []Content{
		{Type: "text", Text: resultText},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolCallResult{Content: content},
	}
}

func formatContent(input FormatInput) (*FormatOutput, error) {
	// Load configuration
	cfg := getDefaultConfig()
	if input.Config != "" {
		if err := toml.Unmarshal([]byte(input.Config), &cfg); err == nil {
			// Config loaded successfully
		}
	}

	// Build front matter
	frontMatter := make(map[string]interface{})

	// Apply defaults
	for k, v := range cfg.FrontMatter.Defaults {
		frontMatter[k] = v
	}

	// Apply user metadata
	for k, v := range input.Meta {
		frontMatter[k] = v
	}

	// Validate title
	title, ok := frontMatter["title"].(string)
	if !ok || strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("title is required")
	}

	// Auto-generate slug if missing
	if _, ok := frontMatter["slug"]; !ok {
		frontMatter["slug"] = slugify(title)
	}

	// Auto-generate date if missing
	if _, ok := frontMatter["date"]; !ok {
		frontMatter["date"] = time.Now().Format(time.RFC3339)
	}

	// Ensure required fields have defaults
	if _, ok := frontMatter["tags"]; !ok {
		frontMatter["tags"] = []string{}
	}
	if _, ok := frontMatter["description"]; !ok {
		frontMatter["description"] = ""
	}
	if _, ok := frontMatter["excerpt"]; !ok {
		frontMatter["excerpt"] = ""
	}

	// Validate front matter
	warnings, err := validateFrontMatter(frontMatter, cfg, input.Strategy != "lenient")
	if err != nil {
		return nil, err
	}

	// Format body text
	body := wrapText(input.Raw, cfg.MarkdownRule.WrapAt)

	// Generate YAML front matter
	yamlData, err := yaml.Marshal(frontMatter)
	if err != nil {
		return nil, fmt.Errorf("failed to generate YAML: %v", err)
	}

	// Assemble final markdown
	markdown := fmt.Sprintf("---\n%s---\n\n%s\n", string(yamlData), strings.TrimRight(body, "\n"))

	// Compute path
	dateStr := frontMatter["date"].(string)
	slug := frontMatter["slug"].(string)
	path := computePath(cfg.PathPattern, dateStr, slug)

	return &FormatOutput{
		Path:     path,
		Markdown: markdown,
		Warnings: warnings,
	}, nil
}

func getDefaultConfig() Config {
	var cfg Config
	cfg.Timezone = "Europe/Athens"
	cfg.PathPattern = "content/{yyyy}/{MM}/{slug}/index.md"
	cfg.FrontMatter.Required = []string{"title", "slug", "date", "tags", "description", "draft", "lang", "excerpt"}
	cfg.FrontMatter.Defaults = map[string]interface{}{
		"lang":  "en",
		"draft": false,
	}
	cfg.MarkdownRule.WrapAt = 100
	return cfg
}

func validateFrontMatter(fm map[string]interface{}, cfg Config, strict bool) ([]string, error) {
	required := make(map[string]bool)
	for _, field := range cfg.FrontMatter.Required {
		required[field] = true
		if _, ok := fm[field]; !ok {
			return nil, fmt.Errorf("missing required field: %s", field)
		}
	}

	var warnings []string
	if strict {
		for key := range fm {
			if !required[key] {
				return nil, fmt.Errorf("unknown field in strict mode: %s", key)
			}
		}
	} else {
		for key := range fm {
			if !required[key] {
				warnings = append(warnings, fmt.Sprintf("unknown field: %s", key))
			}
		}
	}

	return warnings, nil
}

func slugify(s string) string {
	s = strings.ToLower(s)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func wrapText(text string, width int) string {
	if width < 20 {
		return text
	}

	var result []string
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result = append(result, line)
			continue
		}

		words := strings.Fields(line)
		var current strings.Builder

		for _, word := range words {
			if current.Len() == 0 {
				current.WriteString(word)
			} else if current.Len()+1+len(word) <= width {
				current.WriteString(" ")
				current.WriteString(word)
			} else {
				result = append(result, current.String())
				current.Reset()
				current.WriteString(word)
			}
		}

		if current.Len() > 0 {
			result = append(result, current.String())
		}
	}

	return strings.Join(result, "\n")
}

func computePath(pattern, date, slug string) string {
	yyyy := date[:4]
	mm := date[5:7]

	path := strings.ReplaceAll(pattern, "{yyyy}", yyyy)
	path = strings.ReplaceAll(path, "{MM}", mm)
	path = strings.ReplaceAll(path, "{slug}", slug)

	return path
}
