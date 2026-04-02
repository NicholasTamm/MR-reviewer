import { Check } from "lucide-react";
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
    <nav aria-label="Review progress" className="flex items-center gap-2">
      {steps.map((step, index) => {
        const isCompleted = index < currentIndex;
        const isCurrent = step.key === currentStep;
        const canNavigate = index < currentIndex;

        return (
          <div key={step.key} className="flex items-center gap-2">
            {index > 0 && (
              <div
                className={cn(
                  "h-px w-8",
                  index <= currentIndex ? "bg-primary" : "bg-border"
                )}
              />
            )}
            <button
              onClick={() => canNavigate && navigate(step.getPath(jobId))}
              disabled={!canNavigate}
              className={cn(
                "flex items-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
                isCurrent && "bg-primary/10 text-primary",
                isCompleted &&
                  "text-muted-foreground hover:text-foreground cursor-pointer",
                !isCurrent &&
                  !isCompleted &&
                  "text-muted-foreground/50 cursor-default"
              )}
            >
              {isCompleted ? (
                <Check className="h-3.5 w-3.5 text-success" />
              ) : (
                <span
                  className={cn(
                    "flex h-5 w-5 items-center justify-center rounded-full text-xs font-mono",
                    isCurrent
                      ? "bg-primary text-primary-foreground step-pulse"
                      : "bg-muted text-muted-foreground"
                  )}
                >
                  {index + 1}
                </span>
              )}
              {step.label}
            </button>
          </div>
        );
      })}
    </nav>
  );
}
