import { Link } from 'react-router-dom';
import type { HostStatus } from '../types/api';
import { StatusBadge } from './StatusBadge';
import { formatBytes, formatUptime } from '../lib/format';

interface HostCardProps {
  host: HostStatus;
}

export function HostCard({ host }: HostCardProps) {
  return (
    <Link
      to={`/hosts/${encodeURIComponent(host.name)}`}
      className="block rounded-xl border border-zinc-800 bg-zinc-900 p-5 hover:border-zinc-600 transition-colors"
    >
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-lg font-semibold text-zinc-100">{host.name}</h3>
        <StatusBadge status={host.status} />
      </div>

      <p className="text-sm text-zinc-500 mb-4">{host.address}</p>

      {host.status === 'online' && (
        <div className="grid grid-cols-3 gap-3">
          <Stat label="Total" value={host.container_count.total} />
          <Stat label="Running" value={host.container_count.running} color="text-emerald-400" />
          <Stat label="Stopped" value={host.container_count.stopped} color="text-zinc-400" />
        </div>
      )}

      {host.system && (
        <div className="mt-4 grid gap-2 border-t border-zinc-800 pt-4 text-xs text-zinc-500">
          <p>{host.system.os || 'Unknown OS'}</p>
          <p>
            CPU {host.system.cpu_cores ?? '-'} cores
            {host.system.load_1 !== undefined ? ` • load ${host.system.load_1.toFixed(2)}` : ''}
          </p>
          <p>
            Memory{' '}
            {host.system.memory_used_bytes
              ? `${formatBytes(host.system.memory_used_bytes)} / ${formatBytes(host.system.memory_total_bytes)}`
              : '-'}
          </p>
          <p>Uptime {formatUptime(host.system.uptime_seconds)}</p>
        </div>
      )}

      {host.error && (
        <p className="text-sm text-red-400 mt-2">{host.error}</p>
      )}

      {host.latency && (
        <p className="text-xs text-zinc-600 mt-3">Latency: {host.latency}</p>
      )}
    </Link>
  );
}

interface StatProps {
  label: string;
  value: number;
  color?: string;
}

function Stat({ label, value, color = 'text-zinc-100' }: StatProps) {
  return (
    <div className="text-center">
      <p className={`text-2xl font-bold ${color}`}>{value}</p>
      <p className="text-xs text-zinc-500">{label}</p>
    </div>
  );
}
