interface StatusBadgeProps {
  status: string;
}

const statusStyles: Record<string, string> = {
  running: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
  online: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
  exited: 'bg-zinc-500/15 text-zinc-400 border-zinc-500/30',
  stopped: 'bg-zinc-500/15 text-zinc-400 border-zinc-500/30',
  offline: 'bg-red-500/15 text-red-400 border-red-500/30',
  error: 'bg-red-500/15 text-red-400 border-red-500/30',
  created: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
};

const fallback = 'bg-zinc-500/15 text-zinc-400 border-zinc-500/30';

export function StatusBadge({ status }: StatusBadgeProps) {
  const style = statusStyles[status.toLowerCase()] ?? fallback;

  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium ${style}`}
    >
      {status}
    </span>
  );
}
