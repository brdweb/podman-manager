import { Link } from 'react-router-dom';
import { useLinks } from '../hooks/useControl';
import { resolveIconSrc } from '../lib/icons';

export function LaunchpadPage() {
  const { data, isLoading, error } = useLinks();

  if (isLoading) {
    return <div className="text-zinc-400">Loading launchpad...</div>;
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-5 text-red-300">
        Failed to load links: {error.message}
      </div>
    );
  }

  const groups = data?.groups ?? [];

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-sm font-medium uppercase tracking-[0.18em] text-emerald-400">
            Launchpad
          </p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight">
            Homelab links and services
          </h1>
        </div>
        <Link
          to="/launchpad/admin"
          className="rounded-md border border-zinc-700 px-4 py-2 text-sm text-zinc-200 transition-colors hover:border-emerald-500 hover:text-emerald-200"
        >
          Manage links
        </Link>
      </div>

      {groups.length === 0 ? (
        <div className="rounded-lg border border-dashed border-zinc-700 bg-zinc-900/70 p-8 text-center text-zinc-400">
          No links yet. Import Homepage or create the first link from Manage links.
        </div>
      ) : (
        groups.map((group) => (
          <section key={group.category.id} className="space-y-3">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold text-zinc-200">{group.category.name}</h2>
              <span className="text-xs text-zinc-500">{group.links.length} links</span>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
              {group.links.map((link) => (
                <a
                  key={link.id}
                  href={link.url}
                  target={link.target}
                  rel={link.target === '_blank' ? 'noreferrer' : undefined}
                  className="group flex min-h-24 items-center gap-4 rounded-lg border border-zinc-800 bg-zinc-900 p-4 transition-colors hover:border-emerald-500/70 hover:bg-zinc-900/80"
                >
                  <img
                    src={resolveIconSrc(link.icon, link.url)}
                    alt=""
                    className="h-11 w-11 shrink-0 rounded-md object-contain"
                    loading="lazy"
                  />
                  <span className="min-w-0">
                    <span className="block truncate font-medium text-zinc-100 group-hover:text-emerald-100">
                      {link.title}
                    </span>
                    {link.description ? (
                      <span className="mt-1 line-clamp-2 block text-sm leading-5 text-zinc-500">
                        {link.description}
                      </span>
                    ) : (
                      <span className="mt-1 block truncate text-sm text-zinc-600">
                        {new URL(link.url).hostname}
                      </span>
                    )}
                  </span>
                </a>
              ))}
            </div>
          </section>
        ))
      )}
    </div>
  );
}
