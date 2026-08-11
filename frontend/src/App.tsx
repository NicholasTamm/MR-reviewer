import { BrowserRouter, HashRouter, Routes, Route } from "react-router-dom";
import { Toaster } from "sonner";
import { ReviewProvider } from "@/context/ReviewContext";
import { AppLayout } from "@/layouts/AppLayout";
import { ConfigurePage } from "@/pages/ConfigurePage";
import { ReviewPage } from "@/pages/ReviewPage";
import { ConfirmationPage } from "@/pages/ConfirmationPage";
import { GitLabProjectsPage } from "@/pages/GitLabProjectsPage";
import { GitLabProjectPage } from "@/pages/GitLabProjectPage";
import { SettingsPage } from "@/pages/SettingsPage";

function App() {
  const Router = window.electronAPI ? HashRouter : BrowserRouter;

  return (
    <ReviewProvider>
      <Toaster theme="dark" position="bottom-right" richColors />
      <Router>
        <Routes>
          <Route element={<AppLayout currentStep="configure" />}>
            <Route path="/" element={<ConfigurePage />} />
            <Route path="/gitlab/merge-requests" element={<GitLabProjectsPage />} />
            <Route path="/gitlab/projects/:projectId" element={<GitLabProjectPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
          <Route element={<AppLayout currentStep="review" />}>
            <Route path="/review/:jobId" element={<ReviewPage />} />
          </Route>
          <Route element={<AppLayout currentStep="confirm" />}>
            <Route path="/confirm/:jobId" element={<ConfirmationPage />} />
          </Route>
        </Routes>
      </Router>
    </ReviewProvider>
  );
}

export default App;
