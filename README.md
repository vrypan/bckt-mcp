# bckt-mcp

A Model Context Protocol (MCP) server that formats blog posts for static site generators. Designed to work with Claude Desktop and other MCP-compatible clients.

## Features

- 📝 Format blog posts with YAML front matter
- 🔧 Configurable path patterns and text wrapping
- 🌍 Timezone-aware date handling
- 📋 Interactive metadata collection
- 💾 Save posts directly to your blog directory
- 👀 Preview before saving

## Installation

### Homebrew (macOS)

```bash
brew install vrypan/bckt-mcp/bckt-mcp
```

### Download Binary

Download the latest release for your platform from the [releases page](https://github.com/vrypan/bckt-mcp/releases).

### Build from Source

```bash
git clone https://github.com/vrypan/bckt-mcp.git
cd bckt-mcp
go build -o bckt-mcp main.go
```

## Configuration

### Claude Desktop

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "bckt": {
      "command": "/path/to/bckt-mcp"
    }
  }
}
```

Or if installed via Homebrew:

```json
{
  "mcpServers": {
    "bckt": {
      "command": "/opt/homebrew/bin/bckt-mcp"
    }
  }
}
```

### First-Time Setup

On first use, run the setup wizard through Claude:

```
setup bckt
```

You'll be prompted for:
- **root_path**: Where your blog posts will be saved (e.g., `~/blog`)
- **timezone**: Your timezone (e.g., `America/New_York`, `Europe/Athens`, `UTC`)
- **path_pattern** (optional): Template for file paths (default: `posts/{yyyy}/{yyyy}-{MM}-{DD}-{slug}/{slug}.md`)
- **wrap_at** (optional): Maximum line width for text wrapping (default: `100`)

Configuration is saved to `~/.config/bckt-mcp/config.toml`.

## Usage

### Tools Available

#### `bckt_setup`
Interactive setup wizard for first-time configuration.

#### `bckt_config`
View or update configuration settings.

#### `bckt_preview`
Preview the formatted output without saving.

#### `bckt`
Format the blog post content with metadata.

#### `bckt_save`
Save the formatted markdown to the configured path.

### Example Workflow with Claude

1. **Setup** (first time only):
   ```
   setup bckt
   ```

2. **Format a blog post**:
   ```
   Format this blog post: [paste your content]
   ```
   Claude will:
   - Ask for title, tags, abstract, slug, and language
   - Show you a preview
   - Ask if you want to save

3. **View configuration**:
   ```
   show bckt config
   ```

4. **Update configuration**:
   ```
   update bckt timezone to Europe/London
   ```

## Front Matter Fields

The generated front matter includes:

- `title`: Post title
- `slug`: URL-friendly slug (auto-generated from title if not provided)
- `date`: Publication date with timezone
- `tags`: Array of tags
- `abstract`: SEO meta description (wrapped to configured width)
- `lang`: Language code (default: `en`)

## Path Pattern Placeholders

- `{yyyy}`: Year (e.g., `2025`)
- `{MM}`: Month (e.g., `01`)
- `{DD}`: Day (e.g., `07`)
- `{slug}`: Post slug

Example: `posts/{yyyy}/{yyyy}-{MM}-{DD}-{slug}/{slug}.md` generates:
```
posts/2025/2025-10-07-my-post/my-post.md
```

## Configuration File

Located at `~/.config/bckt-mcp/config.toml`:

```toml
root_path = "/Users/username/blog"
timezone = "Europe/Athens"
path_pattern = "posts/{yyyy}/{yyyy}-{MM}-{DD}-{slug}/{slug}.md"

[front_matter]
required = ["title", "slug", "date", "tags", "abstract", "lang"]

[front_matter.defaults]
lang = "en"

[markdown_rules]
wrap_at = 100
```

## Development

### Requirements

- Go 1.21 or later

### Build

```bash
go build -o bckt-mcp main.go
```

### Test

```bash
go test ./...
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Author

Panayotis Vryonis ([@vrypan](https://github.com/vrypan))
