package commands

import "encoding/json"

// JSON-RPC types
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

// Tool input/output types
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

// Configuration types
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
