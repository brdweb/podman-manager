import { BrowserRouter, Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ErrorBoundary } from './components/ErrorBoundary';
import { Layout } from './components/Layout';
import { ToastProvider } from './components/Toast';
import { Dashboard } from './pages/Dashboard';
import { HostList } from './pages/HostList';
import { HostDetail } from './pages/HostDetail';
import { ContainersPage } from './pages/ContainersPage';
import { CreateContainerPage } from './pages/CreateContainerPage';
import { NetworksPage } from './pages/NetworksPage';
import { ImagesPage } from './pages/ImagesPage';
import { VolumesPage } from './pages/VolumesPage';
import { EventsPage } from './pages/EventsPage';
import { AdminPage } from './pages/AdminPage';
import { UsersPage } from './pages/UsersPage';
import { LoginPage } from './pages/LoginPage';
import { NotFoundPage } from './pages/NotFoundPage';
import { useSession } from './hooks/useAuth';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5_000,
      retry: 1,
    },
  },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ErrorBoundary>
          <ToastProvider>
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route element={<ProtectedApp />}>
                <Route element={<Layout />}>
                  <Route path="/" element={<Dashboard />} />
                <Route path="/containers" element={<ContainersPage />} />
                <Route path="/images" element={<ImagesPage />} />
                <Route path="/events" element={<EventsPage />} />
                <Route path="/hosts" element={<HostList />} />
                  <Route path="/hosts/:hostId/containers/create" element={<CreateContainerPage />} />
                  <Route path="/hosts/:hostId/networks" element={<NetworksPage />} />
                  <Route path="/hosts/:hostId/volumes" element={<VolumesPage />} />
                  <Route path="/hosts/:hostName" element={<HostDetail />} />
                  <Route path="/admin" element={<AdminPage />} />
                  <Route path="/admin/users" element={<UsersPage />} />
                  <Route path="*" element={<NotFoundPage />} />
                </Route>
              </Route>
            </Routes>
          </ToastProvider>
        </ErrorBoundary>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function ProtectedApp() {
  const { data: session, isLoading, error } = useSession();
  const location = useLocation();

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-950 text-zinc-300">
        Checking session...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-950 px-6 text-center text-zinc-300">
        <div className="rounded-2xl border border-red-500/30 bg-red-500/10 p-6">
          Failed to contact the Podman Manager API.
        </div>
      </div>
    );
  }

  if (session?.enabled && !session.authenticated) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  return <Outlet />;
}
