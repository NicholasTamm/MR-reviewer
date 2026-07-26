/** Typed fetch wrapper for all API endpoints. */

import type {
  ReviewRequest,
  JobStatus,
  ReviewResponse,
  CommentDetail,
  PostRequest,
  ConfigDefaults,
  ProviderCatalog,
} from "@/types/api";

class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

let baseUrl: string | null = null;
let authToken: string | null = null;

async function getBaseUrl(): Promise<string> {
  if (baseUrl !== null) return baseUrl;
  if (window.electronAPI) {
    const port = await window.electronAPI.getBackendPort();
    baseUrl = `http://127.0.0.1:${port}`;
  } else {
    baseUrl = "";
  }
  return baseUrl;
}

async function getAuthToken(): Promise<string | null> {
  if (authToken !== null) return authToken;
  authToken = window.electronAPI ? await window.electronAPI.getAuthToken() : null;
  return authToken;
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const [base, token] = await Promise.all([getBaseUrl(), getAuthToken()]);
  const response = await fetch(`${base}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    try {
      const body: unknown = await response.json();
      if (
        typeof body === "object" &&
        body !== null &&
        "detail" in body &&
        typeof (body as Record<string, unknown>).detail === "string"
      ) {
        message = (body as Record<string, string>).detail;
      }
    } catch {
      // Response body was not JSON; use default message
    }
    throw new ApiError(response.status, message);
  }

  // 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

export function submitReview(
  reviewRequest: ReviewRequest,
): Promise<{ job_id: string }> {
  return request<{ job_id: string }>("/api/reviews", {
    method: "POST",
    body: JSON.stringify(reviewRequest),
  });
}

export function getJobStatus(jobId: string): Promise<JobStatus> {
  return request<JobStatus>(`/api/reviews/${encodeURIComponent(jobId)}`);
}

export function getReviewResults(jobId: string): Promise<ReviewResponse> {
  return request<ReviewResponse>(
    `/api/reviews/${encodeURIComponent(jobId)}/results`,
  );
}

export function editComment(
  jobId: string,
  commentId: string,
  body: string,
): Promise<CommentDetail> {
  return request<CommentDetail>(
    `/api/reviews/${encodeURIComponent(jobId)}/comments/${encodeURIComponent(commentId)}`,
    {
      method: "PATCH",
      body: JSON.stringify({ body }),
    },
  );
}

export function postReview(
  jobId: string,
  postRequest: PostRequest,
): Promise<void> {
  return request<void>(
    `/api/reviews/${encodeURIComponent(jobId)}/post`,
    {
      method: "POST",
      body: JSON.stringify(postRequest),
    },
  );
}

export function getConfigDefaults(): Promise<ConfigDefaults> {
  return request<ConfigDefaults>("/api/config/defaults");
}

export function getProviderModels(): Promise<ProviderCatalog> {
  return request<ProviderCatalog>("/api/providers/models");
}

export { ApiError };
