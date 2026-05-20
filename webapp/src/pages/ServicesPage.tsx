import { Link } from 'react-router-dom';
import { useLinks } from '../hooks/useControl';
import { resolveIconSrc } from '../lib/icons';

export function ServicesPage() {
  const { data, isLoading, error } = useLinks();

  if (isLoading) {
    return <div className="text-zinc-400">Loading service catalog...</div>;
  }
  if (error) {
    return <div className="text-red-300">{error.message}</div>;
  }

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium uppercase tracking-[0.18em] text-emerald-400">
          Services
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight">Service catalog</h1>
      </div>
      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        {data?.links.map((link) => (
          <Link
            key={link.id}
            to={`/services/${encodeURIComponent(link.id)}`}
            className="flex items-center justify-between gap-4 rounded-lg border border-zinc-800 bg-zinc-900 p-4 hover:border-emerald-500/70"
          >
            <span className="flex min-w-0 items-center gap-3">
              <img
                src={resolveIconSrc(link.icon, link.url)}
                alt=""
                className="h-10 w-10 rounded object-contain"
              />
              <span className="min-w-0">
                <span className="block truncate font-medium text-zinc-100">{link.title}</span>
                <span className="block truncate text-sm text-zinc-500">{link.category_name}</span>
              </span>
            </span>
            <span className="rounded-full bg-zinc-800 px-2 py-1 text-xs text-zinc-400">
              {link.status}
            </span>
          </Link>
        ))}
      </div>
    </div>
  );
}
