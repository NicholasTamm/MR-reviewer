import { useNavigate, useParams } from "react-router-dom";
import type { Step } from "@/types";
import { cn } from "@/lib/utils";

interface StepConfig {
  key: Step;
  label: string;
  getPath: (jobId?: string) => string;
}

const steps: StepConfig[] = [
  { key: "configure", label: "Configure", getPath: () => "/" },
  { key: "review", label: "Review", getPath: (id) => id ? `/review/${id}` : "/" },
  { key: "confirm", label: "Confirm", getPath: (id) => id ? `/confirm/${id}` : "/" },
];

const stepOrder: Step[] = ["configure", "review", "confirm"];

function getStepIndex(step: Step): number {
  return stepOrder.indexOf(step);
}

interface StepIndicatorProps {
  currentStep: Step;
}

export function StepIndicator({ currentStep }: StepIndicatorProps) {
  const navigate = useNavigate();
  const { jobId } = useParams<{ jobId: string }>();
  const currentIndex = getStepIndex(currentStep);

  return (
    <nav aria-label="Review progress" className="flex items-center gap-0">
      {steps.map((step, index) => {
        const isCompleted = index < currentIndex;
        const isCurrent = step.key === currentStep;
        const canNavigate = index < currentIndex;

        return (
          <div key={step.key} className="flex items-center">
            {index > 0 && (
              <span className={cn(
                "mx-2 text-xs font-mono select-none",
                index <= currentIndex ? "text-primary/50" : "text-border"
              )}>
                ──
              </span>
            )}
            <button
              onClick={() => canNavigate && navigate(step.getPath(jobId))}
              disabled={!canNavigate && !isCurrent}
              className={cn(
                "flex items-center gap-1.5 text-xs font-mono transition-colors",
                isCurrent && "text-primary",
                isCompleted && "text-muted-foreground hover:text-foreground cursor-pointer",
                !isCurrent && !isCompleted && "text-muted-foreground/30 cursor-default"
              )}
            >
              <span className={cn(
                "text-[10px] tabular-nums",
                isCurrent ? "text-primary" : isCompleted ? "text-muted-foreground/60" : "text-muted-foreground/20"
              )}>
                0{index + 1}
              </span>
              <span>{step.label}</span>
            </button>
          </div>
        );
      })}
    </nav>
  );
}
