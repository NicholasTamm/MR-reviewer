import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  ChevronDown,
  ChevronRight,
  GitMerge,
  GitPullRequest,
  Link2,
  Loader2,
  Play,
  Search,
  type LucideIcon,
} from "lucide-react";
import { ReviewConfigFields } from "@/components/ReviewConfigFields";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useReview } from "@/context/ReviewContext";
import {
  configSummary,
  defaultsFromServer,
  loadLocalDefaults,
  resolveDefaults,
  type ReviewDefaults,
} from "@/lib/defaults";
import { pickModel } from "@/lib/providers";
import {
  ApiError,
  getConfigDefaults,
  getGitHubProjectPRs,
  getGitHubProjects,
  getGitLabProjectMergeRequests,
  getGitLabProjects,
  getProviderModels,
  submitReview,
} from "@/lib/api";
import { cn } from "@/lib/utils";
import type { ProviderModels } from "@/types/api";

type Platform = "gitlab" | "github";
type Stage = "platform" | "project" | "review" | "ready";

type ProjectItem = { id: string; path: string };
type ReviewItem = {
  id: string;
  title: string;
  url: string;
  draft: boolean;
  author?: string;
  source?: string;
  target?: string;
};

const MR_URL_PATTERN = /^https?:\/\/.+\/(merge_requests|pull)\/\d+/;

export function ConfigurePage() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { reset } = useReview();

  const queryPlatform = searchParams.get("platform") === "github" ? "github" : searchParams.get("platform") === "gitlab" ? "gitlab" : null;
  const queryProject = searchParams.get("project") ?? "";
  const queryUrl = searchParams.get("url") ?? "";

  const [platform, setPlatform] = useState<Platform | null>(queryPlatform);
  const [project, setProject] = useState<ProjectItem | null>(queryProject ? { id: queryProject, path: queryProject } : null);
  const [selected, setSelected] = useState<ReviewItem | null>(
    queryUrl ? { id: "", title: queryUrl, url: queryUrl, draft: false } : null,
  );
  const [pasteOpen, setPasteOpen] = useState(Boolean(queryUrl));
  const [url, setUrl] = useState(queryUrl);
  const [urlTouched, setUrlTouched] = useState(false);
  const [configOpen, setConfigOpen] = useState(false);
  const [config, setConfig] = useState<ReviewDefaults>(defaultsFromServer(null));
  const [defaultsReady, setDefaultsReady] = useState(false);

  const [projectSearch, setProjectSearch] = useState("");
  const [reviewSearch, setReviewSearch] = useState("");
  const [projects, setProjects] = useState<ProjectItem[]>([]);
  const [reviews, setReviews] = useState<ReviewItem[]>([]);
  const [projectsLoading, setProjectsLoading] = useState(false);
  const [reviewsLoading, setReviewsLoading] = useState(false);
  const [projectsError, setProjectsError] = useState("");
  const [reviewsError, setReviewsError] = useState("");

  const [providerModels, setProviderModels] = useState<ProviderModels[]>([]);
  const [modelsError, setModelsError] = useState<string | null>(null);
  const [modelsLoading, setModelsLoading] = useState(true);
  const [modelRequest, setModelRequest] = useState(0);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const stage: Stage = selected ? "ready" : project ? "review" : platform ? "project" : "platform";

  useEffect(() => {
    reset();
  }, [reset]);

  useEffect(() => {
    if (queryUrl) {
      setUrl(queryUrl);
      setSelected({ id: "", title: queryUrl, url: queryUrl, draft: false });
      setPasteOpen(true);
    }
  }, [queryUrl]);

  useEffect(() => {
    let cancelled = false;
    Promise.all([getConfigDefaults().catch(() => null), Promise.resolve(loadLocalDefaults())])
      .then(([server, local]) => {
        if (cancelled) return;
        setConfig(resolveDefaults(server, local));
        setDefaultsReady(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    setModelsLoading(true);
    setModelsError(null);
    getProviderModels()
      .then((catalog) => {
        if (!cancelled) setProviderModels(catalog.providers);
      })
      .catch(() => {
        if (!cancelled) setModelsError("Unable to load available models. Check provider configuration and try again.");
      })
      .finally(() => {
        if (!cancelled) setModelsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [modelRequest]);

  useEffect(() => {
    if (stage === "ready" && defaultsReady && !config.model) setConfigOpen(true);
  }, [stage, defaultsReady, config.model]);

  useEffect(() => {
    if (!defaultsReady || modelsLoading) return;
    const active = providerModels.find((item) => item.provider === config.provider);
    if (!active?.available || active.models.length === 0) return;
    if (config.model && active.models.includes(config.model)) return;
    setConfig((current) => ({
      ...current,
      model: pickModel(current.provider, active.models, current.model),
    }));
  }, [defaultsReady, modelsLoading, providerModels, config.provider, config.model]);

  useEffect(() => {
    if (!platform || selected) return;
    const timer = setTimeout(() => {
      setProjectsLoading(true);
      setProjectsError("");
      const load = platform === "github"
        ? getGitHubProjects(projectSearch).then((catalog) =>
            catalog.projects.map((item) => ({ id: item.id || item.path, path: item.path })),
          )
        : getGitLabProjects(projectSearch).then((catalog) =>
            catalog.projects.map((item) => ({ id: String(item.project_id), path: item.project_path })),
          );
      load
        .then((items) => {
          setProjects(items);
          if (queryProject) {
            const match = items.find((item) => item.id === queryProject || item.path === queryProject);
            if (match) setProject(match);
          }
        })
        .catch((err: Error) => setProjectsError(err.message))
        .finally(() => setProjectsLoading(false));
    }, 200);
    return () => clearTimeout(timer);
  }, [platform, projectSearch, selected, queryProject]);

  const projectId = project?.id ?? "";

  useEffect(() => {
    if (!platform || !projectId) return;
    setReviewsLoading(true);
    setReviewsError("");
    const load = platform === "github"
      ? loadGitHubReviews({ id: projectId, path: projectId }).then((items) => {
          const [owner, repo] = splitGitHubPath(projectId);
          if (repo) {
            setProject((current) => current ? { ...current, path: `${owner}/${repo}` } : current);
          }
          return items;
        })
      : getGitLabProjectMergeRequests(projectId).then((catalog) => {
          setProject((current) => current ? { ...current, path: catalog.project_path || current.path } : current);
          return catalog.merge_requests.map((mr) => ({
            id: String(mr.iid),
            title: mr.title,
            url: mr.web_url,
            draft: mr.draft,
            author: mr.author,
            source: mr.source_branch,
            target: mr.target_branch,
          }));
        });
    load
      .then(setReviews)
      .catch((err: Error) => setReviewsError(err.message))
      .finally(() => setReviewsLoading(false));
  }, [platform, projectId]);

  const activeUrl = selected?.url || url;
  const isUrlValid = MR_URL_PATTERN.test(activeUrl);
  const showUrlError = urlTouched && url.length > 0 && !isUrlValid;
  const visibleReviews = useMemo(() => {
    const q = reviewSearch.trim().toLowerCase();
    if (!q) return reviews;
    return reviews.filter((item) =>
      `${item.title} ${item.id} ${item.author ?? ""} ${item.source ?? ""}`.toLowerCase().includes(q),
    );
  }, [reviews, reviewSearch]);

  const handleConfigChange = (next: ReviewDefaults) => {
    setConfig(next);
    setError(null);
  };

  const commitPastedUrl = (value = url) => {
    setUrlTouched(true);
    if (!MR_URL_PATTERN.test(value)) return;
    setSelected({ id: "", title: value, url: value, draft: false });
    setUrl(value);
  };

  const selectPlatform = (next: Platform) => {
    setPlatform(next);
    setProject(null);
    setSelected(null);
    setReviews([]);
    setProjectSearch("");
    setReviewSearch("");
    setUrl("");
    setPasteOpen(false);
    setError(null);
  };

  const selectProject = (next: ProjectItem) => {
    setProject(next);
    setSelected(null);
    setReviewSearch("");
    setError(null);
  };

  const selectReview = (next: ReviewItem) => {
    setSelected(next);
    setUrl(next.url);
    setError(null);
  };

  const handleSubmit = async () => {
    if (!isUrlValid) {
      setUrlTouched(true);
      setPasteOpen(true);
      return;
    }
    if (!config.model) {
      setConfigOpen(true);
      setError("Select a model before running the review.");
      return;
    }
    setIsSubmitting(true);
    setError(null);
    try {
      const result = await submitReview({
        url: activeUrl,
        provider: config.provider,
        model: config.model,
        focus: config.focus,
        max_comments: config.maxComments,
        parallel: config.parallel,
        auto_post: config.autoPost,
      });
      navigate(`/review/${result.job_id}`);
    } catch (err) {
      if (err instanceof ApiError || err instanceof Error) setError(err.message);
      else setError("An unexpected error occurred");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="mx-auto max-w-3xl space-y-6 page-transition">
      <header className="space-y-3">
        <p className="text-[11px] font-mono uppercase tracking-[0.22em] text-muted-foreground">
          {platform ? (platform === "github" ? "GitHub" : "GitLab") : "Choose a platform"}
        </p>
        <Trail
          platform={platform}
          project={project}
          selected={selected}
          onRoot={() => {
            setPlatform(null);
            setProject(null);
            setSelected(null);
            setReviews([]);
            setProjectSearch("");
            setReviewSearch("");
            setUrl("");
            setPasteOpen(false);
            setError(null);
            setSearchParams({}, { replace: true });
          }}
          onPlatform={() => {
            setProject(null);
            setSelected(null);
            setReviews([]);
            setReviewSearch("");
            setUrl("");
            setPasteOpen(false);
            setError(null);
            setSearchParams(platform ? { platform } : {}, { replace: true });
          }}
          onProject={() => {
            setSelected(null);
            setUrl("");
            setError(null);
            const next = new URLSearchParams();
            if (platform) next.set("platform", platform);
            if (project) next.set("project", project.id);
            setSearchParams(next, { replace: true });
          }}
        />
      </header>

      {stage === "platform" && (
        <section className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <PlatformCard
              icon={GitMerge}
              name="GitLab"
              detail="Browse merge requests"
              selected={platform === "gitlab"}
              onClick={() => selectPlatform("gitlab")}
            />
            <PlatformCard
              icon={GitPullRequest}
              name="GitHub"
              detail="Browse pull requests"
              selected={platform === "github"}
              onClick={() => selectPlatform("github")}
            />
          </div>
          <PasteUrl
            open={pasteOpen}
            url={url}
            error={showUrlError}
            onToggle={() => setPasteOpen((value) => !value)}
            onChange={(value) => {
              setUrl(value);
              setError(null);
            }}
            onCommit={commitPastedUrl}
          />
        </section>
      )}

      {stage === "project" && platform && (
        <section className="space-y-3">
          <SearchField
            value={projectSearch}
            onChange={setProjectSearch}
            placeholder="Search projects"
          />
          {projectsError && <p className="text-sm text-destructive">{projectsError}</p>}
          {projectsLoading && <StatusLine>Loading projects…</StatusLine>}
          <div className="overflow-hidden rounded-lg border border-border">
            {projects.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => selectProject(item)}
                className="flex w-full items-center justify-between border-b border-border/70 bg-surface px-4 py-3 text-left last:border-b-0 hover:bg-muted"
              >
                <span className="font-mono text-sm">{item.path}</span>
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              </button>
            ))}
          </div>
          {!projectsLoading && !projectsError && projects.length === 0 && (
            <StatusLine>No accessible projects.</StatusLine>
          )}
          <PasteUrl
            open={pasteOpen}
            url={url}
            error={showUrlError}
            onToggle={() => setPasteOpen((value) => !value)}
            onChange={(value) => {
              setUrl(value);
              setError(null);
            }}
            onCommit={commitPastedUrl}
          />
        </section>
      )}

      {stage === "review" && platform && project && (
        <section className="space-y-3">
          <SearchField
            value={reviewSearch}
            onChange={setReviewSearch}
            placeholder={platform === "github" ? "Search pull requests" : "Search merge requests"}
          />
          {reviewsError && <p className="text-sm text-destructive">{reviewsError}</p>}
          {reviewsLoading && <StatusLine>Loading open reviews…</StatusLine>}
          <div className="space-y-2">
            {visibleReviews.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => selectReview(item)}
                className="flex w-full items-start justify-between gap-4 rounded-lg border border-border bg-surface px-4 py-3 text-left hover:border-primary/40"
              >
                <div className="min-w-0 space-y-1">
                  <div className="flex items-center gap-2 font-mono text-[11px] text-muted-foreground">
                    <span>#{item.id}</span>
                    {item.draft && <span className="rounded bg-muted px-1.5 py-0.5">draft</span>}
                    {item.author && <span>{item.author}</span>}
                  </div>
                  <p className="truncate text-sm text-foreground">{item.title}</p>
                  {item.source && item.target && (
                    <p className="truncate font-mono text-[11px] text-muted-foreground">
                      {item.source} → {item.target}
                    </p>
                  )}
                </div>
                <ChevronRight className="mt-1 h-4 w-4 shrink-0 text-muted-foreground" />
              </button>
            ))}
          </div>
          {!reviewsLoading && !reviewsError && visibleReviews.length === 0 && (
            <StatusLine>No open reviews in this project.</StatusLine>
          )}
        </section>
      )}

      {stage === "ready" && (
        <section className="space-y-4">
          <div className="rounded-lg border border-border bg-surface px-4 py-3">
            <p className="font-mono text-[11px] text-muted-foreground">
              {selected?.id ? `#${selected.id}` : "URL"}
              {selected?.draft ? " · draft" : ""}
            </p>
            <h2 className="mt-1 text-sm font-medium">{selected?.title || activeUrl}</h2>
            {selected?.source && selected?.target && (
              <p className="mt-1 font-mono text-[11px] text-muted-foreground">
                {selected.source} → {selected.target}
              </p>
            )}
            <p className="mt-2 truncate font-mono text-[11px] text-muted-foreground">{activeUrl}</p>
          </div>

          <div className="overflow-hidden rounded-lg border border-border bg-surface">
            <button
              type="button"
              aria-expanded={configOpen}
              onClick={() => setConfigOpen((value) => !value)}
              className="flex w-full items-center justify-between gap-4 px-4 py-3 text-left hover:bg-muted/60"
            >
              <div className="min-w-0">
                <p className="text-sm">Configuration</p>
                <p className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                  {configSummary(config)}
                </p>
              </div>
              <ChevronDown className={cn("h-4 w-4 shrink-0 text-muted-foreground transition-transform", configOpen && "rotate-180")} />
            </button>
            {configOpen && (
              <div className="space-y-4 border-t border-border px-4 py-4">
                <ReviewConfigFields
                  value={config}
                  onChange={handleConfigChange}
                  providers={providerModels}
                  modelsLoading={modelsLoading}
                  modelsError={modelsError}
                  onRetryModels={() => setModelRequest((n) => n + 1)}
                />
                <p className="text-xs text-muted-foreground">
                  Defaults live in <Link to="/settings" className="text-primary hover:underline">Settings</Link>.
                </p>
              </div>
            )}
          </div>

          {error && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
              {error}
            </div>
          )}

          <Button
            onClick={handleSubmit}
            disabled={isSubmitting || !config.model}
            className="h-11 w-full text-sm font-medium"
            size="lg"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Submitting…
              </>
            ) : (
              <>
                <Play className="mr-2 h-4 w-4" />
                Run review
              </>
            )}
          </Button>
        </section>
      )}
    </div>
  );
}

function Trail({
  platform,
  project,
  selected,
  onRoot,
  onPlatform,
  onProject,
}: {
  platform: Platform | null;
  project: ProjectItem | null;
  selected: ReviewItem | null;
  onRoot: () => void;
  onPlatform: () => void;
  onProject: () => void;
}) {
  const platformLabel = platform === "github" ? "GitHub" : platform === "gitlab" ? "GitLab" : "PLATFORM";
  const projectLabel = project?.path || "PROJECT";
  const reviewLabel = selected?.id ? `#${selected.id}` : selected ? "URL" : "MR / PR";

  return (
    <nav aria-label="Selection" className="flex flex-wrap items-center gap-1 font-mono text-sm">
      <TrailCrumb onClick={onRoot}>MR-REVIEWER</TrailCrumb>
      <TrailSep />
      <TrailCrumb onClick={platform ? onPlatform : undefined}>{platformLabel}</TrailCrumb>
      <TrailSep />
      <TrailCrumb onClick={project ? onProject : undefined}>{projectLabel}</TrailCrumb>
      <TrailSep />
      {selected ? (
        <span className="text-foreground">{reviewLabel}</span>
      ) : (
        <span className="text-muted-foreground/50">{reviewLabel}</span>
      )}
    </nav>
  );
}

function TrailCrumb({ children, onClick }: { children: ReactNode; onClick?: () => void }) {
  if (!onClick) {
    return <span className="text-muted-foreground/50">{children}</span>;
  }
  return (
    <button type="button" onClick={onClick} className="text-muted-foreground hover:text-foreground">
      {children}
    </button>
  );
}

function TrailSep() {
  return <span className="text-muted-foreground/50">›</span>;
}

function PlatformCard({
  icon: Icon,
  name,
  detail,
  selected,
  onClick,
}: {
  icon: LucideIcon;
  name: string;
  detail: string;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-xl border bg-surface px-5 py-6 text-left transition-colors",
        selected ? "border-primary bg-primary/5" : "border-border hover:border-primary/40",
      )}
    >
      <Icon className="h-5 w-5 text-primary" />
      <p className="mt-4 text-base font-medium">{name}</p>
      <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
    </button>
  );
}

function SearchField({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <div className="relative">
      <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 w-full rounded-md border border-border bg-surface pl-9 pr-3 text-sm outline-none focus:border-primary"
      />
    </div>
  );
}

function PasteUrl({
  open,
  url,
  error,
  onToggle,
  onChange,
  onCommit,
}: {
  open: boolean;
  url: string;
  error: boolean;
  onToggle: () => void;
  onChange: (value: string) => void;
  onCommit: (value?: string) => void;
}) {
  return (
    <div className="space-y-2">
      <button
        type="button"
        onClick={onToggle}
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <Link2 className="h-3.5 w-3.5" />
        Or paste a review URL
      </button>
      {open && (
        <>
          <Input
            type="url"
            value={url}
            onChange={(event) => onChange(event.target.value)}
            onBlur={() => onCommit()}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                onCommit();
              }
            }}
            placeholder="https://github.com/owner/repo/pull/123"
            className={cn(
              "h-11 font-mono text-sm bg-surface border-border placeholder:text-muted-foreground/40",
              error && "border-destructive",
            )}
          />
          {error && <p className="text-xs text-destructive">Enter a valid GitLab MR or GitHub PR URL</p>}
        </>
      )}
    </div>
  );
}

function StatusLine({ children }: { children: ReactNode }) {
  return <p className="text-sm text-muted-foreground">{children}</p>;
}

async function loadGitHubReviews(project: ProjectItem): Promise<ReviewItem[]> {
  const [owner, repo] = splitGitHubPath(project.path || project.id);
  const catalog = await getGitHubProjectPRs(owner, repo);
  return (catalog.pull_requests || []).map((pr) => ({
    id: String(pr.number),
    title: pr.title,
    url: pr.web_url,
    draft: pr.draft,
    author: pr.author,
    source: pr.source_branch,
    target: pr.target_branch,
  }));
}

function splitGitHubPath(value: string): [string, string] {
  const [owner, repo] = value.split("/");
  return [owner || value, repo || ""];
}
