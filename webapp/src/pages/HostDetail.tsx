import { useParams, Link } from 'react-router-dom';
import { useContainers } from '../hooks/useContainers';
import { ContainerRow } from '../components/ContainerRow';

export function HostDetail() {
  const { hostName } = useParams<{ hostName: string }>();
  const { data: containers, isLoading, error } = useContainers(hostName ?? '');

  if (!hostName) {
    return <p className="text-red-400">No host specified</p>;
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <Link to="/" className="text-zinc-500 hover:text-zinc-300 transition-colors">
          &larr; Dashboard
        </Link>
        <span className="text-zinc-700">/</span>
        <h1 className="text-2xl font-bold">{hostName}</h1>
      </div>

      {isLoading && (
        <div className="animate-pulse space-y-3">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="h-12 bg-zinc-800 rounded" />
          ))}
        </div>
      )}

      {error && (
        <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center">
          <p className="text-red-400 font-medium">Failed to load containers</p>
          <p className="text-red-400/60 text-sm mt-1">{error.message}</p>
        </div>
      )}

      {containers && containers.length === 0 && (
        <p className="text-zinc-500 text-center py-12">
          No containers found on this host.
        </p>
      )}

      {containers && containers.length > 0 && (
        <div className="overflow-x-auto rounded-xl border border-zinc-800">
          <table className="w-full text-left">
            <thead className="bg-zinc-900 text-zinc-400 text-sm">
              <tr>
                <th className="px-4 py-3 font-medium">Name</th>
                <th className="px-4 py-3 font-medium">Image</th>
                <th className="px-4 py-3 font-medium">State</th>
                <th className="px-4 py-3 font-medium">Ports</th>
                <th className="px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {containers.map((c) => (
                <ContainerRow key={c.id} container={c} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
