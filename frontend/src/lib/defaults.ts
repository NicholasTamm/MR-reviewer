import type { ConfigDefaults } from "@/types/api";
import { canonicalProvider, providerLabel } from "@/lib/providers";

export const FOCUS_OPTIONS = [
  "bugs",
  "style",
  "best-practices",
  "security",
  "performance",
] as const;

export const DEFAULTS_STORAGE_KEY = "mr-reviewer.review-defaults";

export type ReviewDefaults = {
  provider: string;
  model: string;
  focus: string[];
  maxComments: number;
  parallel: boolean;
  autoPost: boolean;
};

export const FALLBACK_DEFAULTS: ReviewDefaults = {
  provider: "anthropic",
  model: "",
  focus: ["bugs", "style", "best-practices"],
  maxComments: 10,
  parallel: false,
  autoPost: false,
};

export function loadLocalDefaults(): ReviewDefaults | null {
  try {
    const raw = localStorage.getItem(DEFAULTS_STORAGE_KEY);
    if (!raw) return null;
    return normalizeDefaults(JSON.parse(raw) as Partial<ReviewDefaults>);
  } catch {
    return null;
  }
}

export function saveLocalDefaults(defaults: ReviewDefaults): void {
  localStorage.setItem(DEFAULTS_STORAGE_KEY, JSON.stringify(normalizeDefaults(defaults)));
}

export function defaultsFromServer(server: ConfigDefaults | null): ReviewDefaults {
  if (!server) return { ...FALLBACK_DEFAULTS };
  return normalizeDefaults({
    provider: server.provider,
    model: server.model ?? "",
    focus: server.focus,
    maxComments: server.max_comments,
    parallel: server.parallel,
  });
}

export function defaultsFromEnvMap(values: Record<string, string>): Partial<ReviewDefaults> {
  const focus = values.MR_REVIEWER_FOCUS
    ? values.MR_REVIEWER_FOCUS.split(",").map((item) => item.trim()).filter(Boolean)
    : undefined;
  const max = Number(values.MR_REVIEWER_MAX_COMMENTS);
  return {
    provider: values.MR_REVIEWER_PROVIDER || undefined,
    model: values.MR_REVIEWER_MODEL || undefined,
    focus,
    maxComments: Number.isFinite(max) && max > 0 ? max : undefined,
    parallel: parseTruthy(values.MR_REVIEWER_PARALLEL),
    autoPost: parseTruthy(values.MR_REVIEWER_AUTO_POST),
  };
}

export function defaultsToEnvMap(defaults: ReviewDefaults): Record<string, string> {
  return {
    MR_REVIEWER_PROVIDER: defaults.provider,
    MR_REVIEWER_MODEL: defaults.model,
    MR_REVIEWER_FOCUS: defaults.focus.join(","),
    MR_REVIEWER_MAX_COMMENTS: String(defaults.maxComments),
    MR_REVIEWER_PARALLEL: defaults.parallel ? "true" : "false",
    MR_REVIEWER_AUTO_POST: defaults.autoPost ? "true" : "false",
  };
}

export function resolveDefaults(
  server: ConfigDefaults | null,
  local: ReviewDefaults | null,
  env: Partial<ReviewDefaults> = {},
): ReviewDefaults {
  if (local) return local;
  return normalizeDefaults({
    ...defaultsFromServer(server),
    ...env,
  });
}

export function normalizeDefaults(value: Partial<ReviewDefaults>): ReviewDefaults {
  const focus = (value.focus ?? FALLBACK_DEFAULTS.focus).filter((item) =>
    FOCUS_OPTIONS.includes(item as (typeof FOCUS_OPTIONS)[number]),
  );
  const max = Number(value.maxComments);
  return {
    provider: canonicalProvider(value.provider || FALLBACK_DEFAULTS.provider) || FALLBACK_DEFAULTS.provider,
    model: value.model?.trim() ?? "",
    focus: focus.length > 0 ? focus : [...FALLBACK_DEFAULTS.focus],
    maxComments: Number.isFinite(max) ? Math.min(50, Math.max(1, max)) : FALLBACK_DEFAULTS.maxComments,
    parallel: Boolean(value.parallel),
    autoPost: Boolean(value.autoPost),
  };
}

export function configSummary(value: ReviewDefaults): string {
  const focus = value.focus.length > 0 ? value.focus.join(", ") : "no focus";
  return [
    providerLabel(value.provider),
    value.model || "no model",
    focus,
    `max ${value.maxComments}`,
    value.autoPost ? "auto-post" : "manual",
    value.parallel ? "parallel" : null,
  ].filter(Boolean).join(" · ");
}

function parseTruthy(value: string | undefined): boolean | undefined {
  if (value == null || value === "") return undefined;
  return ["1", "true", "yes"].includes(value.toLowerCase());
}
