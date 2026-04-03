/** TypeScript interfaces mirroring the FastAPI Pydantic schemas. */

export interface ReviewRequest {
  url: string;
  provider: string;
  model: string | null;
  focus: string[];
  max_comments: number;
  parallel: boolean;
  auto_post: boolean;
}

export interface JobStatus {
  job_id: string;
  status: "pending" | "fetching" | "reviewing" | "complete" | "failed" | "posted" | "posting";
  progress: string | null;
  error: string | null;
  error_type: string | null;
  created_at: string;
  url: string;
}

export interface MRMetadataResponse {
  title: string;
  description: string;
  source_branch: string;
  target_branch: string;
  web_url: string;
}

export interface CommentDetail {
  id: string;
  file: string;
  line: number;
  body: string;
  severity: "error" | "warning" | "info";
  is_new_line: boolean;
  diff_context: string[];
  approved: boolean;
}

export interface ReviewResponse {
  job_id: string;
  summary: string;
  comments: CommentDetail[];
  metadata: MRMetadataResponse;
}

export interface PostRequest {
  comment_ids: string[];
  summary: string;
}

export interface ConfigDefaults {
  provider: string;
  model: string;
  focus: string[];
  max_comments: number;
  parallel: boolean;
  parallel_threshold: number;
}
