import { useRef, useEffect, useState } from "react";
import { Check, X } from "lucide-react";
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

  const lastActiveIndexRef = useRef<number>(0);
  if (!isFailed && activeStageIndex >= 0) {
    lastActiveIndexRef.current = activeStageIndex;
  }
  const failedStageIndex = isFailed ? lastActiveIndexRef.current : -1;

  const stageStartTimesRef = useRef<Record<number, number>>({});
  const prevActiveIndexRef = useRef<number>(-1);

  const displayIndex = isFailed ? failedStageIndex : activeStageIndex;
  if (!isFailed && activeStageIndex >= 0 && activeStageIndex !== prevActiveIndexRef.current) {
    stageStartTimesRef.current[activeStageIndex] = Date.now();
    prevActiveIndexRef.current = activeStageIndex;
  }

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
    <div role="status" aria-live="polite" aria-label="Review progress" className="flex flex-col gap-0 w-full">
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
              <div className="flex flex-col items-center w-8">
                <div className="flex items-center justify-center w-8 h-8 mt-0.5">
                  {isCompleted && (
                    <div className="w-5 h-5 rounded-full bg-success/15 flex items-center justify-center">
                      <Check className="w-3 h-3 text-success" />
                    </div>
                  )}
                  {isActive && (
                    <div className="step-pulse w-3.5 h-3.5 rounded-full bg-primary" />
                  )}
                  {isFailed_ && (
                    <div className="w-5 h-5 rounded-full bg-destructive/15 flex items-center justify-center">
                      <X className="w-3 h-3 text-destructive" />
                    </div>
                  )}
                  {isPending && (
                    <div className="w-3.5 h-3.5 rounded-full border border-border" />
                  )}
                </div>
              </div>

              {/* Stage content */}
              <div className="flex-1 pb-1 pt-1.5">
                <div className="flex items-center justify-between">
                  <span className={
                    isCompleted
                      ? "text-xs font-medium text-success"
                      : isActive
                        ? "text-xs font-medium text-primary"
                        : isFailed_
                          ? "text-xs font-medium text-destructive"
                          : "text-xs font-medium text-muted-foreground/30"
                  }>
                    {stage.label}
                  </span>
                  {(isCompleted || isActive) && startTime != null && (
                    <span className="text-[10px] font-mono text-muted-foreground tabular-nums">
                      {formatElapsed(elapsed)}
                    </span>
                  )}
                </div>

                {isActive && progress && (
                  <p className="text-[10px] text-muted-foreground mt-0.5 leading-relaxed">{progress}</p>
                )}
                {isFailed_ && error && (
                  <p className="text-[10px] text-destructive mt-0.5 leading-relaxed">{error}</p>
                )}
              </div>
            </div>

            {/* Connector */}
            {!isLast && (
              <div className="flex ml-3.5">
                <div className={`w-px h-5 ml-0.5 ${index < displayIndex ? "bg-success/40" : "bg-border"}`} />
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
