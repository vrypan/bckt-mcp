package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	Role    string  `json:"role"`
	Content Content `json:"content"`
}

type PromptGetResult struct {
	Messages []PromptMessage `json:"messages"`
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
	RootPath    string `toml:"root_path"`
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

var globalConfig *Config

func main() {
	// Load global config on startup
	globalConfig = loadGlobalConfig()

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
				"version": "1.0.0",
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
			Abstract: "Format raw text and metadata into bckt-compatible Markdown with YAML front matter and compute file path. IMPORTANT: Before calling this tool, you MUST ask the user to provide: title, tags, abstract, and optionally slug, draft status, language, and excerpt. Never auto-generate metadata without explicit user confirmation.",
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
							"excerpt":  map[string]interface{}{"type": "string"},
							"draft":    map[string]interface{}{"type": "boolean"},
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
							"excerpt":  map[string]interface{}{"type": "string"},
							"draft":    map[string]interface{}{"type": "boolean"},
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

	switch params.Name {
	case "bckt", "bckt_preview":
		return handleBcktFormat(req.ID, params, params.Name == "bckt_preview")
	case "bckt_save":
		return handleBcktSave(req.ID, params)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32601, Message: "Unknown tool"},
		}
	}
}

func handleBcktFormat(id interface{}, params ToolCallParams, previewMode bool) *Response {
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

	output, err := formatContent(input)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &Error{Code: 1, Message: err.Error()},
		}
	}

	// Format result as text
	var resultText string
	if previewMode {
		resultText = fmt.Sprintf("PREVIEW MODE - Not saved\n\nPath: %s\n\n%s", output.Path, output.Markdown)
	} else {
		resultText = fmt.Sprintf("Path: %s\n\n%s", output.Path, output.Markdown)
	}

	if len(output.Warnings) > 0 {
		resultText = fmt.Sprintf("Warnings:\n- %s\n\n%s", strings.Join(output.Warnings, "\n- "), resultText)
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

func handleBcktSave(id interface{}, params ToolCallParams) *Response {
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
				// Not provided in arguments either
				return &Response{
					JSONRPC: "2.0",
					ID:      id,
					Error:   &Error{Code: -32602, Message: "root_path is not configured. Please provide root_path parameter (e.g., root_path: \"/Users/yourname/blog\")"},
				}
			}

			// Save the provided root_path to config
			if globalConfig == nil {
				cfg := getDefaultConfig()
				globalConfig = &cfg
			}
			globalConfig.RootPath = args.RootPath
			homeDir, _ := os.UserHomeDir()
			configPath := filepath.Join(homeDir, ".config", "bckt-mcp", "config.toml")
			if err := saveGlobalConfig(configPath, globalConfig); err != nil {
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

Follow these steps:
1. Ask the user for the blog post title
2. Ask for tags (comma-separated list)
3. Ask for a brief abstract (SEO meta abstract)
4. Ask if they want to specify a custom slug (or auto-generate from title)
5. Ask if this should be a draft (true/false)
6. Ask for the language code (default: en)
7. Ask for an excerpt (optional, short summary for listings)

For each one of these steps, provide the user with a value based on the content.

Once you have all the information, use the bckt tool with:
- raw: the content provided below
- meta: object with title, tags (array), abstract, slug (optional), draft, lang, excerpt (optional)

Content to format:
` + content

	messages := []PromptMessage{
		{
			Role:    "user",
			Content: Content{Type: "text", Text: instructions},
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  PromptGetResult{Messages: messages},
	}
}

func loadGlobalConfig() *Config {
	// Try to load from ~/.config/bckt-mcp/config.toml
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	configPath := filepath.Join(homeDir, ".config", "bckt-mcp", "config.toml")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config file
		cfg := getDefaultConfig()
		if err := saveGlobalConfig(configPath, &cfg); err == nil {
			fmt.Fprintf(os.Stderr, "Created default config at: %s\n", configPath)
		}
		return &cfg
	}

	// Load existing config
	var cfg Config
	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config from %s: %v\n", configPath, err)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Loaded config from: %s\n", configPath)
	return &cfg
}

func saveGlobalConfig(path string, cfg *Config) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create file
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write TOML
	encoder := toml.NewEncoder(f)
	return encoder.Encode(cfg)
}

func formatContent(input FormatInput) (*FormatOutput, error) {
	// Start with global config or defaults
	var cfg Config
	if globalConfig != nil {
		cfg = *globalConfig
	} else {
		cfg = getDefaultConfig()
	}

	// Override with inline config if provided
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
		// Load timezone
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			loc = time.UTC
		}
		// Format: "2006-01-02 15:04:05 -0700"
		frontMatter["date"] = time.Now().In(loc).Format("2006-01-02 15:04:05 -0700")
	}

	// Ensure required fields have defaults
	if _, ok := frontMatter["tags"]; !ok {
		frontMatter["tags"] = []string{}
	}
	if _, ok := frontMatter["abstract"]; !ok {
		frontMatter["abstract"] = ""
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
	relativePath := computePath(cfg.PathPattern, dateStr, slug)

	// Prepend root path if configured
	fullPath := relativePath
	if cfg.RootPath != "" {
		fullPath = filepath.Join(cfg.RootPath, relativePath)
	}

	return &FormatOutput{
		Path:     fullPath,
		Markdown: markdown,
		Warnings: warnings,
	}, nil
}

func getDefaultConfig() Config {
	var cfg Config
	cfg.RootPath = "" // Must be set by user on first save
	cfg.Timezone = "UTC"
	cfg.PathPattern = "posts/{yyyy}/{yyyy}-{MM}-{DD}-{slug}/{slug}.md"
	cfg.FrontMatter.Required = []string{"title", "slug", "date", "tags", "abstract", "draft", "lang", "excerpt"}
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
	// Date format: "2006-01-02 15:04:05 -0700" or RFC3339
	// Extract yyyy-MM-dd part
	datePart := date
	if len(date) >= 10 {
		datePart = date[:10] // Get "2025-10-06"
	}

	parts := strings.Split(datePart, "-")
	if len(parts) < 3 {
		// Fallback if date format is unexpected
		return pattern
	}

	yyyy := parts[0]
	mm := parts[1]
	dd := parts[2]

	path := strings.ReplaceAll(pattern, "{yyyy}", yyyy)
	path = strings.ReplaceAll(path, "{MM}", mm)
	path = strings.ReplaceAll(path, "{DD}", dd)
	path = strings.ReplaceAll(path, "{slug}", slug)

	return path
}
