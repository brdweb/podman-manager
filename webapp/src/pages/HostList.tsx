import { useHosts } from '../hooks/useHosts';
import { StatusBadge } from '../components/StatusBadge';
import { Link } from 'react-router-dom';

export function HostList() {
  const { data: hosts, isLoading, error } = useHosts();

  if (isLoading) {
    return (
      <div className="animate-pulse space-y-3">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-16 bg-zinc-800 rounded-xl" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center">
        <p className="text-red-400 font-medium">Failed to load hosts</p>
        <p className="text-red-400/60 text-sm mt-1">{error.message}</p>
      </div>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Hosts</h1>

      <div className="overflow-x-auto rounded-xl border border-zinc-800">
        <table className="w-full text-left">
          <thead className="bg-zinc-900 text-zinc-400 text-sm">
            <tr>
              <th className="px-4 py-3 font-medium">Name</th>
              <th className="px-4 py-3 font-medium">Address</th>
              <th className="px-4 py-3 font-medium">Mode</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3 font-medium">Latency</th>
            </tr>
          </thead>
          <tbody>
            {hosts?.map((host) => (
              <tr
                key={host.name}
                className="border-b border-zinc-800 hover:bg-zinc-800/50 transition-colors"
              >
                <td className="px-4 py-3">
                  <Link
                    to={`/hosts/${encodeURIComponent(host.name)}`}
                    className="font-medium text-zinc-100 hover:text-white underline-offset-2 hover:underline"
                  >
                    {host.name}
                  </Link>
                </td>
                <td className="px-4 py-3 text-zinc-400 font-mono text-sm">{host.address}</td>
                <td className="px-4 py-3 text-zinc-400 text-sm">{host.mode}</td>
                <td className="px-4 py-3">
                  <StatusBadge status={host.status} />
                </td>
                <td className="px-4 py-3 text-zinc-500 text-sm">{host.latency ?? '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
