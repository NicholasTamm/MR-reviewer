import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { ReviewConfigFields } from "@/components/ReviewConfigFields";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { getConfigDefaults, getProviderModels } from "@/lib/api";
import {
  defaultsFromEnvMap,
  defaultsToEnvMap,
  loadLocalDefaults,
  resolveDefaults,
  saveLocalDefaults,
  type ReviewDefaults,
} from "@/lib/defaults";
import { pickModel, settingsFocusFor } from "@/lib/providers";
import { cn } from "@/lib/utils";
import type { ProviderModels } from "@/types/api";

const CREDENTIALS = [
  { key: "GITLAB_TOKEN", label: "GitLab token" },
  { key: "GITHUB_TOKEN", label: "GitHub token" },
  { key: "ANTHROPIC_API_KEY", label: "Anthropic" },
  { key: "OPENAI_API_KEY", label: "OpenAI" },
  { key: "XAI_API_KEY", label: "xAI" },
  { key: "GEMINI_API_KEY", label: "Google / Gemini" },
  { key: "KIMI_API_KEY", label: "Kimi" },
  { key: "DEEPSEEK_API_KEY", label: "DeepSeek" },
] as const;

export function SettingsPage() {
  const electronAPI = window.electronAPI;
  const [searchParams] = useSearchParams();
  const focusId = searchParams.get("focus") || settingsFocusFor(searchParams.get("provider") ?? "");
  const [config, setConfig] = useState<ReviewDefaults>(resolveDefaults(null, loadLocalDefaults()));
  const [hosts, setHosts] = useState({
    MR_REVIEWER_GITLAB_URL: "",
    MR_REVIEWER_GITHUB_API: "",
    OLLAMA_HOST: "",
  });
  const [secrets, setSecrets] = useState<Record<string, string>>({});
  const [present, setPresent] = useState<Record<string, boolean>>({});
  const [providers, setProviders] = useState<ProviderModels[]>([]);
  const [modelsLoading, setModelsLoading] = useState(true);
  const [modelsError, setModelsError] = useState<string | null>(null);
  const [modelRequest, setModelRequest] = useState(0);
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      getConfigDefaults().catch(() => null),
      electronAPI?.getRuntimeSettings() ?? Promise.resolve(null),
    ]).then(([server, runtime]) => {
      if (cancelled) return;
      const env = runtime ? defaultsFromEnvMap(runtime.settings) : {};
      setConfig(resolveDefaults(server, loadLocalDefaults(), env));
      if (runtime) {
        setHosts({
          MR_REVIEWER_GITLAB_URL: runtime.settings.MR_REVIEWER_GITLAB_URL || "",
          MR_REVIEWER_GITHUB_API: runtime.settings.MR_REVIEWER_GITHUB_API || "",
          OLLAMA_HOST: runtime.settings.OLLAMA_HOST || "",
        });
        setPresent(runtime.credentials);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [electronAPI]);

  useEffect(() => {
    let cancelled = false;
    setModelsLoading(true);
    setModelsError(null);
    getProviderModels()
      .then((catalog) => {
        if (!cancelled) setProviders(catalog.providers);
      })
      .catch(() => {
        if (!cancelled) setModelsError("Unable to load available models.");
      })
      .finally(() => {
        if (!cancelled) setModelsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [modelRequest]);

  useEffect(() => {
    if (modelsLoading) return;
    const active = providers.find((item) => item.provider === config.provider);
    if (!active?.available || active.models.length === 0) return;
    if (config.model && active.models.includes(config.model)) return;
    setConfig((current) => ({
      ...current,
      model: pickModel(current.provider, active.models, current.model),
    }));
  }, [modelsLoading, providers, config.provider, config.model]);

  useEffect(() => {
    if (!focusId) return;
    const node = document.getElementById(focusId);
    if (!(node instanceof HTMLElement)) return;
    node.scrollIntoView({ behavior: "smooth", block: "center" });
    node.focus();
  }, [focusId, electronAPI]);

  const saveDefaults = async () => {
    setSaving(true);
    setMessage("");
    try {
      saveLocalDefaults(config);
      if (electronAPI) {
        await electronAPI.saveRuntimeSettings({
          ...defaultsToEnvMap(config),
          ...hosts,
        });
      }
      setMessage("Review defaults saved.");
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Unable to save defaults.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-10 page-transition">
      <div>
        <p className="text-[11px] font-mono uppercase tracking-[0.22em] text-muted-foreground">Preferences</p>
        <h1 className="mt-1 text-xl font-medium tracking-tight">Settings</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          These defaults fill the configuration panel when you start a review.
        </p>
      </div>

      <section className="space-y-4">
        <h2 className="text-sm font-medium">Review defaults</h2>
        <ReviewConfigFields
          value={config}
          onChange={setConfig}
          providers={providers}
          modelsLoading={modelsLoading}
          modelsError={modelsError}
          onRetryModels={() => setModelRequest((n) => n + 1)}
        />
        <Button onClick={saveDefaults} disabled={saving}>
          {saving ? "Saving…" : "Save defaults"}
        </Button>
      </section>

      {electronAPI && (
        <>
          <section className="space-y-4">
            <div>
              <h2 className="text-sm font-medium">Hosts</h2>
              <p className="mt-1 text-sm text-muted-foreground">Optional overrides for self-hosted endpoints.</p>
            </div>
            <HostField
              id="MR_REVIEWER_GITLAB_URL"
              label="GitLab URL"
              value={hosts.MR_REVIEWER_GITLAB_URL}
              placeholder="https://gitlab.com"
              focused={focusId === "MR_REVIEWER_GITLAB_URL"}
              onChange={(value) => setHosts((current) => ({ ...current, MR_REVIEWER_GITLAB_URL: value }))}
            />
            <HostField
              id="MR_REVIEWER_GITHUB_API"
              label="GitHub API"
              value={hosts.MR_REVIEWER_GITHUB_API}
              placeholder="https://api.github.com"
              focused={focusId === "MR_REVIEWER_GITHUB_API"}
              onChange={(value) => setHosts((current) => ({ ...current, MR_REVIEWER_GITHUB_API: value }))}
            />
            <HostField
              id="OLLAMA_HOST"
              label="Ollama host"
              value={hosts.OLLAMA_HOST}
              placeholder="http://localhost:11434"
              focused={focusId === "OLLAMA_HOST"}
              onChange={(value) => setHosts((current) => ({ ...current, OLLAMA_HOST: value }))}
            />
          </section>

          <section className="space-y-4">
            <div>
              <h2 className="text-sm font-medium">Credentials</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Stored in the OS keychain. Onboarding still writes the shared ~/.mr-reviewer store.
              </p>
            </div>
            {CREDENTIALS.map((item) => (
              <div key={item.key} className="flex items-end gap-2">
                <div className="flex-1 space-y-1.5">
                  <Label htmlFor={item.key} className="text-sm text-muted-foreground">
                    {item.label}
                    {present[item.key] ? " · configured" : ""}
                  </Label>
                  <Input
                    id={item.key}
                    type="password"
                    value={secrets[item.key] ?? ""}
                    placeholder={present[item.key] ? "••••••••" : "Not configured"}
                    onChange={(event) => setSecrets((current) => ({ ...current, [item.key]: event.target.value }))}
                    className={cn("h-9 bg-surface", focusId === item.key && "ring-2 ring-ring")}
                  />
                </div>
                <Button
                  variant="outline"
                  onClick={() =>
                    electronAPI.saveCredential(item.key, secrets[item.key] || "").then(() => {
                      setMessage(`${item.label} saved`);
                      setPresent((current) => ({ ...current, [item.key]: true }));
                    })
                  }
                >
                  Save
                </Button>
              </div>
            ))}
          </section>
        </>
      )}

      {!electronAPI && (
        <p className="text-sm text-muted-foreground">
          Credentials live in ~/.mr-reviewer. Review defaults on this page are stored in this browser.
        </p>
      )}

      {message && <p className="text-sm text-success">{message}</p>}
    </div>
  );
}

function HostField({
  id,
  label,
  value,
  placeholder,
  focused,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  placeholder: string;
  focused?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id} className="text-sm text-muted-foreground">{label}</Label>
      <Input
        id={id}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className={cn("h-9 font-mono text-sm bg-surface", focused && "ring-2 ring-ring")}
      />
    </div>
  );
}
