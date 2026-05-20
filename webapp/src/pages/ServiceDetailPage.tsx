import type { ReactNode } from 'react';
import { useParams } from 'react-router-dom';
import { useLinks } from '../hooks/useControl';
import { resolveIconSrc } from '../lib/icons';

export function ServiceDetailPage() {
  const { serviceId } = useParams();
  const { data, isLoading } = useLinks();
  const service = data?.links.find((link) => link.id === serviceId);

  if (isLoading) {
    return <div className="text-zinc-400">Loading service...</div>;
  }
  if (!service) {
    return <div className="text-zinc-400">Service not found.</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <img
          src={resolveIconSrc(service.icon, service.url)}
          alt=""
          className="h-14 w-14 rounded-lg object-contain"
        />
        <div>
          <p className="text-sm font-medium uppercase tracking-[0.18em] text-emerald-400">
            {service.category_name}
          </p>
          <h1 className="mt-1 text-3xl font-semibold tracking-tight">{service.title}</h1>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <Panel title="Route">
          <a href={service.url} target="_blank" rel="noreferrer" className="text-emerald-300">
            {service.url}
          </a>
          {service.health_url && (
            <p className="mt-3 text-sm text-zinc-500">Health: {service.health_url}</p>
          )}
        </Panel>
        <Panel title="Status">
          <p className="text-2xl font-semibold text-zinc-100">{service.status}</p>
          <p className="mt-2 text-sm text-zinc-500">Source: {service.source || 'manual'}</p>
        </Panel>
        <Panel title="Admin Actions">
          <p className="text-sm text-zinc-500">
            Container, Compose, metrics, logs, and backup actions attach here as Docker agents come
            online.
          </p>
        </Panel>
      </div>
    </div>
  );
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded-lg border border-zinc-800 bg-zinc-900 p-5">
      <h2 className="mb-3 text-sm font-medium uppercase tracking-wide text-zinc-500">{title}</h2>
      {children}
    </section>
  );
}
