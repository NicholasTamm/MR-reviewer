import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { getGitHubProjects, getGitLabProjects } from "@/lib/api";

export function BrowseProjectsPage() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const platform = params.get("platform") === "github" ? "github" : "gitlab";
  const [search, setSearch] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [projects, setProjects] = useState<{ id: string; path: string }[]>([]);

  useEffect(() => {
    const timer = setTimeout(() => {
      setLoading(true);
      const load = platform === "github"
        ? getGitHubProjects(search).then((catalog) => catalog.projects.map((project) => ({ id: project.id || project.path, path: project.path })))
        : getGitLabProjects(search).then((catalog) => catalog.projects.map((project) => ({ id: String(project.project_id), path: project.project_path })));
      load.then(setProjects).catch((err: Error) => setError(err.message)).finally(() => setLoading(false));
    }, 200);
    return () => clearTimeout(timer);
  }, [platform, search]);

  return (
    <div className="max-w-2xl space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-medium">Browse {platform === "github" ? "GitHub" : "GitLab"}</h1>
          <p className="text-sm text-muted-foreground">Select a project before loading reviews.</p>
        </div>
        <button
          className="rounded-md border border-border px-3 py-2 text-sm"
          onClick={() => setParams({ platform: platform === "github" ? "gitlab" : "github" })}
        >
          Switch to {platform === "github" ? "GitLab" : "GitHub"}
        </button>
      </div>
      <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search projects" className="h-10 w-full rounded-md border border-border bg-surface px-3" />
      {error && <p className="text-sm text-destructive">{error}</p>}
      {loading && <p className="text-sm text-muted-foreground">Loading projects...</p>}
      {projects.map((project) => (
        <button key={project.id} onClick={() => navigate(`/browse/${platform}/${project.id}`)} className="block w-full rounded-md border border-border bg-surface p-4 text-left hover:bg-muted">
          <div className="font-mono text-sm">{project.path}</div>
        </button>
      ))}
      {!loading && !error && projects.length === 0 && <p className="text-sm text-muted-foreground">No accessible projects.</p>}
      <Link to="/" className="inline-block text-sm text-muted-foreground hover:text-foreground">Or paste a review URL</Link>
    </div>
  );
}
