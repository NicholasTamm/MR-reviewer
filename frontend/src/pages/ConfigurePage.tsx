import { useState, useEffect } from "react";
import { useNavigate, Link } from "react-router-dom";
import { Play, AlertTriangle, Zap, Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { getConfigDefaults, submitReview, ApiError } from "@/lib/api";
import { useReview } from "@/context/ReviewContext";

const FOCUS_OPTIONS = [
  "bugs",
  "style",
  "best-practices",
  "security",
  "performance",
];

const PROVIDER_META: Record<string, { name: string; description: string }> = {
  anthropic: { name: "Anthropic", description: "Claude — best overall" },
  gemini: { name: "Gemini", description: "Google — fast, free tier" },
  ollama: { name: "Ollama", description: "Local — fully private" },
};

const MR_URL_PATTERN = /^https?:\/\/.+\/(merge_requests|pull)\/\d+/;

export function ConfigurePage() {
  const navigate = useNavigate();
  const { reset } = useReview();

  const [url, setUrl] = useState("");
  const [provider, setProvider] = useState("anthropic");
  const [model, setModel] = useState("");
  const [focusAreas, setFocusAreas] = useState<string[]>(["bugs", "style", "best-practices"]);
  const [maxComments, setMaxComments] = useState(10);
  const [autoPost, setAutoPost] = useState(false);
  const [parallel, setParallel] = useState(false);
  const [credentialsPresent, setCredentialsPresent] = useState<Record<string, boolean>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [urlTouched, setUrlTouched] = useState(false);

  useEffect(() => {
    reset();
  }, [reset]);

  useEffect(() => {
    let cancelled = false;
    getConfigDefaults()
      .then((defaults) => {
        if (cancelled) return;
        setProvider(defaults.provider);
        setModel(defaults.model);
        setFocusAreas(defaults.focus);
        setMaxComments(defaults.max_comments);
        setParallel(defaults.parallel);
        setCredentialsPresent(defaults.credentials_present ?? {});
      })
      .catch(() => {});
    return () => { cancelled = true; };
  }, []);

  const providerCredentialKey: Record<string, string> = {
    anthropic: "ANTHROPIC_API_KEY",
    gemini: "GEMINI_API_KEY",
    ollama: "OLLAMA_HOST",
  };
  const currentProviderKey = providerCredentialKey[provider];
  const showCredentialWarning =
    currentProviderKey !== undefined && credentialsPresent[currentProviderKey] === false;

  const isUrlValid = MR_URL_PATTERN.test(url);
  const showUrlError = urlTouched && url.length > 0 && !isUrlValid;

  const toggleFocus = (area: string) => {
    setFocusAreas((prev) =>
      prev.includes(area) ? prev.filter((a) => a !== area) : [...prev, area]
    );
  };

  const handleSubmit = async () => {
    if (!isUrlValid) {
      setUrlTouched(true);
      return;
    }
    setIsSubmitting(true);
    setError(null);
    try {
      const result = await submitReview({
        url,
        provider,
        model: model || null,
        focus: focusAreas,
        max_comments: maxComments,
        parallel,
        auto_post: autoPost,
      });
      navigate(`/review/${result.job_id}`);
    } catch (err) {
      setError(
        err instanceof ApiError || err instanceof Error
          ? err.message
          : "An unexpected error occurred"
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="max-w-xl mx-auto space-y-8 page-transition">

      {/* URL input */}
      <div className="space-y-2">
        <Label htmlFor="url" className="text-xs font-mono uppercase tracking-widest text-muted-foreground">
          MR / PR URL
        </Label>
        <Input
          id="url"
          type="url"
          placeholder="https://github.com/owner/repo/pull/123"
          value={url}
          onChange={(e) => { setUrl(e.target.value); setError(null); }}
          onBlur={() => setUrlTouched(true)}
          onKeyDown={(e) => { if (e.key === "Enter") handleSubmit(); }}
          className={cn(
            "h-11 font-mono text-sm bg-surface border-border placeholder:text-muted-foreground/30 focus-visible:ring-primary",
            showUrlError && "border-destructive"
          )}
        />
        {showUrlError && (
          <p className="text-xs text-destructive font-mono">
            Enter a valid GitLab MR or GitHub PR URL
          </p>
        )}
      </div>

      {/* Provider */}
      <div className="space-y-2">
        <Label className="text-xs font-mono uppercase tracking-widest text-muted-foreground">
          AI Provider
        </Label>
        <div className="grid grid-cols-3 gap-2">
          {["anthropic", "gemini", "ollama"].map((p) => {
            const meta = PROVIDER_META[p];
            const isSelected = provider === p;
            return (
              <button
                key={p}
                onClick={() => setProvider(p)}
                className={cn(
                  "provider-card flex flex-col gap-1 rounded border p-3 text-left",
                  isSelected
                    ? "border-primary bg-primary/5 text-foreground"
                    : "border-border bg-surface text-muted-foreground hover:border-muted-foreground/20 hover:text-foreground"
                )}
              >
                <span className={cn(
                  "text-sm font-heading font-600",
                  isSelected ? "text-primary" : "text-foreground"
                )}>
                  {meta.name}
                </span>
                <span className="text-[10px] leading-snug">{meta.description}</span>
              </button>
            );
          })}
        </div>
        {showCredentialWarning && (
          <p className="flex items-center gap-1.5 text-xs text-severity-warning font-mono">
            <AlertTriangle className="h-3 w-3 shrink-0" />
            API key not set —{" "}
            <Link to="/settings" className="underline underline-offset-2 hover:text-severity-warning/70">
              configure in Settings
            </Link>
          </p>
        )}
      </div>

      {/* Model */}
      <div className="space-y-2">
        <Label htmlFor="model" className="text-xs font-mono uppercase tracking-widest text-muted-foreground">
          Model
        </Label>
        <Input
          id="model"
          value={model}
          onChange={(e) => setModel(e.target.value)}
          placeholder="Leave blank for provider default"
          className="font-mono text-sm bg-surface border-border focus-visible:ring-primary placeholder:text-muted-foreground/30"
        />
      </div>

      {/* Focus Areas */}
      <div className="space-y-2">
        <Label className="text-xs font-mono uppercase tracking-widest text-muted-foreground">
          Focus Areas
        </Label>
        <div className="flex flex-wrap gap-1.5">
          {FOCUS_OPTIONS.map((area) => {
            const isActive = focusAreas.includes(area);
            return (
              <button
                key={area}
                role="checkbox"
                aria-checked={isActive}
                onClick={() => toggleFocus(area)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") { e.preventDefault(); toggleFocus(area); }
                }}
                className={cn(
                  "px-3 py-1 text-xs font-mono rounded-full border transition-colors select-none",
                  isActive
                    ? "border-primary text-primary bg-primary/8"
                    : "border-border text-muted-foreground hover:border-muted-foreground/30 hover:text-foreground"
                )}
              >
                {area}
              </button>
            );
          })}
        </div>
      </div>

      {/* Max Comments */}
      <div className="space-y-2">
        <Label className="text-xs font-mono uppercase tracking-widest text-muted-foreground">
          Max Comments
        </Label>
        <div className="flex items-center gap-3">
          <button
            aria-label="Decrease max comments"
            onClick={() => setMaxComments(Math.max(1, maxComments - 1))}
            className="h-8 w-8 flex items-center justify-center rounded border border-border bg-surface text-muted-foreground hover:text-foreground hover:border-muted-foreground/30 transition-colors text-sm font-mono"
          >
            −
          </button>
          <span className="w-8 text-center font-mono text-sm text-foreground tabular-nums">
            {maxComments}
          </span>
          <button
            aria-label="Increase max comments"
            onClick={() => setMaxComments(Math.min(50, maxComments + 1))}
            className="h-8 w-8 flex items-center justify-center rounded border border-border bg-surface text-muted-foreground hover:text-foreground hover:border-muted-foreground/30 transition-colors text-sm font-mono"
          >
            +
          </button>
        </div>
      </div>

      {/* Toggles */}
      <div className="rounded border border-border bg-surface divide-y divide-border">
        <div className="flex items-center justify-between px-4 py-3">
          <div className="space-y-0.5">
            <p className="text-sm font-medium">Auto-post</p>
            <p className="text-xs text-muted-foreground">Post comments immediately without review</p>
          </div>
          <div className="flex items-center gap-2">
            {autoPost && (
              <span className="flex items-center gap-1 text-[10px] text-severity-warning font-mono">
                <AlertTriangle className="h-3 w-3" />
                enabled
              </span>
            )}
            <Switch checked={autoPost} onCheckedChange={setAutoPost} />
          </div>
        </div>
        <div className="flex items-center justify-between px-4 py-3">
          <div className="flex items-center gap-2.5">
            <Zap className="h-4 w-4 text-muted-foreground" />
            <div className="space-y-0.5">
              <p className="text-sm font-medium">Parallel Mode</p>
              <p className="text-xs text-muted-foreground">Split large PRs across multiple agents</p>
            </div>
          </div>
          <Switch checked={parallel} onCheckedChange={setParallel} />
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="rounded border border-destructive/20 bg-destructive/5 px-4 py-3 text-sm text-destructive font-mono">
          {error}
        </div>
      )}

      {/* Submit */}
      <button
        onClick={handleSubmit}
        disabled={isSubmitting}
        className={cn(
          "w-full h-11 flex items-center justify-center gap-2 rounded text-sm font-heading font-700 transition-all",
          "bg-primary text-primary-foreground hover:bg-primary/90 active:scale-[0.99]",
          "disabled:opacity-50 disabled:cursor-not-allowed"
        )}
      >
        {isSubmitting ? (
          <>
            <Loader2 className="h-4 w-4 animate-spin" />
            Submitting…
          </>
        ) : (
          <>
            <Play className="h-4 w-4" />
            Run Review
          </>
        )}
      </button>
    </div>
  );
}
