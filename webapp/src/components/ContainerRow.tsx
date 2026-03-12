import type { Container } from '../types/api';
import { useContainerAction } from '../hooks/useContainers';
import { StatusBadge } from './StatusBadge';

interface ContainerRowProps {
  container: Container;
}

export function ContainerRow({ container }: ContainerRowProps) {
  const { start, stop, restart } = useContainerAction();
  const isRunning = container.state === 'running';

  function formatPorts(c: Container): string {
    if (!c.ports?.length) return '-';
    return c.ports
      .map((p) => {
        const host = p.host_ip && p.host_ip !== '0.0.0.0' ? p.host_ip + ':' : '';
        return `${host}${p.host_port}:${p.container_port}/${p.protocol}`;
      })
      .join(', ');
  }

  return (
    <tr className="border-b border-zinc-800 hover:bg-zinc-800/50 transition-colors">
      <td className="px-4 py-3 font-medium text-zinc-100">{container.name}</td>
      <td className="px-4 py-3 text-zinc-400 text-sm font-mono truncate max-w-60">
        {container.image}
      </td>
      <td className="px-4 py-3">
        <StatusBadge status={container.state} />
      </td>
      <td className="px-4 py-3 text-zinc-400 text-sm font-mono">
        {formatPorts(container)}
      </td>
      <td className="px-4 py-3">
        <div className="flex gap-1.5">
          {isRunning ? (
            <>
              <ActionButton
                label="Stop"
                onClick={() => stop.mutate({ host: container.host, id: container.id })}
                disabled={stop.isPending}
                variant="danger"
              />
              <ActionButton
                label="Restart"
                onClick={() => restart.mutate({ host: container.host, id: container.id })}
                disabled={restart.isPending}
                variant="neutral"
              />
            </>
          ) : (
            <ActionButton
              label="Start"
              onClick={() => start.mutate({ host: container.host, id: container.id })}
              disabled={start.isPending}
              variant="success"
            />
          )}
        </div>
      </td>
    </tr>
  );
}

interface ActionButtonProps {
  label: string;
  onClick: () => void;
  disabled: boolean;
  variant: 'success' | 'danger' | 'neutral';
}

const variantStyles: Record<string, string> = {
  success: 'bg-emerald-600 hover:bg-emerald-500 text-white',
  danger: 'bg-red-600 hover:bg-red-500 text-white',
  neutral: 'bg-zinc-700 hover:bg-zinc-600 text-zinc-200',
};

function ActionButton({ label, onClick, disabled, variant }: ActionButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`rounded px-2.5 py-1 text-xs font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${variantStyles[variant]}`}
    >
      {label}
    </button>
  );
}
