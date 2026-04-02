import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { Play, AlertTriangle, Zap, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
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

const MR_URL_PATTERN =
  /^https?:\/\/.+\/(merge_requests|pull)\/\d+/;

export function ConfigurePage() {
  const navigate = useNavigate();
  const { reset } = useReview();

  const [url, setUrl] = useState("");
  const [provider, setProvider] = useState("anthropic");
  const [model, setModel] = useState("");
  const [focusAreas, setFocusAreas] = useState<string[]>([
    "bugs",
    "style",
    "best-practices",
  ]);
  const [maxComments, setMaxComments] = useState(10);
  const [autoPost, setAutoPost] = useState(false);
  const [parallel, setParallel] = useState(false);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [urlTouched, setUrlTouched] = useState(false);

  // Reset review context when returning to configure page
  useEffect(() => {
    reset();
  }, [reset]);

  // Load server defaults on mount
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
      })
      .catch(() => {
        // Server may not be available; keep client defaults
      });
    return () => {
      cancelled = true;
    };
  }, []);

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
      if (err instanceof ApiError) {
        setError(err.message);
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto space-y-8 page-transition">
      {/* URL input */}
      <div className="space-y-2">
        <Label htmlFor="url" className="text-sm text-muted-foreground">
          Merge Request / Pull Request URL
        </Label>
        <Input
          id="url"
          type="url"
          placeholder="https://github.com/owner/repo/pull/123"
          value={url}
          onChange={(e) => {
            setUrl(e.target.value);
            setError(null);
          }}
          onBlur={() => setUrlTouched(true)}
          className={cn(
            "h-12 font-mono text-sm bg-surface border-border placeholder:text-muted-foreground/40",
            showUrlError && "border-destructive"
          )}
        />
        {showUrlError && (
          <p className="text-xs text-destructive">
            Enter a valid GitLab MR or GitHub PR URL
          </p>
        )}
      </div>

      {/* Provider */}
      <div className="space-y-3">
        <Label className="text-sm text-muted-foreground">AI Provider</Label>
        <RadioGroup
          value={provider}
          onValueChange={setProvider}
          className="flex gap-4"
        >
          {["anthropic", "gemini", "ollama"].map((p) => (
            <div key={p} className="flex items-center gap-2">
              <RadioGroupItem value={p} id={`provider-${p}`} />
              <Label
                htmlFor={`provider-${p}`}
                className="text-sm capitalize cursor-pointer"
              >
                {p}
              </Label>
            </div>
          ))}
        </RadioGroup>
      </div>

      {/* Model */}
      <div className="space-y-2">
        <Label htmlFor="model" className="text-sm text-muted-foreground">
          Model
        </Label>
        <Input
          id="model"
          value={model}
          onChange={(e) => setModel(e.target.value)}
          placeholder="Leave blank for provider default"
          className="font-mono text-sm bg-surface border-border"
        />
      </div>

      {/* Focus Areas */}
      <div className="space-y-3">
        <Label className="text-sm text-muted-foreground">Focus Areas</Label>
        <div className="flex flex-wrap gap-2">
          {FOCUS_OPTIONS.map((area) => (
            <Badge
              key={area}
              role="checkbox"
              aria-checked={focusAreas.includes(area)}
              tabIndex={0}
              variant={focusAreas.includes(area) ? "default" : "outline"}
              className={cn(
                "cursor-pointer select-none transition-colors px-3 py-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                focusAreas.includes(area)
                  ? "bg-primary text-primary-foreground hover:bg-primary/80"
                  : "hover:bg-muted"
              )}
              onClick={() => toggleFocus(area)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  toggleFocus(area);
                }
              }}
            >
              {area}
            </Badge>
          ))}
        </div>
      </div>

      {/* Max Comments */}
      <div className="space-y-2">
        <Label htmlFor="max-comments" className="text-sm text-muted-foreground">
          Max Comments
        </Label>
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            aria-label="Decrease max comments"
            className="h-8 w-8 p-0"
            onClick={() => setMaxComments(Math.max(1, maxComments - 1))}
          >
            -
          </Button>
          <Input
            id="max-comments"
            type="number"
            min={1}
            max={50}
            value={maxComments}
            onChange={(e) => setMaxComments(Math.min(50, Math.max(1, Number(e.target.value) || 1)))}
            className="w-20 text-center font-mono text-sm bg-surface border-border"
          />
          <Button
            variant="outline"
            size="sm"
            aria-label="Increase max comments"
            className="h-8 w-8 p-0"
            onClick={() => setMaxComments(Math.min(50, maxComments + 1))}
          >
            +
          </Button>
        </div>
      </div>

      {/* Toggles */}
      <div className="space-y-4 rounded-lg border border-border bg-surface p-4">
        {/* Review Mode */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Label className="text-sm">Review Mode</Label>
            {autoPost && (
              <span className="inline-flex items-center gap-1 rounded bg-severity-warning/10 px-2 py-0.5 text-xs text-severity-warning border border-severity-warning/20">
                <AlertTriangle className="h-3 w-3" />
                Auto-post enabled
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">Manual</span>
            <Switch checked={autoPost} onCheckedChange={setAutoPost} />
            <span className="text-xs text-muted-foreground">Auto-post</span>
          </div>
        </div>

        {/* Parallel */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Zap className="h-4 w-4 text-muted-foreground" />
            <Label className="text-sm">Parallel Mode</Label>
          </div>
          <Switch checked={parallel} onCheckedChange={setParallel} />
        </div>
      </div>

      {/* Error display */}
      {error && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Submit */}
      <Button
        onClick={handleSubmit}
        disabled={isSubmitting}
        className="w-full h-11 text-sm font-medium"
        size="lg"
      >
        {isSubmitting ? (
          <>
            <Loader2 className="h-4 w-4 mr-2 animate-spin" />
            Submitting...
          </>
        ) : (
          <>
            <Play className="h-4 w-4 mr-2" />
            Run Review
          </>
        )}
      </Button>
    </div>
  );
}
