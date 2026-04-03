import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Toaster } from "sonner";
import { ReviewProvider } from "@/context/ReviewContext";
import { AppLayout } from "@/layouts/AppLayout";
import { ConfigurePage } from "@/pages/ConfigurePage";
import { ReviewPage } from "@/pages/ReviewPage";
import { ConfirmationPage } from "@/pages/ConfirmationPage";

function App() {
  return (
    <ReviewProvider>
      <Toaster theme="dark" position="bottom-right" richColors />
      <BrowserRouter>
        <Routes>
          <Route element={<AppLayout currentStep="configure" />}>
            <Route path="/" element={<ConfigurePage />} />
          </Route>
          <Route element={<AppLayout currentStep="review" />}>
            <Route path="/review/:jobId" element={<ReviewPage />} />
          </Route>
          <Route element={<AppLayout currentStep="confirm" />}>
            <Route path="/confirm/:jobId" element={<ConfirmationPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ReviewProvider>
  );
}

export default App;
