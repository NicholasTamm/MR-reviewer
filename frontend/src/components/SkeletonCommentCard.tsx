export function SkeletonCommentCard() {
  return (
    <div aria-hidden="true" className="rounded-lg border border-border bg-surface p-4 animate-pulse">
      {/* Header skeleton */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-3">
          <div className="h-4 w-4 rounded bg-muted" />
          <div className="h-4 w-32 rounded bg-muted" />
          <div className="h-4 w-8 rounded bg-muted" />
          <div className="h-5 w-14 rounded bg-muted" />
        </div>
        <div className="flex items-center gap-1">
          <div className="h-7 w-8 rounded bg-muted" />
          <div className="h-7 w-8 rounded bg-muted" />
        </div>
      </div>

      {/* Diff block skeleton */}
      <div className="rounded-md border border-border overflow-hidden mb-3">
        <div className="bg-muted px-3 py-1.5">
          <div className="h-3 w-40 rounded bg-muted-foreground/10" />
        </div>
        <div className="bg-diff-bg p-2 space-y-1.5">
          <div className="h-3 w-full rounded bg-muted-foreground/5" />
          <div className="h-3 w-4/5 rounded bg-muted-foreground/5" />
          <div className="h-3 w-3/4 rounded bg-muted-foreground/5" />
          <div className="h-3 w-5/6 rounded bg-muted-foreground/5" />
        </div>
      </div>

      {/* Body skeleton */}
      <div className="mt-3 space-y-2">
        <div className="h-4 w-full rounded bg-muted" />
        <div className="h-4 w-3/4 rounded bg-muted" />
      </div>
    </div>
  );
}
