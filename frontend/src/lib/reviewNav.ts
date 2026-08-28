export type ConfigureSelection = {
  platform?: string | null;
  project?: string | null;
  url?: string | null;
};

export function selectionSearch(selection: ConfigureSelection): string {
  const params = new URLSearchParams();
  const platform = normalizePlatform(selection.platform) ?? platformFromReviewUrl(selection.url);
  if (platform) params.set("platform", platform);
  if (selection.project) params.set("project", selection.project);
  if (selection.url) params.set("url", selection.url);
  const query = params.toString();
  return query ? `?${query}` : "";
}

export function configurePath(selection: ConfigureSelection): string {
  return `/${selectionSearch(selection)}`;
}

function normalizePlatform(value?: string | null): "gitlab" | "github" | null {
  if (value === "github" || value === "gitlab") return value;
  return null;
}

function platformFromReviewUrl(url?: string | null): "gitlab" | "github" | null {
  if (!url) return null;
  if (/\/pull\/\d+/.test(url)) return "github";
  if (/\/merge_requests\/\d+/.test(url)) return "gitlab";
  return null;
}
