import React, { lazy, Suspense } from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { App } from "./App";
import { LoginPage } from "./pages/LoginPage";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { PageSkeleton } from "./components/Skeleton";

const DataSourcesPage = lazy(() => import("./pages/DataSourcesPage").then(m => ({ default: m.DataSourcesPage })));
const KnowledgePage = lazy(() => import("./pages/KnowledgePage").then(m => ({ default: m.KnowledgePage })));
const TablesPage = lazy(() => import("./pages/TablesPage").then(m => ({ default: m.TablesPage })));
const AdminUsersPage = lazy(() => import("./pages/AdminUsersPage").then(m => ({ default: m.AdminUsersPage })));

function Root() {
  const token = localStorage.getItem("token");
  return (
    <BrowserRouter>
      <ErrorBoundary>
        <Suspense fallback={<PageSkeleton />}>
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
              path="/tables"
              element={token ? <TablesPage /> : <Navigate to="/login" replace />}
            />
            <Route
              path="/admin/users"
              element={token ? <AdminUsersPage /> : <Navigate to="/login" replace />}
            />
            <Route
              path="/*"
              element={token ? <App /> : <Navigate to="/login" replace />}
            />
          </Routes>
        </Suspense>
      </ErrorBoundary>
    </BrowserRouter>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>
);
