import { useEffect, useState } from "react";
import { BrowserRouter, HashRouter, Navigate, Outlet, Route, Routes, useLocation, useParams, useSearchParams } from "react-router-dom";
import { Toaster } from "sonner";
import { ReviewProvider } from "@/context/ReviewContext";
import { AppLayout } from "@/layouts/AppLayout";
import { ConfigurePage } from "@/pages/ConfigurePage";
import { ReviewPage } from "@/pages/ReviewPage";
import { ConfirmationPage } from "@/pages/ConfirmationPage";
import { OnboardingPage } from "@/pages/OnboardingPage";
import { SettingsPage } from "@/pages/SettingsPage";
import { getOnboarding } from "@/lib/api";

function OnboardingGate() {
  const location = useLocation();
  const [complete, setComplete] = useState<boolean | null>(null);
  useEffect(() => {
    let cancelled = false;
    getOnboarding()
      .then((status) => {
        if (!cancelled) setComplete(status.complete);
      })
      .catch(() => {
        if (!cancelled) setComplete(false);
      });
    return () => {
      cancelled = true;
    };
  }, [location.pathname]);
  if (complete === null) return <p className="p-6 text-sm text-muted-foreground">Checking shared setup...</p>;
  if (!complete && location.pathname !== "/onboarding") return <Navigate to="/onboarding" replace />;
  if (complete && location.pathname === "/onboarding") return <Navigate to="/" replace />;
  return <Outlet />;
}

function BrowseRedirect() {
  const [params] = useSearchParams();
  const platform = params.get("platform") === "github" ? "github" : "gitlab";
  return <Navigate to={`/?platform=${platform}`} replace />;
}

function BrowseProjectRedirect() {
  const { platform = "gitlab", projectId = "", repo = "" } = useParams();
  const project = platform === "github" && repo ? `${projectId}/${repo}` : projectId;
  const query = new URLSearchParams({ platform, ...(project ? { project } : {}) });
  return <Navigate to={`/?${query}`} replace />;
}

function App() {
  const Router = window.electronAPI ? HashRouter : BrowserRouter;

  return (
    <ReviewProvider>
      <Toaster theme="dark" position="bottom-right" richColors />
      <Router>
        <Routes>
          <Route element={<OnboardingGate />}>
            <Route element={<AppLayout currentStep="configure" />}>
              <Route path="/" element={<ConfigurePage />} />
              <Route path="/onboarding" element={<OnboardingPage />} />
              <Route path="/browse" element={<BrowseRedirect />} />
              <Route path="/browse/github/:projectId/:repo" element={<BrowseProjectRedirect />} />
              <Route path="/browse/:platform/:projectId" element={<BrowseProjectRedirect />} />
              <Route path="/gitlab/merge-requests" element={<Navigate to="/?platform=gitlab" replace />} />
              <Route path="/gitlab/projects/:projectId" element={<BrowseProjectRedirect />} />
              <Route path="/settings" element={<SettingsPage />} />
            </Route>
            <Route element={<AppLayout currentStep="review" />}>
              <Route path="/review/:jobId" element={<ReviewPage />} />
            </Route>
            <Route element={<AppLayout currentStep="confirm" />}>
              <Route path="/confirm/:jobId" element={<ConfirmationPage />} />
            </Route>
          </Route>
        </Routes>
      </Router>
    </ReviewProvider>
  );
}

export default App;
