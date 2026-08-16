package provider

import "encoding/json"

// ReviewToolSchema is the submit_review JSON schema shared by all providers.
const ReviewToolName = "submit_review"

const reviewToolJSON = `{
  "type": "object",
  "properties": {
    "summary": {"type": "string", "description": "Overall review summary"},
    "comments": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "file": {"type": "string"},
          "line": {"type": "integer"},
          "body": {"type": "string"},
          "severity": {"type": "string", "enum": ["info", "warning", "error"]}
        },
        "required": ["file", "line", "body", "severity"]
      }
    }
  },
  "required": ["summary", "comments"]
}`

func reviewToolSchema() json.RawMessage {
	return json.RawMessage(reviewToolJSON)
}

// DefaultModel returns the catalog pin for a built-in provider.
func DefaultModel(provider string) string {
	switch Canonical(provider) {
	case "openai":
		return "gpt-4o"
	case "xai":
		return "grok-4"
	case "google":
		return "gemini-2.5-flash"
	case "kimi":
		return "moonshot-v1"
	case "deepseek":
		return "deepseek-chat"
	case "echo":
		return "echo"
	case "ollama":
		return "llama3.2"
	default:
		return "claude-sonnet-4-5"
	}
}

// Names is the strike-set of built-in model providers plus echo/ollama.
func Names() []string {
	return []string{"anthropic", "openai", "xai", "google", "kimi", "deepseek", "echo", "ollama"}
}

func Canonical(id string) string {
	switch id {
	case "gemini":
		return "google"
	default:
		return id
	}
}

func BaseURL(name string) string {
	switch Canonical(name) {
	case "openai":
		return "https://api.openai.com/v1"
	case "xai":
		return "https://api.x.ai/v1"
	case "kimi":
		return "https://api.moonshot.cn/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "ollama":
		return "http://localhost:11434/v1"
	case "google":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "anthropic":
		return "https://api.anthropic.com"
	default:
		return ""
	}
}
