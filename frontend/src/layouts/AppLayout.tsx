import { Outlet, Link, useLocation } from "react-router-dom";
import { Settings, X } from "lucide-react";
import { StepIndicator } from "@/components/StepIndicator";
import type { Step } from "@/types";

interface AppLayoutProps {
  currentStep: Step;
}

export function AppLayout({ currentStep }: AppLayoutProps) {
  const location = useLocation();
  const onSettings = location.pathname === "/settings";

  return (
    <div className="min-h-screen bg-background">
      {/* Top bar */}
      <header className="sticky top-0 z-50 border-b border-border bg-background/90 backdrop-blur-sm">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-6">
          <Link to="/" className="font-heading text-base font-700 tracking-tight text-foreground hover:text-primary transition-colors">
            MR Reviewer
          </Link>
          <div className="flex items-center gap-5">
            <StepIndicator currentStep={currentStep} />
            <Link
              to={onSettings ? "/" : "/settings"}
              aria-label={onSettings ? "Close settings" : "Settings"}
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              {onSettings
                ? <X className="h-4 w-4" />
                : <Settings className="h-4 w-4" />
              }
            </Link>
          </div>
        </div>
      </header>

      {/* Main content */}
      <main className="mx-auto max-w-5xl px-6 py-10">
        <Outlet />
      </main>
    </div>
  );
}
