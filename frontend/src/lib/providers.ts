export const BUILTIN_PROVIDERS = [
  "anthropic",
  "openai",
  "xai",
  "google",
  "kimi",
  "deepseek",
  "ollama",
] as const;

export const DEFAULT_MODELS: Record<string, string> = {
  anthropic: "claude-sonnet-4-5",
  openai: "gpt-4o",
  xai: "grok-4",
  google: "gemini-2.5-flash",
  kimi: "moonshot-v1",
  deepseek: "deepseek-chat",
  ollama: "llama3.2",
  echo: "echo",
};

const PROVIDER_LABELS: Record<string, string> = {
  anthropic: "Anthropic",
  openai: "OpenAI",
  xai: "xAI",
  google: "Google",
  kimi: "Kimi",
  deepseek: "DeepSeek",
  ollama: "Ollama",
  echo: "Echo",
};

export function canonicalProvider(id: string): string {
  const name = id.trim().toLowerCase();
  return name === "gemini" ? "google" : name;
}

const PROVIDER_FOCUS: Record<string, string> = {
  anthropic: "ANTHROPIC_API_KEY",
  openai: "OPENAI_API_KEY",
  xai: "XAI_API_KEY",
  google: "GEMINI_API_KEY",
  kimi: "KIMI_API_KEY",
  deepseek: "DEEPSEEK_API_KEY",
  ollama: "OLLAMA_HOST",
};

export function providerLabel(id: string): string {
  const name = canonicalProvider(id);
  return PROVIDER_LABELS[name] ?? name;
}

export function settingsFocusFor(provider: string): string {
  return PROVIDER_FOCUS[canonicalProvider(provider)] ?? "";
}

export function defaultModelFor(provider: string): string {
  return DEFAULT_MODELS[canonicalProvider(provider)] ?? "";
}

export function pickModel(provider: string, models: string[], preferred = ""): string {
  if (preferred && models.includes(preferred)) return preferred;
  const pinned = defaultModelFor(provider);
  if (pinned && models.includes(pinned)) return pinned;
  return models[0] ?? preferred ?? pinned;
}
