import { Link, Outlet, useLocation } from 'react-router-dom';
import { isAdmin, useLogout, useSession } from '../hooks/useAuth';
import { useQuery } from '@tanstack/react-query';
import { get } from '../api/client';

const navItems = [
  { to: '/', label: 'Dashboard' },
  { to: '/containers', label: 'Containers' },
  { to: '/hosts', label: 'Volumes' },
  { to: '/images', label: 'Images' },
  { to: '/events', label: 'Events' },
  { to: '/hosts', label: 'Hosts' },
  { to: '/admin', label: 'Admin' },
  { to: '/admin/users', label: 'Users', adminOnly: true },
];

async function getVersion(): Promise<string> {
  const data = await get<{ version: string }>('/api/version');
  return data.version;
}

export function Layout() {
  const location = useLocation();
  const { data: session } = useSession();
  const logout = useLogout();
  const { data: version } = useQuery({
    queryKey: ['version'],
    queryFn: getVersion,
    staleTime: Infinity,
  });
  const hostMatch = location.pathname.match(/^\/hosts\/([^/]+)/);
  const hostName = hostMatch ? decodeURIComponent(hostMatch[1]) : null;
  const scopedNavItems = hostName
    ? [
        ...navItems,
        {
          to: `/hosts/${encodeURIComponent(hostName)}/networks`,
          label: 'Networks',
        },
      ]
    : navItems;

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100">
      <header className="border-b border-zinc-800 bg-zinc-900/80 backdrop-blur-sm sticky top-0 z-10">
        <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-6">
          <div className="flex items-center">
          <Link to="/" className="text-lg font-bold tracking-tight mr-8">
            Podman Manager
          </Link>
          <nav className="flex gap-1">
            {scopedNavItems.filter((item) => !item.adminOnly || isAdmin(session)).map((item) => {
              const to = item.label === 'Volumes' && hostName
                ? `/hosts/${encodeURIComponent(hostName)}/volumes`
                : item.to;
              const isActive =
                item.label === 'Volumes'
                  ? /^\/hosts\/[^/]+\/volumes$/.test(location.pathname)
                : item.label === 'Hosts'
                  ? location.pathname === '/hosts' || /^\/hosts\/[^/]+$/.test(location.pathname)
                : item.to === '/'
                  ? location.pathname === '/'
                  : item.to === '/admin'
                    ? location.pathname === '/admin'
                  : location.pathname.startsWith(to);
              return (
                <Link
                  key={item.label}
                  to={to}
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

          <div className="flex items-center gap-4">
            {version && (
              <span className="text-xs text-zinc-500 font-mono">v{version}</span>
            )}
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
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8">
        <Outlet />
      </main>
    </div>
  );
}
