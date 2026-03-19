import { useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useLogin, useSession } from '../hooks/useAuth';

export function LoginPage() {
  const { data: session, isLoading, error } = useSession();
  const login = useLogin();
  const location = useLocation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  const from = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname || '/';

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-950 text-zinc-300">
        Loading...
      </div>
    );
  }

  if (!session?.enabled || session.authenticated) {
    return <Navigate to={from} replace />;
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await login.mutateAsync({ username, password });
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-950 px-6">
      <div className="w-full max-w-md rounded-3xl border border-zinc-800 bg-zinc-900 p-8 shadow-2xl shadow-black/30">
        <p className="text-xs uppercase tracking-[0.28em] text-zinc-500">Podman Manager</p>
        <h1 className="mt-3 text-3xl font-semibold text-zinc-100">Sign in</h1>
        <p className="mt-2 text-sm text-zinc-500">
          Enter the admin credentials configured for this standalone deployment.
        </p>

        {error && (
          <div className="mt-6 rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
            Failed to contact the Podman Manager API.
          </div>
        )}

        <form onSubmit={handleSubmit} className="mt-8 space-y-4">
          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
              Username
            </span>
            <input
              autoFocus
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              className="w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-zinc-100 outline-none transition-colors focus:border-zinc-600"
            />
          </label>

          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
              Password
            </span>
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-zinc-100 outline-none transition-colors focus:border-zinc-600"
            />
          </label>

          {login.isError && (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
              {(login.error as Error).message}
            </div>
          )}

          <button
            type="submit"
            disabled={login.isPending}
            className="w-full rounded-xl bg-emerald-600 px-4 py-3 text-sm font-medium text-white transition-colors hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {login.isPending ? 'Signing in...' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  );
}
