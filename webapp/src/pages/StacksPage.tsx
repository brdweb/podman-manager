import { useStacks } from '../hooks/useControl';

export function StacksPage() {
  const { data, isLoading, error } = useStacks();

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
        ) : data?.stacks.length ? (
          <pre className="overflow-auto text-sm text-zinc-300">{JSON.stringify(data.stacks, null, 2)}</pre>
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
