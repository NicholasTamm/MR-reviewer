import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getGitLabProjects } from "@/lib/api";
import type { GitLabProjectSummary } from "@/types/api";

export function GitLabProjectsPage() {
  const [projects, setProjects] = useState<GitLabProjectSummary[]>([]);
  const [search, setSearch] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      setLoading(true);
      getGitLabProjects(search).then((catalog) => {
        setProjects(catalog.projects);
        setError("");
      }).catch((requestError: Error) => setError(requestError.message)).finally(() => setLoading(false));
    }, 200);
    return () => clearTimeout(timer);
  }, [search]);

  return <div className="max-w-2xl space-y-4">
    <div><h1 className="text-lg font-medium">GitLab Merge Requests</h1><p className="text-sm text-muted-foreground">Open merge requests visible to your token.</p></div>
    <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search projects and merge requests" className="h-10 w-full rounded-md border border-border bg-surface px-3" />
    {error && <p className="text-sm text-destructive">{error}</p>}
    {loading && <p className="text-sm text-muted-foreground">Loading visible projects...</p>}
    {projects.map((project) => <Link key={project.project_id} to={`/gitlab/projects/${project.project_id}`} className="block rounded-md border border-border bg-surface p-4 hover:bg-muted"><div className="font-mono text-sm">{project.project_path}</div><div className="text-sm text-muted-foreground">View open merge requests</div></Link>)}
    {!loading && !error && projects.length === 0 && <p className="text-sm text-muted-foreground">No visible projects found.</p>}
  </div>;
}
