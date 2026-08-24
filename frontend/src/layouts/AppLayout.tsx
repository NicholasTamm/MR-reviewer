import { Link, Outlet } from "react-router-dom";
import { StepIndicator } from "@/components/StepIndicator";
import type { Step } from "@/types";

interface AppLayoutProps {
  currentStep: Step;
}

export function AppLayout({ currentStep }: AppLayoutProps) {
  return (
    <div className="min-h-screen bg-background">
      <header className="sticky top-0 z-50 border-b border-border bg-background/80 backdrop-blur-sm">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-6">
          <div className="flex items-center gap-6">
            <Link to="/" className="font-mono text-sm font-bold tracking-tight text-foreground hover:text-primary">
              MR Reviewer
            </Link>
            <Link to="/settings" className="text-sm text-muted-foreground hover:text-foreground">
              Settings
            </Link>
          </div>
          <StepIndicator currentStep={currentStep} />
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8 page-transition">
        <Outlet />
      </main>
    </div>
  );
}
