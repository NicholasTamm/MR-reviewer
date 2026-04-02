import { createContext, useContext, useState, useCallback } from "react";
import type { ReactNode } from "react";
import type { CommentDetail, MRMetadataResponse } from "@/types/api";

interface ReviewState {
  jobId: string | null;
  summary: string;
  comments: CommentDetail[];
  metadata: MRMetadataResponse | null;
  /** Number of comments that were posted (set after posting). */
  postedCount: number;
}

interface ReviewContextValue extends ReviewState {
  setReview: (
    jobId: string,
    summary: string,
    comments: CommentDetail[],
    metadata: MRMetadataResponse,
  ) => void;
  setSummary: (summary: string) => void;
  toggleApproval: (commentId: string) => void;
  updateCommentBody: (commentId: string, body: string) => void;
  setPostedCount: (count: number) => void;
  reset: () => void;
}

const initialState: ReviewState = {
  jobId: null,
  summary: "",
  comments: [],
  metadata: null,
  postedCount: 0,
};

const ReviewContext = createContext<ReviewContextValue | null>(null);

export function ReviewProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ReviewState>(initialState);

  const setReview = useCallback(
    (
      jobId: string,
      summary: string,
      comments: CommentDetail[],
      metadata: MRMetadataResponse,
    ) => {
      setState({ jobId, summary, comments, metadata, postedCount: 0 });
    },
    [],
  );

  const setSummary = useCallback((summary: string) => {
    setState((prev) => ({ ...prev, summary }));
  }, []);

  const toggleApproval = useCallback((commentId: string) => {
    setState((prev) => ({
      ...prev,
      comments: prev.comments.map((c) =>
        c.id === commentId ? { ...c, approved: !c.approved } : c,
      ),
    }));
  }, []);

  const updateCommentBody = useCallback((commentId: string, body: string) => {
    setState((prev) => ({
      ...prev,
      comments: prev.comments.map((c) =>
        c.id === commentId ? { ...c, body } : c,
      ),
    }));
  }, []);

  const setPostedCount = useCallback((count: number) => {
    setState((prev) => ({ ...prev, postedCount: count }));
  }, []);

  const reset = useCallback(() => {
    setState(initialState);
  }, []);

  return (
    <ReviewContext
      value={{
        ...state,
        setReview,
        setSummary,
        toggleApproval,
        updateCommentBody,
        setPostedCount,
        reset,
      }}
    >
      {children}
    </ReviewContext>
  );
}

export function useReview(): ReviewContextValue {
  const ctx = useContext(ReviewContext);
  if (ctx === null) {
    throw new Error("useReview must be used within a ReviewProvider");
  }
  return ctx;
}
