import { AlertTriangle, Zap } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { FOCUS_OPTIONS, type ReviewDefaults } from "@/lib/defaults";
import { providerLabel } from "@/lib/providers";
import { cn } from "@/lib/utils";
import type { ProviderModels } from "@/types/api";

type ReviewConfigFieldsProps = {
  value: ReviewDefaults;
  onChange: (next: ReviewDefaults) => void;
  providers: ProviderModels[];
  modelsLoading?: boolean;
  modelsError?: string | null;
  onRetryModels?: () => void;
  onUnavailableProvider?: (provider: string) => void;
};

export function ReviewConfigFields({
  value,
  onChange,
  providers,
  modelsLoading = false,
  modelsError = null,
  onRetryModels,
  onUnavailableProvider,
}: ReviewConfigFieldsProps) {
  const catalog = providers.length > 0 ? providers : [{ provider: value.provider, models: [], available: true, error: null }];
  const active = catalog.find((item) => item.provider === value.provider) ?? catalog[0];

  const patch = (partial: Partial<ReviewDefaults>) => onChange({ ...value, ...partial });

  const toggleFocus = (area: string) => {
    patch({
      focus: value.focus.includes(area)
        ? value.focus.filter((item) => item !== area)
        : [...value.focus, area],
    });
  };

  return (
    <div className="space-y-6">
      <div className="space-y-3">
        <Label className="text-sm text-muted-foreground">AI provider</Label>
        <div className="flex flex-wrap gap-2">
          {catalog.map((item) => {
            const selected = item.provider === value.provider;
            return (
              <button
                key={item.provider}
                type="button"
                aria-label={
                  item.available
                    ? providerLabel(item.provider)
                    : `${providerLabel(item.provider)} (not configured)`
                }
                onClick={() => {
                  if (!item.available && onUnavailableProvider) {
                    onUnavailableProvider(item.provider);
                    return;
                  }
                  patch({ provider: item.provider, model: "" });
                }}
                className={cn(
                  "rounded-md border px-3 py-1.5 font-mono text-xs transition-colors",
                  selected
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-border bg-surface text-muted-foreground hover:border-primary/40 hover:text-foreground",
                  !item.available && "opacity-40",
                )}
              >
                {providerLabel(item.provider)}
              </button>
            );
          })}
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="review-model" className="text-sm text-muted-foreground">
          Model
        </Label>
        <select
          id="review-model"
          value={value.model}
          onChange={(event) => patch({ model: event.target.value })}
          disabled={modelsLoading || !active?.available || (active?.models.length ?? 0) === 0}
          className="flex h-9 w-full rounded-md border border-border bg-surface px-3 py-1 font-mono text-sm shadow-sm outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          <option value="">{modelsLoading ? "Loading models…" : "Select a model"}</option>
          {active?.models.map((model) => (
            <option key={model} value={model}>
              {model}
            </option>
          ))}
        </select>
        {active && !active.available && (
          <p className="text-xs text-destructive">{active.error}</p>
        )}
        {active?.available && active.models.length === 0 && !modelsLoading && (
          <p className="text-xs text-muted-foreground">No review-capable models for this provider.</p>
        )}
        {modelsError && (
          <div className="flex items-center gap-2 text-xs text-destructive">
            <p>{modelsError}</p>
            {onRetryModels && (
              <Button variant="outline" size="sm" onClick={onRetryModels}>
                Retry
              </Button>
            )}
          </div>
        )}
      </div>

      <div className="space-y-3">
        <Label className="text-sm text-muted-foreground">Focus areas</Label>
        <div className="flex flex-wrap gap-2">
          {FOCUS_OPTIONS.map((area) => (
            <Badge
              key={area}
              role="checkbox"
              aria-checked={value.focus.includes(area)}
              tabIndex={0}
              variant={value.focus.includes(area) ? "default" : "outline"}
              className={cn(
                "cursor-pointer select-none px-3 py-1 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                value.focus.includes(area)
                  ? "bg-primary text-primary-foreground hover:bg-primary/80"
                  : "hover:bg-muted",
              )}
              onClick={() => toggleFocus(area)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  toggleFocus(area);
                }
              }}
            >
              {area}
            </Badge>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="max-comments" className="text-sm text-muted-foreground">
          Max comments
        </Label>
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            aria-label="Decrease max comments"
            className="h-8 w-8 p-0"
            onClick={() => patch({ maxComments: Math.max(1, value.maxComments - 1) })}
          >
            -
          </Button>
          <Input
            id="max-comments"
            type="number"
            min={1}
            max={50}
            value={value.maxComments}
            onChange={(event) =>
              patch({ maxComments: Math.min(50, Math.max(1, Number(event.target.value) || 1)) })
            }
            className="w-20 text-center font-mono text-sm bg-surface border-border"
          />
          <Button
            variant="outline"
            size="sm"
            aria-label="Increase max comments"
            className="h-8 w-8 p-0"
            onClick={() => patch({ maxComments: Math.min(50, value.maxComments + 1) })}
          >
            +
          </Button>
        </div>
      </div>

      <div className="space-y-4 rounded-lg border border-border bg-surface p-4">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <Label htmlFor="auto-post" className="text-sm">Review mode</Label>
            {value.autoPost && (
              <span className="inline-flex items-center gap-1 rounded bg-severity-warning/10 px-2 py-0.5 text-xs text-severity-warning border border-severity-warning/20">
                <AlertTriangle className="h-3 w-3" />
                Auto-post
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">Manual</span>
            <Switch
              id="auto-post"
              checked={value.autoPost}
              onCheckedChange={(autoPost) => patch({ autoPost })}
            />
            <span className="text-xs text-muted-foreground">Auto-post</span>
          </div>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Zap className="h-4 w-4 text-muted-foreground" />
            <Label htmlFor="parallel-review" className="text-sm">Parallel mode</Label>
          </div>
          <Switch
            id="parallel-review"
            checked={value.parallel}
            onCheckedChange={(parallel) => patch({ parallel })}
          />
        </div>
      </div>
    </div>
  );
}
