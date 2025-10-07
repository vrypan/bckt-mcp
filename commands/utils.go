package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, path[2:])
		}
	}
	return path
}

func GetDefaultConfig() Config {
	var cfg Config
	cfg.RootPath = "" // Must be set by user on first save
	cfg.Timezone = "UTC"
	cfg.PathPattern = "posts/{yyyy}/{yyyy}-{MM}-{DD}-{slug}/{slug}.md"
	cfg.FrontMatter.Required = []string{"title", "slug", "date", "tags", "abstract", "lang"}
	cfg.FrontMatter.Defaults = map[string]interface{}{
		"lang": "en",
	}
	cfg.MarkdownRule.WrapAt = 100
	return cfg
}

func LoadGlobalConfig() *Config {
	// Try to load from ~/.config/bckt-mcp/config.toml
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	configPath := filepath.Join(homeDir, ".config", "bckt-mcp", "config.toml")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config file
		cfg := GetDefaultConfig()
		if err := SaveGlobalConfig(configPath, &cfg); err == nil {
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

func SaveGlobalConfig(path string, cfg *Config) error {
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

func FormatContent(input FormatInput, globalConfig *Config) (*FormatOutput, error) {
	// Start with global config or defaults
	var cfg Config
	if globalConfig != nil {
		cfg = *globalConfig
	} else {
		cfg = GetDefaultConfig()
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

	// Validate front matter
	warnings, err := validateFrontMatter(frontMatter, cfg, input.Strategy != "lenient")
	if err != nil {
		return nil, err
	}

	// Wrap abstract if present
	if abstract, ok := frontMatter["abstract"].(string); ok && abstract != "" {
		frontMatter["abstract"] = wrapText(abstract, cfg.MarkdownRule.WrapAt)
	}

	// Format body text
	body := wrapText(input.Raw, cfg.MarkdownRule.WrapAt)

	// Generate YAML front matter with literal style for multiline fields
	var yamlBuf bytes.Buffer
	encoder := yaml.NewEncoder(&yamlBuf)
	encoder.SetIndent(2)
	if err := encoder.Encode(frontMatter); err != nil {
		return nil, fmt.Errorf("failed to generate YAML: %v", err)
	}
	encoder.Close()
	yamlData := yamlBuf.Bytes()

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
