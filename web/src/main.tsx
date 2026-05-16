import React, { lazy, Suspense } from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { App } from "./App";
import { LoginPage } from "./pages/LoginPage";

const DataSourcesPage = lazy(() => import("./pages/DataSourcesPage").then(m => ({ default: m.DataSourcesPage })));
const KnowledgePage = lazy(() => import("./pages/KnowledgePage").then(m => ({ default: m.KnowledgePage })));

function Root() {
  const token = localStorage.getItem("token");
  return (
    <BrowserRouter>
      <Suspense fallback={<div style={{ padding: 24, fontFamily: "system-ui" }}>loading...</div>}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/data-sources"
            element={token ? <DataSourcesPage /> : <Navigate to="/login" replace />}
          />
          <Route
            path="/knowledge"
            element={token ? <KnowledgePage /> : <Navigate to="/login" replace />}
          />
          <Route
            path="/*"
            element={token ? <App /> : <Navigate to="/login" replace />}
          />
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>
);
