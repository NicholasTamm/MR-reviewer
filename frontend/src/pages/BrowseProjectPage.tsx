import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getGitHubProjectPRs, getGitLabProjectMergeRequests } from "@/lib/api";

export function BrowseProjectPage() {
  const { platform = "gitlab", projectId = "", repo = "" } = useParams();
  const navigate = useNavigate();
  const [title, setTitle] = useState(projectId);
  const [error, setError] = useState("");
  const [items, setItems] = useState<{ id: string; title: string; url: string; draft: boolean }[]>([]);

  useEffect(() => {
    if (platform === "github") {
      const owner = projectId;
      getGitHubProjectPRs(owner, repo).then((catalog) => {
        setTitle(catalog.path || `${owner}/${repo}`);
        setItems((catalog.pull_requests || []).map((pr) => ({ id: String(pr.number), title: pr.title, url: pr.web_url, draft: pr.draft })));
      }).catch((err: Error) => setError(err.message));
      return;
    }
    getGitLabProjectMergeRequests(projectId).then((catalog) => {
      setTitle(catalog.project_path);
      setItems(catalog.merge_requests.map((mr) => ({ id: String(mr.iid), title: mr.title, url: mr.web_url, draft: mr.draft })));
    }).catch((err: Error) => setError(err.message));
  }, [platform, projectId, repo]);

  return (
    <div className="max-w-3xl space-y-4">
      <Link to={`/browse?platform=${platform}`} className="text-sm text-muted-foreground hover:text-foreground">Back to projects</Link>
      <h1 className="font-mono text-xl">{title}</h1>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {items.map((item) => (
        <article key={item.id} className="flex items-center justify-between rounded-lg border border-border bg-surface p-4">
          <div>
            <p className="text-xs font-mono text-muted-foreground">#{item.id}{item.draft ? " draft" : ""}</p>
            <h2 className="text-base">{item.title}</h2>
          </div>
          <button onClick={() => navigate(`/?url=${encodeURIComponent(item.url)}`)} className="rounded-md border border-primary/40 px-3 py-2 text-sm text-primary">Review</button>
        </article>
      ))}
      {!error && items.length === 0 && <p className="text-sm text-muted-foreground">No open reviews in this project.</p>}
    </div>
  );
}
