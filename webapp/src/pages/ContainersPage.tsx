import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAllContainers } from '../hooks/useContainers';
import { useOverview } from '../hooks/useHosts';
import { ContainerTable } from '../components/ContainerTable';

export function ContainersPage() {
  const { data: containers, isLoading, error } = useAllContainers();
  const { data: overview } = useOverview();
  const [filter, setFilter] = useState('');
  const [createHost, setCreateHost] = useState('');

  const hostOptions = useMemo(() => {
    const names = new Set<string>();
    overview?.hosts.forEach((host) => names.add(host.name));
    containers?.forEach((container) => names.add(container.host));
    return Array.from(names).sort((a, b) => a.localeCompare(b));
  }, [containers, overview]);

  const selectedCreateHost = createHost || hostOptions[0] || '';

  const filteredContainers = useMemo(() => {
    if (!containers) return [];
    const query = filter.trim().toLowerCase();
    if (!query) return containers;
    return containers.filter((container) => {
      return (
        container.name.toLowerCase().includes(query) ||
        container.image.toLowerCase().includes(query) ||
        container.host.toLowerCase().includes(query) ||
        container.state.toLowerCase().includes(query)
      );
    });
  }, [containers, filter]);

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-2xl font-bold">All Containers</h1>
          <p className="mt-1 text-sm text-zinc-500">
            Live state, CPU, memory, ports, and mounts across every configured host.
          </p>
        </div>

        <div className="flex flex-col gap-4 md:flex-row md:items-end">
          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
              Filter
            </span>
            <input
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
              placeholder="Search by container, image, host, or state"
              className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600 md:w-80"
            />
          </label>

          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
              Host
            </span>
            <select
              value={selectedCreateHost}
              onChange={(event) => setCreateHost(event.target.value)}
              className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors focus:border-zinc-600 md:w-44"
            >
              {hostOptions.length === 0 ? (
                <option value="">No hosts</option>
              ) : (
                hostOptions.map((host) => (
                  <option key={host} value={host}>
                    {host}
                  </option>
                ))
              )}
            </select>
          </label>

          <Link
            to={
              selectedCreateHost
                ? `/hosts/${encodeURIComponent(selectedCreateHost)}/containers/create`
                : '#'
            }
            aria-disabled={!selectedCreateHost}
            className={`rounded-xl bg-blue-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 ${
              selectedCreateHost ? '' : 'pointer-events-none opacity-50'
            }`}
          >
            Create
          </Link>
        </div>
      </div>

      {isLoading && (
        <div className="animate-pulse space-y-3">
          {[1, 2, 3, 4].map((item) => (
            <div key={item} className="h-14 rounded-xl bg-zinc-800" />
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
        <ContainerTable
          containers={filteredContainers}
          showHost
          emptyMessage="No containers matched the current filter."
        />
      )}
    </div>
  );
}
