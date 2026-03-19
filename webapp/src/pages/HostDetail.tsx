import { useParams, Link } from 'react-router-dom';
import { useContainers } from '../hooks/useContainers';
import { useOverview } from '../hooks/useHosts';
import { ContainerTable } from '../components/ContainerTable';
import { HostSystemCard } from '../components/HostSystemCard';

export function HostDetail() {
  const { hostName } = useParams<{ hostName: string }>();
  const { data: containers, isLoading, error } = useContainers(hostName ?? '');
  const { data: overview } = useOverview();

  if (!hostName) {
    return <p className="text-red-400">No host specified</p>;
  }

  const host = overview?.hosts.find((entry) => entry.name === hostName);

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <Link to="/" className="text-zinc-500 hover:text-zinc-300 transition-colors">
          &larr; Dashboard
        </Link>
        <span className="text-zinc-700">/</span>
        <h1 className="text-2xl font-bold">{hostName}</h1>
      </div>

      {host && (
        <div className="mb-6">
          <HostSystemCard host={host} />
        </div>
      )}

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

      {containers && (
        <ContainerTable containers={containers} emptyMessage="No containers found on this host." />
      )}
    </div>
  );
}
