import type { HostStatus } from '../types/api';
import { formatBytes, formatPercent, formatUptime } from '../lib/format';

export function HostSystemCard({ host }: { host: HostStatus }) {
  if (!host.system) {
    return null;
  }

  const memoryPercent =
    host.system.memory_total_bytes && host.system.memory_used_bytes
      ? (host.system.memory_used_bytes / host.system.memory_total_bytes) * 100
      : undefined;
  const diskPercent =
    host.system.disk_total_bytes && host.system.disk_used_bytes
      ? (host.system.disk_used_bytes / host.system.disk_total_bytes) * 100
      : undefined;

  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-4">
      <div className="mb-3 flex items-center justify-between gap-4">
        <div>
          <p className="text-sm font-semibold text-zinc-100">
            {host.system.hostname || host.name}
          </p>
          <p className="text-xs text-zinc-500">
            {host.system.os || 'Unknown OS'} {host.system.kernel ? `• ${host.system.kernel}` : ''}
          </p>
        </div>
        <p className="text-xs uppercase tracking-[0.24em] text-zinc-500">
          {formatUptime(host.system.uptime_seconds)}
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Metric
          label="CPU"
          value={host.system.cpu_cores ? `${host.system.cpu_cores} cores` : '-'}
          hint={
            host.system.load_1 !== undefined
              ? `load ${host.system.load_1.toFixed(2)} / ${host.system.load_5?.toFixed(2) ?? '0.00'} / ${host.system.load_15?.toFixed(2) ?? '0.00'}`
              : undefined
          }
        />
        <Metric
          label="Memory"
          value={
            host.system.memory_used_bytes
              ? `${formatBytes(host.system.memory_used_bytes)} / ${formatBytes(host.system.memory_total_bytes)}`
              : '-'
          }
          hint={formatPercent(memoryPercent)}
        />
        <Metric
          label="Disk"
          value={
            host.system.disk_used_bytes
              ? `${formatBytes(host.system.disk_used_bytes)} / ${formatBytes(host.system.disk_total_bytes)}`
              : '-'
          }
          hint={formatPercent(diskPercent)}
        />
        <Metric label="Latency" value={host.latency ?? '-'} />
      </div>
    </div>
  );
}

function Metric({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-950/70 p-3">
      <p className="text-xs uppercase tracking-[0.2em] text-zinc-500">{label}</p>
      <p className="mt-1 text-sm font-medium text-zinc-100">{value}</p>
      {hint && <p className="mt-1 text-xs text-zinc-500">{hint}</p>}
    </div>
  );
}
