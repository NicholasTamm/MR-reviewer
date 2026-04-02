import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { CheckCircle2, ExternalLink, RotateCcw, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { useReview } from "@/context/ReviewContext";
import { getReviewResults } from "@/lib/api";

export function ConfirmationPage() {
  const navigate = useNavigate();
  const { jobId } = useParams<{ jobId: string }>();
  const { metadata, comments, postedCount, setReview, setPostedCount } = useReview();

  const [loading, setLoading] = useState(false);

  // If we don't have metadata (e.g. direct navigation), re-fetch from API
  useEffect(() => {
    if (metadata !== null || !jobId) return;

    let cancelled = false;
    setLoading(true);

    getReviewResults(jobId)
      .then((results) => {
        if (cancelled) return;
        setReview(
          results.job_id,
          results.summary,
          results.comments,
          results.metadata,
        );
        // If we're on the confirmation page, all approved comments were posted
        const approved = results.comments.filter((c) => c.approved).length;
        setPostedCount(approved);
      })
      .catch(() => {
        if (!cancelled) {
          // Can't load data; redirect to configure
          navigate("/", { replace: true });
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [jobId, metadata, navigate, setReview, setPostedCount]);

  if (loading || metadata === null) {
    return (
      <div className="flex flex-col items-center justify-center pt-24">
        <Loader2 className="h-8 w-8 text-primary animate-spin" />
      </div>
    );
  }

  const totalComments = comments.length;
  const rejectedCount = totalComments - postedCount;
  const webUrl = metadata.web_url;
  const platformLabel = webUrl.includes("gitlab")
    ? "View on GitLab"
    : "View on GitHub";

  return (
    <div className="max-w-lg mx-auto text-center space-y-8 pt-12 page-transition">
      {/* Success icon */}
      <div className="flex justify-center">
        <div className="rounded-full bg-success/10 p-4">
          <CheckCircle2 className="h-12 w-12 text-success" />
        </div>
      </div>

      {/* Message */}
      <div className="space-y-2">
        <h1 className="text-xl font-medium text-foreground">
          Review posted successfully
        </h1>
        <p className="text-sm text-muted-foreground">
          Your review has been posted to the merge request.
        </p>
      </div>

      <Separator />

      {/* Stats */}
      <div className="flex justify-center gap-8 text-sm">
        <div className="text-center">
          <div className="text-2xl font-mono font-medium text-foreground">
            {postedCount}
          </div>
          <div className="text-muted-foreground">comments posted</div>
        </div>
        <div className="text-center">
          <div className="text-2xl font-mono font-medium text-muted-foreground">
            {rejectedCount}
          </div>
          <div className="text-muted-foreground">rejected</div>
        </div>
      </div>

      {/* Link to MR */}
      {webUrl && (
        <a
          href={webUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 text-sm text-primary hover:text-primary/80 transition-colors"
        >
          <ExternalLink className="h-4 w-4" />
          {platformLabel}
        </a>
      )}

      <Separator />

      {/* Review Another */}
      <Button
        onClick={() => navigate("/")}
        variant="outline"
        size="lg"
        className="gap-2"
      >
        <RotateCcw className="h-4 w-4" />
        Review Another
      </Button>
    </div>
  );
}
