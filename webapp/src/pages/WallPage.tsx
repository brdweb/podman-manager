import { useControlOverview } from '../hooks/useControl';
import { useOverview } from '../hooks/useHosts';

export function WallPage() {
  const { data } = useOverview();
  const { data: control } = useControlOverview();
  const hosts = data?.hosts ?? [];
  const online = hosts.filter((host) => host.status === 'online').length;
  const containers = hosts.reduce((sum, host) => sum + host.container_count.total, 0);
  const running = hosts.reduce((sum, host) => sum + host.container_count.running, 0);

  return (
    <div className="min-h-[calc(100vh-8rem)] space-y-6">
      <div>
        <p className="text-sm font-medium uppercase tracking-[0.18em] text-emerald-400">Wall</p>
        <h1 className="mt-2 text-4xl font-semibold tracking-tight">Homelab at a glance</h1>
      </div>
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        <WallCard label="Hosts Online" value={`${online}/${hosts.length}`} />
        <WallCard label="Running Containers" value={`${running}/${containers}`} />
        <WallCard label="Launchpad Links" value={control?.links ?? '-'} />
        <WallCard label="Alerts" value="0" />
      </div>
    </div>
  );
}

function WallCard({ label, value }: { label: string; value: string | number }) {
  return (
    <section className="rounded-lg border border-zinc-800 bg-zinc-900 p-8">
      <p className="text-sm uppercase tracking-wide text-zinc-500">{label}</p>
      <p className="mt-4 text-5xl font-semibold text-zinc-100">{value}</p>
    </section>
  );
}
