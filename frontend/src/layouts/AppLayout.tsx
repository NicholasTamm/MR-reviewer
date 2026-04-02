import { Outlet } from "react-router-dom";
import { StepIndicator } from "@/components/StepIndicator";
import type { Step } from "@/types";

interface AppLayoutProps {
  currentStep: Step;
}

export function AppLayout({ currentStep }: AppLayoutProps) {
  return (
    <div className="min-h-screen bg-background">
      {/* Top bar */}
      <header className="sticky top-0 z-50 border-b border-border bg-background/80 backdrop-blur-sm">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-6">
          <span className="font-mono text-sm font-bold tracking-tight text-foreground">
            MR Reviewer
          </span>
          <StepIndicator currentStep={currentStep} />
        </div>
      </header>

      {/* Main content */}
      <main className="mx-auto max-w-5xl px-6 py-8 page-transition">
        <Outlet />
      </main>
    </div>
  );
}
