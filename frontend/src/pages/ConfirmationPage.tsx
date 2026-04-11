import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ExternalLink, RotateCcw, Loader2 } from "lucide-react";
import { useReview } from "@/context/ReviewContext";
import { getReviewResults } from "@/lib/api";

export function ConfirmationPage() {
  const navigate = useNavigate();
  const { jobId } = useParams<{ jobId: string }>();
  const { metadata, comments, postedCount, setReview, setPostedCount } = useReview();
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (metadata !== null || !jobId) return;
    let cancelled = false;
    setLoading(true);
    getReviewResults(jobId)
      .then((results) => {
        if (cancelled) return;
        setReview(results.job_id, results.summary, results.comments, results.metadata);
        setPostedCount(results.comments.filter((c) => c.approved).length);
      })
      .catch(() => { if (!cancelled) navigate("/", { replace: true }); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [jobId, metadata, navigate, setReview, setPostedCount]);

  if (loading || metadata === null) {
    return (
      <div className="flex items-center justify-center pt-24">
        <Loader2 className="h-6 w-6 text-primary animate-spin" />
      </div>
    );
  }

  const totalComments = comments.length;
  const rejectedCount = totalComments - postedCount;
  const webUrl = metadata.web_url;
  const platformLabel = webUrl.includes("gitlab") ? "View on GitLab" : "View on GitHub";

  return (
    <div className="max-w-sm mx-auto pt-16 space-y-10 page-transition">
      {/* Success indicator */}
      <div className="text-center space-y-4">
        <div className="inline-flex items-center justify-center w-14 h-14 rounded-full border border-success/20 bg-success/8">
          <svg className="w-6 h-6 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
          </svg>
        </div>
        <div>
          <h1 className="text-lg font-heading font-700 text-foreground">Review posted</h1>
          <p className="text-sm text-muted-foreground mt-1">Comments have been submitted to the MR.</p>
        </div>
      </div>

      {/* Stats */}
      <div className="flex divide-x divide-border border border-border rounded overflow-hidden">
        <div className="flex-1 text-center py-4 px-3 bg-surface">
          <div className="text-2xl font-mono font-500 text-foreground tabular-nums">{postedCount}</div>
          <div className="text-xs text-muted-foreground mt-0.5">posted</div>
        </div>
        <div className="flex-1 text-center py-4 px-3 bg-surface">
          <div className="text-2xl font-mono font-500 text-muted-foreground tabular-nums">{rejectedCount}</div>
          <div className="text-xs text-muted-foreground mt-0.5">rejected</div>
        </div>
      </div>

      {/* Link to MR */}
      {webUrl && (
        <div className="text-center">
          <a
            href={webUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 text-sm text-primary hover:text-primary/80 transition-colors"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            {platformLabel}
          </a>
        </div>
      )}

      {/* Review another */}
      <div className="text-center">
        <button
          onClick={() => navigate("/")}
          className="flex items-center gap-2 mx-auto px-4 py-2 text-sm rounded border border-border bg-surface hover:border-muted-foreground/30 text-foreground transition-colors"
        >
          <RotateCcw className="h-3.5 w-3.5" />
          Review Another
        </button>
      </div>
    </div>
  );
}
