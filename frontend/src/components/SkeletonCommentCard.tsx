export function SkeletonCommentCard() {
  return (
    <div aria-hidden="true" className="flex rounded border border-border bg-surface overflow-hidden animate-pulse">
      {/* Left stripe */}
      <div className="w-[3px] shrink-0 self-stretch bg-muted" />

      {/* Content */}
      <div className="flex-1 p-4 space-y-3">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="h-3 w-36 rounded bg-muted" />
            <div className="h-3 w-8 rounded bg-muted" />
          </div>
          <div className="h-3 w-12 rounded bg-muted" />
        </div>

        {/* Diff block */}
        <div className="rounded border border-border overflow-hidden">
          <div className="bg-muted/60 px-3 py-1.5">
            <div className="h-2.5 w-40 rounded bg-muted-foreground/10" />
          </div>
          <div className="bg-diff-bg p-2 space-y-1.5">
            <div className="h-2.5 w-full rounded bg-muted-foreground/5" />
            <div className="h-2.5 w-4/5 rounded bg-muted-foreground/5" />
            <div className="h-2.5 w-3/4 rounded bg-muted-foreground/5" />
          </div>
        </div>

        {/* Body */}
        <div className="space-y-2">
          <div className="h-3.5 w-full rounded bg-muted" />
          <div className="h-3.5 w-3/4 rounded bg-muted" />
        </div>

        {/* Footer */}
        <div className="flex gap-px pt-1">
          <div className="flex-1 h-8 rounded bg-muted" />
          <div className="flex-1 h-8 rounded bg-muted" />
        </div>
      </div>
    </div>
  );
}
