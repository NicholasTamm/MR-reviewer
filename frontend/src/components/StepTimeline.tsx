import { useRef, useEffect, useState } from "react";
import { CheckCircle2, XCircle } from "lucide-react";
import type { JobStatus } from "@/types/api";

interface StepTimelineProps {
  status: JobStatus["status"];
  progress: string | null;
  error: string | null;
}

interface Stage {
  label: string;
  statuses: JobStatus["status"][];
}

const STAGES: Stage[] = [
  { label: "Queued", statuses: ["pending"] },
  { label: "Fetching MR", statuses: ["fetching"] },
  { label: "Reviewing", statuses: ["reviewing"] },
  { label: "Complete", statuses: ["complete", "posted"] },
];

function getActiveStageIndex(status: JobStatus["status"]): number {
  for (let i = 0; i < STAGES.length; i++) {
    if (STAGES[i].statuses.includes(status)) {
      return i;
    }
  }
  return -1;
}

function formatElapsed(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function StepTimeline({ status, progress, error }: StepTimelineProps) {
  const isFailed = status === "failed";
  const activeStageIndex = isFailed ? -1 : getActiveStageIndex(status);

  // Track the last non-failed stage for failed visual state
  const lastActiveIndexRef = useRef<number>(0);
  if (!isFailed && activeStageIndex >= 0) {
    lastActiveIndexRef.current = activeStageIndex;
  }
  const failedStageIndex = isFailed ? lastActiveIndexRef.current : -1;

  // Track when each stage started (client-side only)
  const stageStartTimesRef = useRef<Record<number, number>>({});
  const prevActiveIndexRef = useRef<number>(-1);

  // Record timestamp when we first see a new stage become active
  const displayIndex = isFailed ? failedStageIndex : activeStageIndex;
  if (!isFailed && activeStageIndex >= 0 && activeStageIndex !== prevActiveIndexRef.current) {
    stageStartTimesRef.current[activeStageIndex] = Date.now();
    prevActiveIndexRef.current = activeStageIndex;
  }

  // Force re-render every second during active polling
  const [, setTick] = useState(0);
  useEffect(() => {
    if (status === "complete" || status === "posted" || status === "failed") {
      return;
    }
    const id = setInterval(() => setTick((t) => t + 1), 1000);
    return () => clearInterval(id);
  }, [status]);

  const now = Date.now();

  return (
    <div role="status" aria-live="polite" aria-label="Review progress" className="flex flex-col gap-0 w-full max-w-sm mx-auto pt-12">
      {STAGES.map((stage, index) => {
        const isCompleted = !isFailed && index < activeStageIndex;
        const isActive = !isFailed && index === activeStageIndex;
        const isFailed_ = isFailed && index === failedStageIndex;
        const isPending = !isCompleted && !isActive && !isFailed_;

        const startTime = stageStartTimesRef.current[index];
        const elapsed = startTime != null ? now - startTime : 0;
        const isLast = index === STAGES.length - 1;

        return (
          <div key={stage.label} className="flex flex-col">
            <div className="flex items-start gap-4">
              {/* Step node */}
              <div className="flex flex-col items-center">
                <div className="flex items-center justify-center w-8 h-8 mt-0.5">
                  {isCompleted && (
                    <CheckCircle2 className="w-6 h-6 text-success" />
                  )}
                  {isActive && (
                    <div className="step-pulse w-4 h-4 rounded-full bg-primary" />
                  )}
                  {isFailed_ && (
                    <XCircle className="w-6 h-6 text-destructive" />
                  )}
                  {isPending && (
                    <div className="w-4 h-4 rounded-full border-2 border-muted-foreground/40" />
                  )}
                </div>
              </div>

              {/* Stage content */}
              <div className="flex-1 pb-1">
                <div className="flex items-center justify-between">
                  <span
                    className={
                      isCompleted
                        ? "text-sm font-medium text-success"
                        : isActive
                          ? "text-sm font-medium text-primary"
                          : isFailed_
                            ? "text-sm font-medium text-destructive"
                            : "text-sm font-medium text-muted-foreground/40"
                    }
                  >
                    {stage.label}
                  </span>
                  {/* Elapsed time */}
                  {isCompleted && startTime != null && (
                    <span className="text-xs text-muted-foreground">
                      {formatElapsed(elapsed)}
                    </span>
                  )}
                  {isActive && startTime != null && (
                    <span className="text-xs text-muted-foreground tabular-nums">
                      {formatElapsed(elapsed)}
                    </span>
                  )}
                </div>

                {/* Subtitle */}
                {isActive && progress && (
                  <p className="text-xs text-muted-foreground mt-0.5">{progress}</p>
                )}
                {isFailed_ && error && (
                  <p className="text-xs text-destructive mt-0.5">{error}</p>
                )}
              </div>
            </div>

            {/* Connector line between steps */}
            {!isLast && (
              <div className="flex">
                <div className="flex justify-center w-8">
                  <div
                    className={`w-0.5 h-6 ${
                      index < displayIndex ? "bg-primary" : "bg-border"
                    }`}
                  />
                </div>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
