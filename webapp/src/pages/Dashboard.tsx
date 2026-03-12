import { useOverview } from '../hooks/useHosts';
import { HostCard } from '../components/HostCard';

export function Dashboard() {
  const { data, isLoading, error } = useOverview();

  if (isLoading) {
    return <LoadingSkeleton />;
  }

  if (error) {
    return (
      <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center">
        <p className="text-red-400 font-medium">Failed to load overview</p>
        <p className="text-red-400/60 text-sm mt-1">{error.message}</p>
      </div>
    );
  }

  const hosts = data?.hosts ?? [];
  const totalContainers = hosts.reduce((sum, h) => sum + h.container_count.total, 0);
  const totalRunning = hosts.reduce((sum, h) => sum + h.container_count.running, 0);
  const onlineHosts = hosts.filter((h) => h.status === 'online').length;

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Dashboard</h1>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        <SummaryCard label="Hosts Online" value={`${onlineHosts}/${hosts.length}`} />
        <SummaryCard label="Containers" value={totalContainers} />
        <SummaryCard label="Running" value={totalRunning} accent="text-emerald-400" />
      </div>

      <h2 className="text-lg font-semibold mb-4 text-zinc-300">Hosts</h2>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {hosts.map((host) => (
          <HostCard key={host.name} host={host} />
        ))}
      </div>
    </div>
  );
}

function SummaryCard({
  label,
  value,
  accent = 'text-zinc-100',
}: {
  label: string;
  value: string | number;
  accent?: string;
}) {
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-5">
      <p className="text-sm text-zinc-500 mb-1">{label}</p>
      <p className={`text-3xl font-bold ${accent}`}>{value}</p>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="animate-pulse">
      <div className="h-8 w-40 bg-zinc-800 rounded mb-6" />
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-24 bg-zinc-800 rounded-xl" />
        ))}
      </div>
      <div className="h-6 w-24 bg-zinc-800 rounded mb-4" />
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-40 bg-zinc-800 rounded-xl" />
        ))}
      </div>
    </div>
  );
}
