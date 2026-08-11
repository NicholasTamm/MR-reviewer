import { useEffect, useState } from "react";
import { ArrowLeft, GitPullRequest, Search, SlidersHorizontal } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getGitLabProjectMergeRequests } from "@/lib/api";
import type { GitLabProjectMergeRequests } from "@/types/api";

type Filter = "all" | "ready" | "draft";

export function GitLabProjectPage() {
  const { projectId } = useParams();
  const navigate = useNavigate();
  const [project, setProject] = useState<GitLabProjectMergeRequests | null>(null);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<Filter>("all");

  useEffect(() => {
    if (projectId) getGitLabProjectMergeRequests(projectId).then(setProject).catch((requestError: Error) => setError(requestError.message));
  }, [projectId]);

  if (!project) return <div className="rounded-lg border border-border bg-surface p-6 text-sm text-muted-foreground">{error || "Loading project merge requests..."}</div>;

  const visibleMrs = project.merge_requests.filter((mr) => {
    const matchesSearch = `${mr.title} ${mr.iid} ${mr.author} ${mr.source_branch}`.toLowerCase().includes(search.toLowerCase());
    return matchesSearch && (filter === "all" || (filter === "draft" ? mr.draft : !mr.draft));
  });

  return (
    <div className="max-w-4xl space-y-6">
      <div className="space-y-4 border-b border-border pb-5">
        <Link to="/gitlab/merge-requests" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"><ArrowLeft className="h-4 w-4" /> All projects</Link>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div><p className="text-xs font-mono uppercase tracking-widest text-muted-foreground">Project queue</p><h1 className="mt-1 font-mono text-2xl text-foreground">{project.project_path}</h1></div>
          <span className="rounded-full border border-border bg-surface px-3 py-1 text-xs text-muted-foreground">{project.merge_requests.length} open</span>
        </div>
      </div>

      <div className="flex flex-col gap-3 rounded-lg border border-border bg-surface p-3 sm:flex-row sm:items-center">
        <div className="relative flex-1"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search title, MR number, author, or branch" className="h-10 w-full rounded-md border border-border bg-background pl-9 pr-3 text-sm outline-none focus:border-primary" /></div>
        <div className="flex items-center gap-1" aria-label="Merge request filter"><SlidersHorizontal className="mr-1 h-4 w-4 text-muted-foreground" />{(["all", "ready", "draft"] as Filter[]).map((option) => <button key={option} onClick={() => setFilter(option)} className={`rounded-md px-3 py-2 text-xs capitalize transition-colors ${filter === option ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-muted hover:text-foreground"}`}>{option}</button>)}</div>
      </div>

      <div className="space-y-2">
        {visibleMrs.map((mr) => <article key={mr.iid} className="group rounded-lg border border-border bg-surface p-4 transition-colors hover:border-primary/40">
          <div className="flex flex-col justify-between gap-4 sm:flex-row">
            <div className="min-w-0 space-y-2"><div className="flex items-center gap-2 text-xs font-mono text-muted-foreground"><GitPullRequest className="h-4 w-4 text-primary" />!{mr.iid}{mr.draft && <span className="rounded bg-muted px-1.5 py-0.5">DRAFT</span>}<span>{mr.author || "Unknown author"}</span></div><h2 className="text-base font-medium leading-snug text-foreground">{mr.title}</h2><p className="truncate font-mono text-xs text-muted-foreground">{mr.source_branch} <span className="mx-1">to</span> {mr.target_branch}</p></div>
            <button onClick={() => navigate(`/?url=${encodeURIComponent(mr.web_url)}`)} className="h-9 shrink-0 rounded-md border border-primary/40 px-3 text-sm text-primary transition-colors hover:bg-primary hover:text-primary-foreground">Review MR</button>
          </div>
        </article>)}
        {visibleMrs.length === 0 && <div className="rounded-lg border border-dashed border-border py-12 text-center text-sm text-muted-foreground">No merge requests match these filters.</div>}
      </div>
    </div>
  );
}
