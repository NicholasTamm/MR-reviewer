import { useState, useEffect, useCallback } from "react";
import { Eye, EyeOff, CheckCircle2, Circle } from "lucide-react";
import { toast } from "sonner";
import { Label } from "@/components/ui/label";
import { getConfigDefaults } from "@/lib/api";

interface CredentialField {
  key: string;
  label: string;
  description: string;
  placeholder?: string;
  isPassword: boolean;
}

const AI_PROVIDER_FIELDS: CredentialField[] = [
  {
    key: "ANTHROPIC_API_KEY",
    label: "Anthropic API Key",
    description: "Claude — required for Anthropic provider",
    isPassword: true,
  },
  {
    key: "GEMINI_API_KEY",
    label: "Gemini API Key",
    description: "Google — required for Gemini provider",
    isPassword: true,
  },
  {
    key: "OLLAMA_HOST",
    label: "Ollama Host",
    description: "Local Ollama server URL",
    placeholder: "http://localhost:11434",
    isPassword: false,
  },
];

const PLATFORM_FIELDS: CredentialField[] = [
  {
    key: "GITLAB_TOKEN",
    label: "GitLab Token",
    description: "Personal access token with 'api' scope",
    isPassword: true,
  },
  {
    key: "GITHUB_TOKEN",
    label: "GitHub Token",
    description: "Personal access token",
    isPassword: true,
  },
];

interface CredentialRowProps {
  field: CredentialField;
  isPresent: boolean;
  onSaved: () => void;
}

function CredentialRow({ field, isPresent, onSaved }: CredentialRowProps) {
  const [value, setValue] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!value.trim()) return;
    setSaving(true);
    try {
      if (window.electronAPI) {
        await window.electronAPI.setCredential(field.key, value.trim());
        setValue("");
        toast.success(`${field.label} saved`);
        onSaved();
      } else {
        toast.info("Set credentials via environment variables in web mode");
        return;
      }
    } catch {
      toast.error("Failed to save");
    } finally {
      setSaving(false);
    }
  };

  const inputType = field.isPassword ? (showPassword ? "text" : "password") : "text";

  return (
    <div className="flex items-start gap-4 py-4">
      {/* Status dot */}
      <div className="pt-0.5 shrink-0">
        {isPresent ? (
          <CheckCircle2 className="h-4 w-4 text-success" />
        ) : (
          <Circle className="h-4 w-4 text-muted-foreground/30" />
        )}
      </div>

      {/* Field */}
      <div className="flex-1 min-w-0 space-y-2">
        <div>
          <Label className="text-sm font-medium text-foreground">{field.label}</Label>
          <p className="text-xs text-muted-foreground mt-0.5">{field.description}</p>
        </div>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <input
              type={inputType}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder={field.placeholder ?? (field.isPassword ? "Enter new value…" : "")}
              onKeyDown={(e) => { if (e.key === "Enter") handleSave(); }}
              className="w-full h-9 px-3 rounded border border-border bg-surface text-sm font-mono placeholder:text-muted-foreground/30 focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary transition-colors pr-9"
            />
            {field.isPassword && (
              <button
                type="button"
                onClick={() => setShowPassword((s) => !s)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                aria-label={showPassword ? "Hide" : "Show"}
              >
                {showPassword ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              </button>
            )}
          </div>
          <button
            onClick={handleSave}
            disabled={saving || !value.trim()}
            className="px-3 h-9 text-xs font-medium rounded border border-border bg-surface hover:border-muted-foreground/30 text-foreground disabled:opacity-30 disabled:cursor-not-allowed transition-colors shrink-0"
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}

export function SettingsPage() {
  const [credentialsPresent, setCredentialsPresent] = useState<Record<string, boolean>>({});
  const allFields = [...AI_PROVIDER_FIELDS, ...PLATFORM_FIELDS];

  const fetchCredentialStatus = useCallback(async () => {
    if (window.electronAPI) {
      const results: Record<string, boolean> = {};
      await Promise.all(
        allFields.map(async (field) => {
          const val = await window.electronAPI!.getCredential(field.key);
          results[field.key] = val !== null && val !== "";
        }),
      );
      setCredentialsPresent(results);
    } else {
      getConfigDefaults()
        .then((defaults) => setCredentialsPresent(defaults.credentials_present ?? {}))
        .catch(() => {});
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    fetchCredentialStatus();
  }, [fetchCredentialStatus]);

  const renderSection = (title: string, fields: CredentialField[]) => (
    <div className="space-y-1">
      <h2 className="text-xs font-mono uppercase tracking-widest text-muted-foreground px-1 pb-1">
        {title}
      </h2>
      <div className="rounded border border-border bg-surface divide-y divide-border/60 px-4">
        {fields.map((field) => (
          <CredentialRow
            key={field.key}
            field={field}
            isPresent={credentialsPresent[field.key] ?? false}
            onSaved={fetchCredentialStatus}
          />
        ))}
      </div>
    </div>
  );

  return (
    <div className="max-w-xl mx-auto space-y-8 page-transition">
      <div>
        <h1 className="text-lg font-heading font-700">Settings</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Credentials are saved to the OS keychain and never leave your machine.
          Environment variables take precedence.
        </p>
      </div>
      {renderSection("AI Providers", AI_PROVIDER_FIELDS)}
      {renderSection("Platforms", PLATFORM_FIELDS)}
    </div>
  );
}
