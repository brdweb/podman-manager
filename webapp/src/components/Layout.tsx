import { Link, Outlet, useLocation } from 'react-router-dom';
import { useLogout, useSession } from '../hooks/useAuth';

const navItems = [
  { to: '/', label: 'Dashboard' },
  { to: '/containers', label: 'Containers' },
  { to: '/hosts', label: 'Hosts' },
  { to: '/admin', label: 'Admin' },
];

export function Layout() {
  const location = useLocation();
  const { data: session } = useSession();
  const logout = useLogout();

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100">
      <header className="border-b border-zinc-800 bg-zinc-900/80 backdrop-blur-sm sticky top-0 z-10">
        <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-6">
          <div className="flex items-center">
          <Link to="/" className="text-lg font-bold tracking-tight mr-8">
            Podman Manager
          </Link>
          <nav className="flex gap-1">
            {navItems.map((item) => {
              const isActive =
                item.to === '/'
                  ? location.pathname === '/'
                  : location.pathname.startsWith(item.to);
              return (
                <Link
                  key={item.to}
                  to={item.to}
                  className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-zinc-800 text-zinc-100'
                      : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50'
                  }`}
                >
                  {item.label}
                </Link>
              );
            })}
          </nav>
          </div>

          {session?.enabled && (
            <div className="flex items-center gap-3 text-sm text-zinc-400">
              <span>{session.username}</span>
              <button
                type="button"
                onClick={() => logout.mutate()}
                className="rounded-md border border-zinc-700 px-3 py-1.5 text-zinc-300 transition-colors hover:border-zinc-500 hover:text-zinc-100"
              >
                Sign out
              </button>
            </div>
          )}
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8">
        <Outlet />
      </main>
    </div>
  );
}
