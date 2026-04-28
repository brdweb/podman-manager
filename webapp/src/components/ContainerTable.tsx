import { useMemo, useState, type ReactNode } from 'react';
import type { Container } from '../types/api';
import { useContainerAction, useContainerDetail, useCheckUpdate, useUpdateContainer } from '../hooks/useContainers';
import { StatusBadge } from './StatusBadge';
import { formatBytes, formatPercent, formatTimestamp } from '../lib/format';
import { LogViewer } from './LogViewer';

interface ContainerTableProps {
  containers: Container[];
  showHost?: boolean;
  emptyMessage?: string;
}

type SortKey = 'name' | 'host' | 'state' | 'cpu' | 'memory';

export function ContainerTable({
  containers,
  showHost = false,
  emptyMessage = 'No containers found.',
}: ContainerTableProps) {
  const { start, stop, restart } = useContainerAction();
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  const [viewingLogsFor, setViewingLogsFor] = useState<{host: string, id: string, name: string} | null>(null);
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(() => new Set());
  const [sort, setSort] = useState<{ key: SortKey; direction: 'asc' | 'desc' }>({
    key: 'name',
    direction: 'asc',
  });
  const [bulkError, setBulkError] = useState<string | null>(null);
  const [bulkAction, setBulkAction] = useState<'start' | 'stop' | 'restart' | null>(null);

  const sortedContainers = useMemo(() => {
    const direction = sort.direction === 'asc' ? 1 : -1;
    return [...containers].sort((a, b) => compareContainers(a, b, sort.key) * direction);
  }, [containers, sort]);

  const selectedContainers = sortedContainers.filter((container) =>
    selectedKeys.has(containerKey(container))
  );
  const allVisibleSelected = sortedContainers.length > 0 && selectedContainers.length === sortedContainers.length;
  const isBulkPending = bulkAction !== null;

  function handleSort(key: SortKey) {
    setSort((current) => ({
      key,
      direction: current.key === key && current.direction === 'asc' ? 'desc' : 'asc',
    }));
  }

  function toggleAllVisible(checked: boolean) {
    setSelectedKeys((current) => {
      const next = new Set(current);
      for (const container of sortedContainers) {
        const key = containerKey(container);
        if (checked) {
          next.add(key);
        } else {
          next.delete(key);
        }
      }
      return next;
    });
  }

  function toggleContainer(container: Container, checked: boolean) {
    setSelectedKeys((current) => {
      const next = new Set(current);
      const key = containerKey(container);
      if (checked) {
        next.add(key);
      } else {
        next.delete(key);
      }
      return next;
    });
  }

  async function runBulkAction(action: 'start' | 'stop' | 'restart') {
    setBulkError(null);
    setBulkAction(action);

    try {
      const mutation = action === 'start' ? start : action === 'stop' ? stop : restart;
      for (const container of selectedContainers) {
        await mutation.mutateAsync({ host: container.host, id: container.id });
      }
      setSelectedKeys(new Set());
    } catch (error) {
      setBulkError(error instanceof Error ? error.message : `Failed to ${action} selected containers`);
    } finally {
      setBulkAction(null);
    }
  }

  if (containers.length === 0) {
    return <p className="text-zinc-500 text-center py-12">{emptyMessage}</p>;
  }

  return (
    <div className="overflow-x-auto rounded-2xl border border-zinc-800 bg-zinc-950/70">
      {selectedContainers.length > 0 && (
        <div className="flex flex-col gap-3 border-b border-zinc-800 bg-zinc-900/70 px-4 py-3 text-sm text-zinc-300 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <span className="font-medium text-zinc-100">{selectedContainers.length}</span> selected
            {bulkError && <span className="ml-3 text-red-400">{bulkError}</span>}
          </div>
          <div className="flex flex-wrap gap-2">
            <BulkButton label="Start" disabled={isBulkPending} active={bulkAction === 'start'} onClick={() => void runBulkAction('start')} />
            <BulkButton label="Stop" disabled={isBulkPending} active={bulkAction === 'stop'} onClick={() => void runBulkAction('stop')} />
            <BulkButton label="Restart" disabled={isBulkPending} active={bulkAction === 'restart'} onClick={() => void runBulkAction('restart')} />
            <button
              type="button"
              onClick={() => setSelectedKeys(new Set())}
              disabled={isBulkPending}
              className="rounded-md px-3 py-1.5 text-xs font-medium text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-200 disabled:opacity-50"
            >
              Clear
            </button>
          </div>
        </div>
      )}
      <table className="w-full text-left">
        <thead className="bg-zinc-900/80 text-zinc-400 text-sm">
          <tr>
            <th className="w-10 px-4 py-3">
              <input
                type="checkbox"
                checked={allVisibleSelected}
                onChange={(event) => toggleAllVisible(event.target.checked)}
                className="h-4 w-4 rounded border-zinc-700 bg-zinc-950 text-emerald-500"
                aria-label="Select all visible containers"
              />
            </th>
            <SortableHeader label="Container" sortKey="name" current={sort} onSort={handleSort} />
            {showHost && <SortableHeader label="Host" sortKey="host" current={sort} onSort={handleSort} />}
            <SortableHeader label="State" sortKey="state" current={sort} onSort={handleSort} />
            <SortableHeader label="CPU" sortKey="cpu" current={sort} onSort={handleSort} />
            <SortableHeader label="Memory" sortKey="memory" current={sort} onSort={handleSort} />
            <th className="px-4 py-3 font-medium">Network / Ports</th>
            <th className="px-4 py-3 font-medium">Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedContainers.map((container) => {
            const key = containerKey(container);
            const isExpanded = expandedKey === key;
            return (
              <ContainerTableRow
                key={key}
                container={container}
                showHost={showHost}
                isExpanded={isExpanded}
                isSelected={selectedKeys.has(key)}
                onSelectedChange={(checked) => toggleContainer(container, checked)}
                onToggle={() => setExpandedKey(isExpanded ? null : key)}
                onViewLogs={() => setViewingLogsFor({ host: container.host, id: container.id, name: container.name })}
              />
            );
          })}
        </tbody>
      </table>

      {viewingLogsFor && (
        <LogViewer
          host={viewingLogsFor.host}
          containerId={viewingLogsFor.id}
          containerName={viewingLogsFor.name}
          onClose={() => setViewingLogsFor(null)}
        />
      )}
    </div>
  );
}

function containerKey(container: Container): string {
  return `${container.host}/${container.id}`;
}

function compareContainers(a: Container, b: Container, key: SortKey): number {
  switch (key) {
    case 'host':
      return a.host.localeCompare(b.host) || a.name.localeCompare(b.name);
    case 'state':
      return a.state.localeCompare(b.state) || a.name.localeCompare(b.name);
    case 'cpu':
      return (a.stats?.cpu_percent ?? -1) - (b.stats?.cpu_percent ?? -1);
    case 'memory':
      return (a.stats?.memory_usage_bytes ?? -1) - (b.stats?.memory_usage_bytes ?? -1);
    case 'name':
    default:
      return a.name.localeCompare(b.name) || a.host.localeCompare(b.host);
  }
}

function SortableHeader({
  label,
  sortKey,
  current,
  onSort,
}: {
  label: string;
  sortKey: SortKey;
  current: { key: SortKey; direction: 'asc' | 'desc' };
  onSort: (key: SortKey) => void;
}) {
  const active = current.key === sortKey;
  const icon = !active ? '↕' : current.direction === 'asc' ? '↑' : '↓';

  return (
    <th className="px-4 py-3 font-medium">
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        className="inline-flex items-center gap-1 text-left transition-colors hover:text-zinc-100"
      >
        <span>{label}</span>
        <span className={active ? 'text-emerald-400' : 'text-zinc-600'}>{icon}</span>
      </button>
    </th>
  );
}

function BulkButton({
  label,
  disabled,
  active,
  onClick,
}: {
  label: string;
  disabled: boolean;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="rounded-md bg-zinc-100 px-3 py-1.5 text-xs font-medium text-zinc-900 transition-colors hover:bg-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {active ? `${label}...` : label}
    </button>
  );
}

interface ContainerTableRowProps {
  container: Container;
  showHost: boolean;
  isExpanded: boolean;
  isSelected: boolean;
  onSelectedChange: (checked: boolean) => void;
  onToggle: () => void;
  onViewLogs: () => void;
}

function ContainerTableRow({
  container,
  showHost,
  isExpanded,
  isSelected,
  onSelectedChange,
  onToggle,
  onViewLogs,
}: ContainerTableRowProps) {
  const { start, stop, restart, remove } = useContainerAction();
  const updateMutation = useUpdateContainer();
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isUpdateDialogOpen, setIsUpdateDialogOpen] = useState(false);
  const isRunning = container.state === 'running';
  const colSpan = showHost ? 8 : 7;

  const { data: updateCheck } = useCheckUpdate(container.host, container.id, isRunning);
  const hasUpdate = updateCheck?.update_available;

  const handleDelete = (force: boolean) => {
    remove.mutate(
      { host: container.host, id: container.id, force },
      {
        onSuccess: () => {
          setIsDeleteDialogOpen(false);
        },
      }
    );
  };

  const handleUpdate = () => {
    updateMutation.mutate(
      { host: container.host, id: container.id },
      {
        onSuccess: () => {
          setIsUpdateDialogOpen(false);
        },
      }
    );
  };

  return (
    <>
      <tr
        className="border-b border-zinc-800/80 hover:bg-zinc-900/60 transition-colors cursor-pointer"
        onClick={onToggle}
      >
        <td className="px-4 py-3" onClick={(event) => event.stopPropagation()}>
          <input
            type="checkbox"
            checked={isSelected}
            onChange={(event) => onSelectedChange(event.target.checked)}
            className="h-4 w-4 rounded border-zinc-700 bg-zinc-950 text-emerald-500"
            aria-label={`Select ${container.name}`}
          />
        </td>
        <td className="px-4 py-3">
          <div className="group flex items-center gap-2">
            <span className="font-medium text-zinc-100 group-hover:text-white">
              {container.name}
            </span>
            {hasUpdate && (
              <span
                className="inline-flex items-center rounded-full bg-orange-500/10 px-1.5 py-0.5 text-[10px] font-medium text-orange-400 ring-1 ring-inset ring-orange-500/20"
                title="Update available"
              >
                Update Available
              </span>
            )}
            <span className="text-xs uppercase tracking-[0.2em] text-zinc-500">
              {container.manager}
            </span>
          </div>
          <p className="mt-1 max-w-80 truncate text-xs font-mono text-zinc-500">
            {container.image}
          </p>
        </td>
        {showHost && (
          <td className="px-4 py-3 text-sm text-zinc-400">{container.host}</td>
        )}
        <td className="px-4 py-3">
          <StatusBadge status={container.state} />
        </td>
        <td className="px-4 py-3 text-sm text-zinc-300">
          {formatPercent(container.stats?.cpu_percent)}
        </td>
        <td className="px-4 py-3 text-sm text-zinc-300">
          {container.stats?.memory_usage_bytes
            ? `${formatBytes(container.stats.memory_usage_bytes)} / ${formatBytes(container.stats.memory_limit_bytes)}`
            : '-'}
        </td>
        <td className="px-4 py-3 text-xs text-zinc-400">
          <div className="space-y-1">
            <div>{formatNetworks(container)}</div>
            <div className="font-mono">{formatPorts(container)}</div>
          </div>
        </td>
        <td className="px-4 py-3">
          <div className="flex flex-wrap gap-1.5 items-center">
             {isRunning ? (
               <>
                 <ActionButton
                   label="Logs"
                   onClick={(e) => {
                     e.stopPropagation();
                     onViewLogs();
                   }}
                   disabled={false}
                   variant="neutral"
                 />
                 <ActionButton
                   label="Stop"
                   onClick={(e) => {
                     e.stopPropagation();
                     stop.mutate({ host: container.host, id: container.id });
                   }}
                   disabled={stop.isPending}
                   variant="danger"
                 />
                 <ActionButton
                   label="Restart"
                   onClick={(e) => {
                     e.stopPropagation();
                     restart.mutate({ host: container.host, id: container.id });
                   }}
                   disabled={restart.isPending}
                   variant="neutral"
                 />
                 {hasUpdate && (
                   <ActionButton
                     label="Update"
                     onClick={(e) => {
                       e.stopPropagation();
                       setIsUpdateDialogOpen(true);
                     }}
                     disabled={updateMutation.isPending}
                     variant="warning"
                   />
                 )}
               </>
             ) : (
               <>
                 <ActionButton
                   label="Logs"
                   onClick={(e) => {
                     e.stopPropagation();
                     onViewLogs();
                   }}
                   disabled={false}
                   variant="neutral"
                 />
                 <ActionButton
                   label="Start"
                   onClick={(e) => {
                     e.stopPropagation();
                     start.mutate({ host: container.host, id: container.id });
                   }}
                   disabled={start.isPending}
                   variant="success"
                 />
               </>
             )}
             <button
               type="button"
               onClick={(e) => {
                 e.stopPropagation();
                 setIsDeleteDialogOpen(true);
               }}
               className="ml-1 rounded p-1 text-zinc-500 hover:bg-zinc-800 hover:text-red-400 transition-colors"
               title="Delete container"
             >
               <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                 <title>Delete container</title>
                 <path d="M3 6h18"></path>
                 <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"></path>
                 <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"></path>
               </svg>
             </button>
          </div>
        </td>
      </tr>

      {isExpanded && (
        <ContainerExpandedRow
          host={container.host}
          id={container.id}
          colSpan={colSpan}
        />
      )}

      <DeleteDialog
        isOpen={isDeleteDialogOpen}
        onClose={() => setIsDeleteDialogOpen(false)}
        onConfirm={handleDelete}
        containerName={container.name}
        isPending={remove.isPending}
      />

      <UpdateDialog
        isOpen={isUpdateDialogOpen}
        onClose={() => setIsUpdateDialogOpen(false)}
        onConfirm={handleUpdate}
        containerName={container.name}
        isPending={updateMutation.isPending}
      />
    </>
  );
}

function ContainerExpandedRow({
  host,
  id,
  colSpan,
}: {
  host: string;
  id: string;
  colSpan: number;
}) {
  const { data, isLoading, error } = useContainerDetail(host, id);

  return (
    <tr className="border-b border-zinc-800/80 bg-zinc-900/60">
      <td colSpan={colSpan} className="px-4 py-4">
        {isLoading && <p className="text-sm text-zinc-400">Loading container details...</p>}
        {error && <p className="text-sm text-red-400">{error.message}</p>}
        {data && (
          <div className="grid gap-4 lg:grid-cols-3">
            <div className="lg:col-span-1">
              <DetailCard title="Runtime">
                <DetailLine label="Hostname" value={data.hostname || '-'} />
                <DetailLine label="Restart" value={data.restart_policy || '-'} />
                <DetailLine label="PID" value={data.pid ? String(data.pid) : '-'} />
                <DetailLine label="Started" value={formatTimestamp(data.started_at)} />
                <DetailLine label="Finished" value={formatTimestamp(data.finished_at)} />
                <DetailLine
                  label="Memory"
                  value={
                    data.stats?.memory_usage_bytes
                      ? `${formatBytes(data.stats.memory_usage_bytes)} / ${formatBytes(data.stats.memory_limit_bytes)} (${formatPercent(data.stats.memory_percent)})`
                      : '-'
                  }
                />
              </DetailCard>
            </div>

            <div className="lg:col-span-2">
              <DetailCard title="Networking">
                {data.networks?.length ? (
                  data.networks.map((network) => (
                    <p key={network.name} className="text-sm text-zinc-300">
                      <span className="font-medium text-zinc-100">{network.name}</span>
                      <span className="ml-2 text-zinc-500">{network.ip || 'no IP'}</span>
                      {network.gateway && (
                        <span className="ml-2 text-zinc-600">gw {network.gateway}</span>
                      )}
                    </p>
                  ))
                ) : (
                  <p className="text-sm text-zinc-500">No network details.</p>
                )}

                {data.labels && Object.keys(data.labels).length > 0 && (
                  <div className="mt-4 border-t border-zinc-800 pt-4">
                    <p className="mb-2 text-xs uppercase tracking-[0.2em] text-zinc-500">Labels</p>
                    <div className="space-y-1">
                      {Object.entries(data.labels).map(([key, value]) => (
                        <p key={key} className="break-all text-sm text-zinc-400">
                          <span className="font-mono text-zinc-200">{key}</span>
                          <span className="mx-2 text-zinc-700">=</span>
                          <span className="font-mono text-zinc-500">{value}</span>
                        </p>
                      ))}
                    </div>
                  </div>
                )}
              </DetailCard>
            </div>

            <div className="lg:col-span-3">
              <DetailCard title="Volumes">
                {data.mounts?.length ? (
                  data.mounts.map((mount) => (
                    <p key={`${mount.source}-${mount.destination}`} className="text-sm text-zinc-300">
                      <span className="font-mono text-zinc-100">{mount.destination}</span>
                      <span className="mx-2 text-zinc-600">&larr;</span>
                      <span className="font-mono text-zinc-500">{mount.source}</span>
                      <span className="ml-2 text-xs uppercase tracking-[0.2em] text-zinc-500">
                        {mount.rw ? 'rw' : 'ro'}
                      </span>
                    </p>
                  ))
                ) : (
                  <p className="text-sm text-zinc-500">No mounted volumes.</p>
                )}
              </DetailCard>
            </div>
          </div>
        )}
      </td>
    </tr>
  );
}

function formatPorts(container: Container): string {
  if (!container.ports?.length) return 'No published ports';
  return container.ports
    .map((p) => {
      const host = p.host_ip && p.host_ip !== '0.0.0.0' ? `${p.host_ip}:` : '';
      return `${host}${p.host_port}:${p.container_port}/${p.protocol}`;
    })
    .join(', ');
}

function formatNetworks(container: Container): string {
  if (!container.networks?.length) return 'No networks';
  return container.networks
    .map((network) => (network.ip ? `${network.name} (${network.ip})` : network.name))
    .join(', ');
}

function DetailCard({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-950/80 p-4">
      <p className="mb-3 text-xs uppercase tracking-[0.24em] text-zinc-500">{title}</p>
      <div className="space-y-2">{children}</div>
    </div>
  );
}

function DetailLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4 text-sm">
      <span className="text-zinc-500">{label}</span>
      <span className="text-right text-zinc-200">{value}</span>
    </div>
  );
}

interface ActionButtonProps {
  label: string;
  onClick: (e: React.MouseEvent<HTMLButtonElement>) => void;
  disabled: boolean;
  variant: 'success' | 'danger' | 'neutral' | 'warning';
}

const variantStyles: Record<ActionButtonProps['variant'], string> = {
  success: 'bg-emerald-600 hover:bg-emerald-500 text-white',
  danger: 'bg-red-600 hover:bg-red-500 text-white',
  neutral: 'bg-zinc-700 hover:bg-zinc-600 text-zinc-200',
  warning: 'bg-orange-600 hover:bg-orange-500 text-white',
};

function ActionButton({ label, onClick, disabled, variant }: ActionButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`rounded px-2.5 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${variantStyles[variant]}`}
    >
      {label}
    </button>
  );
}

function DeleteDialog({
  isOpen,
  onClose,
  onConfirm,
  containerName,
  isPending,
}: {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (force: boolean) => void;
  containerName: string;
  isPending: boolean;
}) {
  const [force, setForce] = useState(false);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
        <h3 className="text-lg font-medium text-zinc-100">Delete Container</h3>
        <p className="mt-2 text-sm text-zinc-400">
          Are you sure you want to delete <span className="font-mono text-zinc-300">{containerName}</span>?
          This action cannot be undone.
        </p>
        
        <label className="mt-4 flex items-center gap-2 text-sm text-zinc-300 cursor-pointer">
          <input
            type="checkbox"
            checked={force}
            onChange={(e) => setForce(e.target.checked)}
            className="rounded border-zinc-700 bg-zinc-900 text-red-600 focus:ring-red-600 focus:ring-offset-zinc-950"
          />
          Force delete (running container)
        </label>

        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            disabled={isPending}
            className="rounded-lg px-4 py-2 text-sm font-medium text-zinc-300 hover:bg-zinc-900 hover:text-white transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => onConfirm(force)}
            disabled={isPending}
            className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-500 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {isPending ? 'Deleting...' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  );
}

function UpdateDialog({
  isOpen,
  onClose,
  onConfirm,
  containerName,
  isPending,
}: {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  containerName: string;
  isPending: boolean;
}) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
        <h3 className="text-lg font-medium text-zinc-100">Update Container</h3>
        <p className="mt-2 text-sm text-zinc-400">
          Are you sure you want to update <span className="font-mono text-zinc-300">{containerName}</span>?
          This will pull the latest image and recreate the container.
        </p>

        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            disabled={isPending}
            className="rounded-lg px-4 py-2 text-sm font-medium text-zinc-300 hover:bg-zinc-900 hover:text-white transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={isPending}
            className="rounded-lg bg-orange-600 px-4 py-2 text-sm font-medium text-white hover:bg-orange-500 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {isPending ? 'Updating...' : 'Update'}
          </button>
        </div>
      </div>
    </div>
  );
}
