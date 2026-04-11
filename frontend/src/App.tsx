import { HashRouter, Routes, Route } from "react-router-dom";
import { Toaster } from "sonner";
import { ReviewProvider } from "@/context/ReviewContext";
import { AppLayout } from "@/layouts/AppLayout";
import { ConfigurePage } from "@/pages/ConfigurePage";
import { ReviewPage } from "@/pages/ReviewPage";
import { ConfirmationPage } from "@/pages/ConfirmationPage";
import { SettingsPage } from "@/pages/SettingsPage";

function App() {
  return (
    <ReviewProvider>
      <Toaster theme="dark" position="bottom-right" richColors />
      <HashRouter>
        <Routes>
          <Route element={<AppLayout currentStep="configure" />}>
            <Route path="/" element={<ConfigurePage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
          <Route element={<AppLayout currentStep="review" />}>
            <Route path="/review/:jobId" element={<ReviewPage />} />
          </Route>
          <Route element={<AppLayout currentStep="confirm" />}>
            <Route path="/confirm/:jobId" element={<ConfirmationPage />} />
          </Route>
        </Routes>
      </HashRouter>
    </ReviewProvider>
  );
}

export default App;
