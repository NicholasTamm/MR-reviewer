import { useEffect, useState } from "react";
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

function ActiveStageElapsed() {
  const startedAt = useState(() => Date.now())[0];
  const [now, setNow] = useState(startedAt);

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  return <span className="text-xs text-muted-foreground tabular-nums">{formatElapsed(now - startedAt)}</span>;
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

  return (
    <div role="status" aria-live="polite" aria-label="Review progress" className="flex flex-col gap-0 w-full max-w-sm mx-auto pt-12">
      {STAGES.map((stage, index) => {
        const isCompleted = !isFailed && index < activeStageIndex;
        const isActive = !isFailed && index === activeStageIndex;
        const isFailed_ = isFailed && index === Math.max(0, activeStageIndex);
        const isPending = !isCompleted && !isActive && !isFailed_;

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
                  {isActive && <ActiveStageElapsed key={stage.label} />}
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
                      index < activeStageIndex ? "bg-primary" : "bg-border"
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
