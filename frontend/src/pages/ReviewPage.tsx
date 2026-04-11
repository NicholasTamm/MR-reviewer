import { useState, useEffect, useRef, useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "sonner";
import { Send, Loader2, AlertCircle, RotateCcw, CheckCircle2 } from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import { CommentCard } from "@/components/CommentCard";
import { SkeletonCommentCard } from "@/components/SkeletonCommentCard";
import { ShortcutHelp } from "@/components/ShortcutHelp";
import { StepTimeline } from "@/components/StepTimeline";
import { useReview } from "@/context/ReviewContext";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import {
  getJobStatus,
  getReviewResults,
  editComment as editCommentApi,
  postReview,
  ApiError,
} from "@/lib/api";
import type { JobStatus } from "@/types/api";
import { cn } from "@/lib/utils";

const POLL_INTERVAL_MS = 2000;
const POLL_TIMEOUT_MS = 5 * 60 * 1000;

type PagePhase = "polling" | "review" | "error" | "not-found" | "timeout" | "posting";

function getErrorTitle(errorType: string | null): string {
  switch (errorType) {
    case "platform_auth":
    case "provider_auth":
      return "Authentication Failed";
    case "config":
      return "Configuration Error";
    case "invalid_url":
      return "Invalid URL";
    case "platform":
      return "Platform Error";
    case "provider":
      return "AI Provider Error";
    default:
      return "Review Failed";
  }
}

export function ReviewPage() {
  const navigate = useNavigate();
  const { jobId } = useParams<{ jobId: string }>();
  const {
    summary,
    comments,
    metadata,
    setReview,
    setSummary,
    toggleApproval,
    updateCommentBody,
    setPostedCount,
  } = useReview();

  const [phase, setPhase] = useState<PagePhase>("polling");
  const [progressMessage, setProgressMessage] = useState("Starting review...");
  const [errorMessage, setErrorMessage] = useState("");
  const [isPosting, setIsPosting] = useState(false);
  const [focusedIndex, setFocusedIndex] = useState(-1);
  const [editingIndex, setEditingIndex] = useState(-1);
  const [currentStatus, setCurrentStatus] = useState<JobStatus["status"]>("pending");
  const [jobError, setJobError] = useState<string | null>(null);
  const [errorType, setErrorType] = useState<string | null>(null);

  const pollStartRef = useRef<number>(Date.now());
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const stoppedRef = useRef(false);
  const commentRefs = useRef<(HTMLDivElement | null)[]>([]);
  const lastNetworkErrorToastRef = useRef<number>(0);

  const stopPolling = useCallback(() => {
    stoppedRef.current = true;
    if (intervalRef.current !== null) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  }, []);

  const createPollFn = useCallback(
    (jobId: string) => async () => {
      if (stoppedRef.current) return;
      if (Date.now() - pollStartRef.current > POLL_TIMEOUT_MS) {
        stopPolling();
        setPhase("timeout");
        return;
      }
      try {
        const status = await getJobStatus(jobId);
        if (stoppedRef.current) return;
        setCurrentStatus(status.status);
        setJobError(status.error ?? null);
        setProgressMessage(status.progress ?? "Processing...");
        if (status.status === "complete" || status.status === "posted") {
          stopPolling();
          try {
            const results = await getReviewResults(jobId);
            setReview(results.job_id, results.summary, results.comments, results.metadata);
            if (status.status === "posted") {
              const postedCount = results.comments.filter((c) => c.approved).length;
              setPostedCount(postedCount);
              navigate(`/confirm/${jobId}`, { replace: true });
            } else {
              setPhase("review");
            }
          } catch (err) {
            toast.error("Failed to load review results", {
              description: err instanceof Error ? err.message : undefined,
              duration: 8000,
            });
            setErrorMessage(err instanceof Error ? err.message : "Failed to load review results");
            setPhase("error");
          }
          return;
        }
        if (status.status === "failed") {
          stopPolling();
          const msg = status.error ?? "Review failed";
          setCurrentStatus("failed");
          setJobError(msg);
          setErrorMessage(msg);
          setErrorType(status.error_type);
          toast.error(getErrorTitle(status.error_type), {
            description: status.error ?? undefined,
            duration: 8000,
          });
          setPhase("error");
          return;
        }
      } catch (err) {
        if (stoppedRef.current) return;
        if (err instanceof ApiError && err.status === 404) {
          stopPolling();
          setPhase("not-found");
          return;
        }
        const now = Date.now();
        if (now - lastNetworkErrorToastRef.current > 10_000) {
          lastNetworkErrorToastRef.current = now;
          toast.warning("Connection issue", { description: "Retrying...", duration: 4000 });
        }
        setProgressMessage("Connection issue, retrying...");
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [stopPolling, navigate, setReview, setPostedCount],
  );

  useEffect(() => {
    if (focusedIndex >= 0 && commentRefs.current[focusedIndex]) {
      commentRefs.current[focusedIndex]?.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  }, [focusedIndex]);

  useEffect(() => {
    if (editingIndex >= 0 && commentRefs.current[editingIndex]) {
      setTimeout(() => {
        commentRefs.current[editingIndex]?.scrollIntoView({ behavior: "smooth", block: "nearest" });
      }, 50);
    }
  }, [editingIndex]);

  useKeyboardShortcuts({
    commentCount: comments.length,
    focusedIndex,
    setFocusedIndex,
    onApprove: (index) => {
      const comment = comments[index];
      if (comment && !comment.approved) toggleApproval(comment.id);
    },
    onReject: (index) => {
      const comment = comments[index];
      if (comment && comment.approved) toggleApproval(comment.id);
    },
    onEdit: (index) => setEditingIndex(index),
    onEscape: () => setEditingIndex(-1),
    onPost: () => {
      const hasContent = comments.filter((c) => c.approved).length > 0 || summary.trim().length > 0;
      if (phase === "review" && !isPosting && hasContent) handlePost();
    },
    enabled: phase === "review",
  });

  useEffect(() => {
    if (!jobId) { setPhase("not-found"); return; }
    if (phase === "review" && comments.length > 0) return;
    stoppedRef.current = false;
    pollStartRef.current = Date.now();
    const poll = createPollFn(jobId);
    poll();
    intervalRef.current = setInterval(poll, POLL_INTERVAL_MS);
    return () => { stopPolling(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [jobId]);

  const handleContinueWaiting = () => {
    if (!jobId) return;
    pollStartRef.current = Date.now();
    stoppedRef.current = false;
    setPhase("polling");
    const poll = createPollFn(jobId);
    poll();
    intervalRef.current = setInterval(poll, POLL_INTERVAL_MS);
  };

  const handleEditBody = (commentId: string, newBody: string) => {
    updateCommentBody(commentId, newBody);
    setEditingIndex(-1);
    if (jobId) {
      editCommentApi(jobId, commentId, newBody).catch((err) => {
        toast.error("Comment edit not saved", {
          description: err instanceof Error ? err.message : undefined,
          duration: 5000,
        });
      });
    }
  };

  const handlePost = async () => {
    if (!jobId) return;
    const approvedComments = comments.filter((c) => c.approved);
    if (approvedComments.length === 0 && summary.trim().length === 0) return;
    setIsPosting(true);
    try {
      await postReview(jobId, {
        comment_ids: approvedComments.map((c) => c.id),
        summary,
      });
      setPostedCount(approvedComments.length);
      navigate(`/confirm/${jobId}`);
    } catch (err) {
      toast.error("Failed to post review", {
        description: err instanceof Error ? err.message : undefined,
        duration: 8000,
      });
    } finally {
      setIsPosting(false);
    }
  };

  const approvedCount = comments.filter((c) => c.approved).length;
  const rejectedCount = comments.filter((c) => !c.approved).length;

  // --- Polling ---
  if (phase === "polling") {
    return (
      <div className="page-transition relative min-h-[600px] flex items-center justify-center">
        <div
          className="absolute inset-0 z-0 pointer-events-none overflow-hidden space-y-4 opacity-30 select-none"
          aria-hidden="true"
          style={{
            WebkitMaskImage: "linear-gradient(to bottom, black 15%, transparent 80%)",
            maskImage: "linear-gradient(to bottom, black 15%, transparent 80%)"
          }}
        >
          <SkeletonCommentCard />
          <SkeletonCommentCard />
          <SkeletonCommentCard />
        </div>
        <div className="relative z-10 w-full max-w-sm bg-background/80 backdrop-blur-md p-8 rounded border border-border shadow-lg">
          <StepTimeline status={currentStatus} progress={progressMessage} error={jobError} />
        </div>
      </div>
    );
  }

  // --- Timeout ---
  if (phase === "timeout") {
    return (
      <div className="page-transition flex flex-col items-center justify-center pt-24 space-y-6 text-center">
        <AlertCircle className="h-8 w-8 text-severity-warning" />
        <div className="space-y-1.5">
          <h2 className="text-base font-heading font-700">Review is taking a while</h2>
          <p className="text-sm text-muted-foreground">Running for over 5 minutes.</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={handleContinueWaiting}
            className="px-4 py-2 text-sm rounded border border-border bg-surface hover:border-muted-foreground/30 text-foreground transition-colors"
          >
            Keep Waiting
          </button>
          <button
            onClick={() => navigate("/")}
            className="px-4 py-2 text-sm rounded border border-border bg-surface hover:border-muted-foreground/30 text-foreground transition-colors"
          >
            Start Over
          </button>
        </div>
      </div>
    );
  }

  // --- Not found ---
  if (phase === "not-found") {
    return (
      <div className="page-transition flex flex-col items-center justify-center pt-24 space-y-6 text-center">
        <AlertCircle className="h-8 w-8 text-destructive" />
        <div className="space-y-1.5">
          <h2 className="text-base font-heading font-700">Review not found</h2>
          <p className="text-sm text-muted-foreground">This job does not exist or has expired.</p>
        </div>
        <button
          onClick={() => navigate("/")}
          className="px-4 py-2 text-sm rounded border border-border bg-surface hover:border-muted-foreground/30 text-foreground transition-colors"
        >
          Back to Configure
        </button>
      </div>
    );
  }

  // --- Error ---
  if (phase === "error") {
    return (
      <div className="page-transition flex flex-col items-center justify-center pt-24 space-y-6 text-center">
        <AlertCircle className="h-8 w-8 text-destructive" />
        <div className="space-y-1.5">
          <h2 className="text-base font-heading font-700">{getErrorTitle(errorType)}</h2>
          <p className="text-sm text-muted-foreground max-w-sm">{errorMessage}</p>
        </div>
        <button
          onClick={() => navigate("/")}
          className="flex items-center gap-2 px-4 py-2 text-sm rounded border border-border bg-surface hover:border-muted-foreground/30 text-foreground transition-colors"
        >
          <RotateCcw className="h-3.5 w-3.5" />
          Try Again
        </button>
      </div>
    );
  }

  // --- Review ---
  return (
    <div className="page-transition space-y-6">
      {/* MR Info */}
      {metadata && (
        <div className="pb-2 border-b border-border">
          <h1 className="text-base font-heading font-700 text-foreground leading-tight">
            {metadata.title}
          </h1>
          <div className="flex items-center gap-2 mt-1.5">
            <span className="font-mono text-[11px] text-muted-foreground">{metadata.source_branch}</span>
            <span className="text-muted-foreground/30 text-xs">→</span>
            <span className="font-mono text-[11px] text-muted-foreground">{metadata.target_branch}</span>
          </div>
        </div>
      )}

      {/* Summary */}
      <div className="space-y-2">
        <label htmlFor="review-summary" className="text-xs font-mono uppercase tracking-widest text-muted-foreground">
          Summary
        </label>
        <Textarea
          id="review-summary"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          className="min-h-[90px] text-sm bg-surface border-border resize-y leading-relaxed focus-visible:ring-primary"
        />
      </div>

      {/* Empty state */}
      {comments.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 space-y-4">
          <div className="w-12 h-12 rounded-full bg-success/10 flex items-center justify-center">
            <CheckCircle2 className="h-6 w-6 text-success" />
          </div>
          <div className="text-center space-y-1">
            <h3 className="text-sm font-heading font-700">No issues found</h3>
            <p className="text-xs text-muted-foreground">The AI didn't find anything worth commenting on.</p>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {comments.map((comment, index) => (
            <div
              key={comment.id}
              ref={(el) => { commentRefs.current[index] = el; }}
              className="card-reveal"
              style={{ animationDelay: `${index * 35}ms` }}
            >
              <CommentCard
                comment={comment}
                onToggleApproval={toggleApproval}
                onEditBody={handleEditBody}
                isFocused={focusedIndex === index}
                isEditing={editingIndex === index}
                onStartEdit={() => setEditingIndex(index)}
                onCancelEdit={() => setEditingIndex(-1)}
              />
            </div>
          ))}
        </div>
      )}

      {/* Sticky bottom bar */}
      <div className="sticky bottom-0 -mx-6 border-t border-border bg-background/95 backdrop-blur-sm px-6 py-3">
        <div className="flex items-center justify-between max-w-5xl mx-auto">
          <div className="flex items-center gap-4 text-xs font-mono text-muted-foreground">
            <span className="text-foreground">{comments.length} comments</span>
            <span className={cn("tabular-nums", approvedCount > 0 && "text-success")}>
              {approvedCount} approved
            </span>
            <span className={cn("tabular-nums", rejectedCount > 0 && "text-destructive")}>
              {rejectedCount} rejected
            </span>
          </div>
          <button
            onClick={handlePost}
            disabled={(approvedCount === 0 && summary.trim().length === 0) || isPosting}
            className={cn(
              "flex items-center gap-2 px-4 py-2 rounded text-sm font-heading font-600 transition-all",
              "bg-primary text-primary-foreground hover:bg-primary/90 active:scale-[0.99]",
              "disabled:opacity-30 disabled:cursor-not-allowed"
            )}
          >
            {isPosting ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                Posting…
              </>
            ) : (
              <>
                <Send className="h-3.5 w-3.5" />
                {approvedCount > 0
                  ? `Post ${approvedCount} Comment${approvedCount !== 1 ? "s" : ""}`
                  : "Post Summary"}
              </>
            )}
          </button>
        </div>
      </div>

      <ShortcutHelp />
    </div>
  );
}
