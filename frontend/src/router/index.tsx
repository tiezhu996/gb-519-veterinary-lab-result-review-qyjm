import { Navigate, createBrowserRouter } from 'react-router-dom';
import App from '../App';
import { useAuth } from '../hooks/useAuth';
import AnimalCasePage from '../pages/AnimalCasePage';
import SpecimenPage from '../pages/SpecimenPage';
import AssayRunPage from '../pages/AssayRunPage';
import ResultSignoffPage from '../pages/ResultSignoffPage';
import AuditPage from '../pages/AuditPage';
import LoginPage from '../pages/LoginPage';

function ProtectedRoute() {
  const { session, loading } = useAuth();
  if (loading) return <div className="app-loading">正在校验会话…</div>;
  return session ? <App /> : <Navigate to="/login" replace />;
}

function AuditRoute() {
  const { hasRole } = useAuth();
  return hasRole('reviewer') ? <AuditPage /> : <Navigate to="/cases" replace />;
}

export const router = createBrowserRouter([
{ path: '/login', element: <LoginPage /> },
{ path: '/', element: <ProtectedRoute />, children: [
  { index: true, element: <Navigate to="/cases" replace /> },
  { path: 'cases', element: <AnimalCasePage /> }, { path: 'specimens', element: <SpecimenPage /> }, { path: 'assays', element: <AssayRunPage /> }, { path: 'signoff', element: <ResultSignoffPage /> },
  { path: 'audit', element: <AuditRoute /> },
] }], { future: { v7_fetcherPersist: true, v7_normalizeFormMethod: true, v7_partialHydration: true, v7_relativeSplatPath: true, v7_skipActionErrorRevalidation: true } });
