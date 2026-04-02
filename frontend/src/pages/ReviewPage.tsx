import { useState, useEffect, useRef, useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Send, Loader2, AlertCircle, RotateCcw, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Separator } from "@/components/ui/separator";
import { CommentCard } from "@/components/CommentCard";
import { SkeletonCommentCard } from "@/components/SkeletonCommentCard";
import { ShortcutHelp } from "@/components/ShortcutHelp";
import { useReview } from "@/context/ReviewContext";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import {
  getJobStatus,
  getReviewResults,
  editComment as editCommentApi,
  postReview,
  ApiError,
} from "@/lib/api";

const POLL_INTERVAL_MS = 2000;
const POLL_TIMEOUT_MS = 5 * 60 * 1000; // 5 minutes

type PagePhase = "polling" | "review" | "error" | "not-found" | "timeout" | "posting";

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

  const pollStartRef = useRef<number>(Date.now());
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const stoppedRef = useRef(false);
  const commentRefs = useRef<(HTMLDivElement | null)[]>([]);

  const stopPolling = useCallback(() => {
    stoppedRef.current = true;
    if (intervalRef.current !== null) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  }, []);

  // Scroll focused comment into view
  useEffect(() => {
    if (focusedIndex >= 0 && commentRefs.current[focusedIndex]) {
      commentRefs.current[focusedIndex]?.scrollIntoView({
        behavior: "smooth",
        block: "nearest",
      });
    }
  }, [focusedIndex]);

  // Keyboard shortcuts
  useKeyboardShortcuts({
    commentCount: comments.length,
    focusedIndex,
    setFocusedIndex,
    onApprove: (index) => {
      const comment = comments[index];
      if (comment && !comment.approved) {
        toggleApproval(comment.id);
      }
    },
    onReject: (index) => {
      const comment = comments[index];
      if (comment && comment.approved) {
        toggleApproval(comment.id);
      }
    },
    onEdit: (index) => {
      setEditingIndex(index);
    },
    onEscape: () => {
      setEditingIndex(-1);
    },
    onPost: () => {
      if (phase === "review" && !isPosting && comments.filter((c) => c.approved).length > 0) {
        handlePost();
      }
    },
    enabled: phase === "review",
  });

  // Polling effect
  useEffect(() => {
    if (!jobId) {
      setPhase("not-found");
      return;
    }

    // If we already have review data for this job, skip polling
    if (phase === "review" && comments.length > 0) {
      return;
    }

    stoppedRef.current = false;
    pollStartRef.current = Date.now();

    const poll = async () => {
      if (stoppedRef.current) return;

      // Check timeout
      if (Date.now() - pollStartRef.current > POLL_TIMEOUT_MS) {
        stopPolling();
        setPhase("timeout");
        return;
      }

      try {
        const status = await getJobStatus(jobId);

        if (stoppedRef.current) return;

        setProgressMessage(status.progress ?? "Processing...");

        if (status.status === "complete" || status.status === "posted") {
          stopPolling();
          try {
            const results = await getReviewResults(jobId);
            setReview(
              results.job_id,
              results.summary,
              results.comments,
              results.metadata,
            );

            if (status.status === "posted") {
              // auto_post was true; redirect to confirmation
              const postedCount = results.comments.filter((c) => c.approved).length;
              setPostedCount(postedCount);
              navigate(`/confirm/${jobId}`, { replace: true });
            } else {
              setPhase("review");
            }
          } catch (err) {
            if (err instanceof ApiError) {
              setErrorMessage(err.message);
            } else if (err instanceof Error) {
              setErrorMessage(err.message);
            } else {
              setErrorMessage("Failed to load review results");
            }
            setPhase("error");
          }
          return;
        }

        if (status.status === "failed") {
          stopPolling();
          setErrorMessage(status.progress ?? "Review failed");
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
        // Network errors during polling: keep polling, don't crash
        setProgressMessage("Connection issue, retrying...");
      }
    };

    // Initial poll immediately
    poll();
    intervalRef.current = setInterval(poll, POLL_INTERVAL_MS);

    return () => {
      stopPolling();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [jobId]);

  const handleContinueWaiting = () => {
    pollStartRef.current = Date.now();
    setPhase("polling");
    stoppedRef.current = false;
    const poll = async () => {
      if (stoppedRef.current || !jobId) return;
      if (Date.now() - pollStartRef.current > POLL_TIMEOUT_MS) {
        stopPolling();
        setPhase("timeout");
        return;
      }
      try {
        const status = await getJobStatus(jobId);
        if (stoppedRef.current) return;
        setProgressMessage(status.progress ?? "Processing...");
        if (status.status === "complete" || status.status === "posted") {
          stopPolling();
          const results = await getReviewResults(jobId);
          setReview(results.job_id, results.summary, results.comments, results.metadata);
          if (status.status === "posted") {
            const postedCount = results.comments.filter((c) => c.approved).length;
            setPostedCount(postedCount);
            navigate(`/confirm/${jobId}`, { replace: true });
          } else {
            setPhase("review");
          }
        } else if (status.status === "failed") {
          stopPolling();
          setErrorMessage(status.progress ?? "Review failed");
          setPhase("error");
        }
      } catch {
        setProgressMessage("Connection issue, retrying...");
      }
    };
    poll();
    intervalRef.current = setInterval(poll, POLL_INTERVAL_MS);
  };

  const handleEditBody = (commentId: string, newBody: string) => {
    // Optimistic update
    updateCommentBody(commentId, newBody);
    setEditingIndex(-1);
    // Fire-and-forget API call
    if (jobId) {
      editCommentApi(jobId, commentId, newBody).catch(() => {
        // Edit failed server-side; local state still reflects the edit.
      });
    }
  };

  const handlePost = async () => {
    if (!jobId) return;

    const approvedComments = comments.filter((c) => c.approved);
    if (approvedComments.length === 0) return;

    setIsPosting(true);
    setErrorMessage("");

    try {
      await postReview(jobId, {
        comment_ids: approvedComments.map((c) => c.id),
        summary,
      });
      setPostedCount(approvedComments.length);
      navigate(`/confirm/${jobId}`);
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorMessage(err.message);
      } else if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage("Failed to post review");
      }
    } finally {
      setIsPosting(false);
    }
  };

  const approvedCount = comments.filter((c) => c.approved).length;
  const rejectedCount = comments.filter((c) => !c.approved).length;

  // --- Polling phase ---
  if (phase === "polling") {
    return (
      <div className="page-transition space-y-8 pt-12">
        <div className="flex flex-col items-center justify-center space-y-6">
          <Loader2 className="h-10 w-10 text-primary animate-spin" />
          <div className="text-center space-y-2">
            <h2 className="text-lg font-medium text-foreground">Running review</h2>
            <p className="text-sm text-muted-foreground">{progressMessage}</p>
          </div>
        </div>
        {/* Skeleton loading cards */}
        <div className="space-y-4 max-w-3xl mx-auto">
          <SkeletonCommentCard />
          <SkeletonCommentCard />
          <SkeletonCommentCard />
        </div>
      </div>
    );
  }

  // --- Timeout phase ---
  if (phase === "timeout") {
    return (
      <div className="page-transition flex flex-col items-center justify-center pt-24 space-y-6">
        <AlertCircle className="h-10 w-10 text-severity-warning" />
        <div className="text-center space-y-2">
          <h2 className="text-lg font-medium text-foreground">
            Review is taking longer than expected
          </h2>
          <p className="text-sm text-muted-foreground">
            The review has been running for over 5 minutes.
          </p>
        </div>
        <div className="flex gap-3">
          <Button variant="outline" onClick={handleContinueWaiting}>
            Keep Waiting
          </Button>
          <Button variant="outline" onClick={() => navigate("/")}>
            Start Over
          </Button>
        </div>
      </div>
    );
  }

  // --- Not found ---
  if (phase === "not-found") {
    return (
      <div className="page-transition flex flex-col items-center justify-center pt-24 space-y-6">
        <AlertCircle className="h-10 w-10 text-destructive" />
        <div className="text-center space-y-2">
          <h2 className="text-lg font-medium text-foreground">Review not found</h2>
          <p className="text-sm text-muted-foreground">
            This review job does not exist or has expired.
          </p>
        </div>
        <Button variant="outline" onClick={() => navigate("/")}>
          Back to Configure
        </Button>
      </div>
    );
  }

  // --- Error phase ---
  if (phase === "error") {
    return (
      <div className="page-transition flex flex-col items-center justify-center pt-24 space-y-6">
        <AlertCircle className="h-10 w-10 text-destructive" />
        <div className="text-center space-y-2">
          <h2 className="text-lg font-medium text-foreground">Review failed</h2>
          <p className="text-sm text-muted-foreground">{errorMessage}</p>
        </div>
        <Button variant="outline" onClick={() => navigate("/")}>
          <RotateCcw className="h-4 w-4 mr-2" />
          Try Again
        </Button>
      </div>
    );
  }

  // --- Review phase ---
  return (
    <div className="page-transition space-y-6">
      {/* MR Info */}
      {metadata && (
        <div>
          <h1 className="text-lg font-medium text-foreground">
            {metadata.title}
          </h1>
          <div className="flex items-center gap-2 mt-1">
            <span className="font-mono text-xs text-muted-foreground">
              {metadata.source_branch}
            </span>
            <span className="text-muted-foreground/40">&rarr;</span>
            <span className="font-mono text-xs text-muted-foreground">
              {metadata.target_branch}
            </span>
          </div>
        </div>
      )}

      <Separator />

      {/* Summary */}
      <div className="space-y-2">
        <label
          htmlFor="review-summary"
          className="text-sm text-muted-foreground font-medium"
        >
          Summary
        </label>
        <Textarea
          id="review-summary"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          className="min-h-[100px] text-sm bg-surface border-border resize-y"
        />
      </div>

      {/* Stats bar */}
      <div className="flex items-center gap-4 text-sm">
        <span className="text-foreground font-medium">
          {comments.length} comments
        </span>
        <span className="text-muted-foreground">&middot;</span>
        <span className="text-success">{approvedCount} approved</span>
        <span className="text-muted-foreground">&middot;</span>
        <span className="text-destructive">{rejectedCount} rejected</span>
      </div>

      <Separator />

      {/* Empty state */}
      {comments.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 space-y-4">
          <div className="rounded-full bg-success/10 p-4">
            <CheckCircle2 className="h-10 w-10 text-success" />
          </div>
          <div className="text-center space-y-1">
            <h3 className="text-base font-medium text-foreground">No issues found</h3>
            <p className="text-sm text-muted-foreground">
              The AI didn't find any issues worth commenting on.
            </p>
          </div>
        </div>
      ) : (
        <>
          {/* Comment list */}
          <div className="space-y-4">
            {comments.map((comment, index) => (
              <div
                key={comment.id}
                ref={(el) => { commentRefs.current[index] = el; }}
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

          {/* Error inline */}
          {errorMessage && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
              {errorMessage}
            </div>
          )}

          {/* Sticky bottom bar */}
          <div className="sticky bottom-0 -mx-6 border-t border-border bg-background/90 backdrop-blur-sm px-6 py-4">
            <div className="flex items-center justify-between max-w-5xl mx-auto">
              <span className="text-sm text-muted-foreground">
                {approvedCount} comment{approvedCount !== 1 ? "s" : ""} will be
                posted
              </span>
              <Button
                onClick={handlePost}
                disabled={approvedCount === 0 || isPosting}
                className="gap-2"
              >
                {isPosting ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Posting...
                  </>
                ) : (
                  <>
                    <Send className="h-4 w-4" />
                    Post {approvedCount} Approved Comment
                    {approvedCount !== 1 ? "s" : ""}
                  </>
                )}
              </Button>
            </div>
          </div>
        </>
      )}

      {/* Keyboard shortcut help */}
      <ShortcutHelp />
    </div>
  );
}
