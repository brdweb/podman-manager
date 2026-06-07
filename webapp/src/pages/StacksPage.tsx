import { useStacks } from '../hooks/useControl';

export function StacksPage() {
  const { data, isLoading, error } = useStacks();
  const stacks = data?.stacks ?? [];

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium uppercase tracking-[0.18em] text-emerald-400">
          Docker Compose
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight">Stacks</h1>
      </div>
      <section className="rounded-lg border border-zinc-800 bg-zinc-900 p-6">
        {isLoading ? (
          <p className="text-zinc-400">Loading stacks...</p>
        ) : error ? (
          <p className="text-red-300">{error.message}</p>
        ) : data?.status === 'unavailable' ? (
          <div>
            <h2 className="text-lg font-semibold text-zinc-100">Stack inventory unavailable</h2>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-500">{data.message}</p>
            <p className="mt-3 font-mono text-xs text-zinc-600">{data.source_root}</p>
          </div>
        ) : stacks.length ? (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h2 className="text-lg font-semibold text-zinc-100">{data?.count ?? stacks.length} Compose stacks</h2>
              <span className="font-mono text-xs text-zinc-500">{data?.source_root}</span>
            </div>
            <div className="grid gap-3 lg:grid-cols-2">
              {stacks.map((stack) => (
                <article key={stack.relative_path} className="rounded-lg border border-zinc-800 bg-zinc-950 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-500">{stack.host}</p>
                      <h3 className="mt-1 text-base font-semibold text-zinc-100">{stack.name}</h3>
                    </div>
                    <span className="rounded-full bg-zinc-800 px-2.5 py-1 text-xs text-zinc-300">
                      {stack.service_count} services
                    </span>
                  </div>
                  <p className="mt-3 font-mono text-xs text-zinc-600">{stack.relative_path}</p>
                  <div className="mt-4 flex flex-wrap gap-2">
                    {stack.services.map((service) => (
                      <span key={service.name} className="rounded-md bg-zinc-900 px-2 py-1 text-xs text-zinc-300">
                        {service.name}
                      </span>
                    ))}
                  </div>
                </article>
              ))}
            </div>
          </div>
        ) : (
          <div>
            <h2 className="text-lg font-semibold text-zinc-100">Agent-backed stack inventory</h2>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-500">
              The v1 API is in place. Once Docker agents are enrolled, this view will show Git-backed
              Compose projects, pending diffs, deploy history, and stack actions.
            </p>
          </div>
        )}
      </section>
    </div>
  );
}
