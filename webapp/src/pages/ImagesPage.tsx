import { useMemo, useState } from 'react';
import { useAllImages, useImageActions } from '../hooks/useImages';
import { useOverview } from '../hooks/useHosts';

export function ImagesPage() {
  const { data: images, isLoading, error } = useAllImages();
  const { data: overview } = useOverview();
  const { pull, remove, prune } = useImageActions();
  const [filter, setFilter] = useState('');
  const [isPullModalOpen, setIsPullModalOpen] = useState(false);
  const [pullImageRef, setPullImageRef] = useState('');
  const [pullHost, setPullHost] = useState('');
  const [notice, setNotice] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [pullError, setPullError] = useState<string | null>(null);
  const [removeTarget, setRemoveTarget] = useState<{ host: string; id: string; label: string } | null>(null);
  const [forceRemove, setForceRemove] = useState(false);
  const [removeError, setRemoveError] = useState<string | null>(null);
  const [isPruneDialogOpen, setIsPruneDialogOpen] = useState(false);
  const [pruneError, setPruneError] = useState<string | null>(null);

  const hosts = overview?.hosts.map((h) => h.name) || [];

  const filteredImages = useMemo(() => {
    if (!images) return [];
    const query = filter.trim().toLowerCase();
    if (!query) return images;
    return images.filter((image) => {
      return (
        image.repository.toLowerCase().includes(query) ||
        image.tag.toLowerCase().includes(query) ||
        (image.host && image.host.toLowerCase().includes(query)) ||
        image.id.toLowerCase().includes(query)
      );
    });
  }, [images, filter]);

  const handlePull = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!pullHost || !pullImageRef) return;
    setNotice(null);
    setPullError(null);
    try {
      await pull.mutateAsync({ host: pullHost, imageRef: pullImageRef });
      setIsPullModalOpen(false);
      setPullImageRef('');
      setNotice({ type: 'success', message: `Pull started for ${pullImageRef} on ${pullHost}.` });
    } catch (err) {
      setPullError(err instanceof Error ? err.message : 'Failed to pull image.');
    }
  };

  const handleRemove = async () => {
    if (!removeTarget) return;
    setNotice(null);
    setRemoveError(null);
    try {
      await remove.mutateAsync({ host: removeTarget.host, id: removeTarget.id, force: forceRemove });
      setNotice({ type: 'success', message: `Removed image ${removeTarget.label}.` });
      setRemoveTarget(null);
      setForceRemove(false);
    } catch (err) {
      setRemoveError(err instanceof Error ? err.message : 'Failed to remove image.');
    }
  };

  const handlePrune = async () => {
    setNotice(null);
    setPruneError(null);
    const failures: string[] = [];
    for (const host of hosts) {
      try {
        await prune.mutateAsync({ host });
      } catch (err) {
        failures.push(`${host}: ${err instanceof Error ? err.message : 'failed'}`);
      }
    }
    if (failures.length > 0) {
      setPruneError(`Prune completed with errors: ${failures.join('; ')}`);
      return;
    }

    setIsPruneDialogOpen(false);
    setNotice({ type: 'success', message: 'Pruned unused images on all configured hosts.' });
  };

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-2xl font-bold">Images</h1>
          <p className="mt-1 text-sm text-zinc-500">
            Manage container images across all configured hosts.
          </p>
        </div>

        <div className="flex flex-col gap-4 md:flex-row md:items-end">
          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
              Filter
            </span>
            <input
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
              placeholder="Search by repository, tag, or host"
              className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600 md:w-64"
            />
          </label>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setIsPullModalOpen(true)}
              className="rounded-xl bg-zinc-100 px-4 py-2.5 text-sm font-medium text-zinc-900 transition-colors hover:bg-white"
            >
              Pull Image
            </button>
            <button
              type="button"
              onClick={() => setIsPruneDialogOpen(true)}
              disabled={prune.isPending}
              className="rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm font-medium text-zinc-300 transition-colors hover:bg-zinc-800 disabled:opacity-50"
            >
              {prune.isPending ? 'Pruning...' : 'Prune Unused'}
            </button>
          </div>
        </div>
      </div>

      {notice && (
        <div
          className={`mb-6 rounded-xl border p-4 text-sm ${
            notice.type === 'success'
              ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300'
              : 'border-red-500/30 bg-red-500/10 text-red-300'
          }`}
        >
          {notice.message}
        </div>
      )}

      {isLoading && (
        <div className="animate-pulse space-y-3">
          {[1, 2, 3, 4].map((item) => (
            <div key={item} className="h-14 rounded-xl bg-zinc-800" />
          ))}
        </div>
      )}

      {error && (
        <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center">
          <p className="text-red-400 font-medium">Failed to load images</p>
          <p className="text-red-400/60 text-sm mt-1">{error.message}</p>
        </div>
      )}

      {images && (
        <div className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900/50">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-zinc-800 bg-zinc-900/50 text-xs uppercase tracking-wider text-zinc-500">
                <tr>
                  <th className="px-4 py-3 font-medium">Repository</th>
                  <th className="px-4 py-3 font-medium">Tag</th>
                  <th className="px-4 py-3 font-medium">Size</th>
                  <th className="px-4 py-3 font-medium">Created</th>
                  <th className="px-4 py-3 font-medium">Host</th>
                  <th className="px-4 py-3 font-medium text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-800/50">
                {filteredImages.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-zinc-500">
                      No images found.
                    </td>
                  </tr>
                ) : (
                  filteredImages.map((image) => (
                    <tr key={`${image.host}-${image.id}`} className="transition-colors hover:bg-zinc-800/30">
                      <td className="px-4 py-3 font-medium text-zinc-200">
                        {image.repository === '<none>' ? (
                          <span className="text-zinc-500">&lt;none&gt;</span>
                        ) : (
                          image.repository
                        )}
                      </td>
                      <td className="px-4 py-3 text-zinc-400">
                        {image.tag === '<none>' ? (
                          <span className="text-zinc-600">&lt;none&gt;</span>
                        ) : (
                          image.tag
                        )}
                      </td>
                      <td className="px-4 py-3 text-zinc-400">{image.size || '-'}</td>
                      <td className="px-4 py-3 text-zinc-400">{image.created_ago || '-'}</td>
                      <td className="px-4 py-3 text-zinc-400">
                        <span className="inline-flex items-center rounded-md bg-zinc-800 px-2 py-1 text-xs font-medium text-zinc-300">
                          {image.host}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          onClick={() => {
                            setForceRemove(false);
                            setRemoveTarget({
                              host: image.host!,
                              id: image.id,
                              label: `${image.repository}:${image.tag}`,
                            });
                          }}
                          disabled={remove.isPending}
                          className="rounded-md px-2 py-1 text-xs font-medium text-red-400 transition-colors hover:bg-red-400/10 disabled:opacity-50"
                        >
                          Remove
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {isPullModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
            <h2 className="text-xl font-bold text-zinc-100">Pull Image</h2>
            <p className="mt-2 text-sm text-zinc-400">
              Enter the image reference to pull from a registry.
            </p>

            <form onSubmit={handlePull} className="mt-6 space-y-4">
              <div>
                <label htmlFor="host" className="mb-1.5 block text-sm font-medium text-zinc-300">
                  Target Host
                </label>
                <select
                  id="host"
                  value={pullHost}
                  onChange={(e) => setPullHost(e.target.value)}
                  required
                  className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors focus:border-zinc-600"
                >
                  <option value="" disabled>Select a host</option>
                  {hosts.map((host) => (
                    <option key={host} value={host}>
                      {host}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label htmlFor="imageRef" className="mb-1.5 block text-sm font-medium text-zinc-300">
                  Image Reference
                </label>
                <input
                  id="imageRef"
                  type="text"
                  value={pullImageRef}
                  onChange={(e) => setPullImageRef(e.target.value)}
                  placeholder="e.g., docker.io/library/nginx:latest"
                  required
                  className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
                />
              </div>

              {pullError && (
                <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
                  {pullError}
                </div>
              )}

              <div className="mt-6 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsPullModalOpen(false)}
                  className="rounded-xl px-4 py-2.5 text-sm font-medium text-zinc-400 transition-colors hover:text-zinc-200"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={pull.isPending || !pullHost || !pullImageRef}
                  className="rounded-xl bg-zinc-100 px-4 py-2.5 text-sm font-medium text-zinc-900 transition-colors hover:bg-white disabled:opacity-50"
                >
                  {pull.isPending ? 'Pulling...' : 'Pull Image'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {removeTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
            <h2 className="text-xl font-bold text-zinc-100">Remove Image</h2>
            <p className="mt-2 text-sm text-zinc-400">
              Remove <span className="font-mono text-zinc-200">{removeTarget.label}</span> from{' '}
              <span className="font-mono text-zinc-200">{removeTarget.host}</span>?
            </p>
            <label className="mt-4 flex cursor-pointer items-center gap-2 text-sm text-zinc-300">
              <input
                type="checkbox"
                checked={forceRemove}
                onChange={(event) => setForceRemove(event.target.checked)}
                className="h-4 w-4 rounded border-zinc-700 bg-zinc-950 text-red-500"
              />
              Force remove if the image is in use
            </label>
            {removeError && (
              <div className="mt-4 rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
                {removeError}
              </div>
            )}
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setRemoveTarget(null)}
                disabled={remove.isPending}
                className="rounded-xl px-4 py-2.5 text-sm font-medium text-zinc-400 transition-colors hover:text-zinc-200 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => void handleRemove()}
                disabled={remove.isPending}
                className="rounded-xl bg-red-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-red-500 disabled:opacity-50"
              >
                {remove.isPending ? 'Removing...' : 'Remove'}
              </button>
            </div>
          </div>
        </div>
      )}

      {isPruneDialogOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
            <h2 className="text-xl font-bold text-zinc-100">Prune Unused Images</h2>
            <p className="mt-2 text-sm text-zinc-400">
              Prune unused images on all configured hosts? This may remove images that are not currently referenced by containers.
            </p>
            {pruneError && (
              <div className="mt-4 rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
                {pruneError}
              </div>
            )}
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setIsPruneDialogOpen(false)}
                disabled={prune.isPending}
                className="rounded-xl px-4 py-2.5 text-sm font-medium text-zinc-400 transition-colors hover:text-zinc-200 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => void handlePrune()}
                disabled={prune.isPending}
                className="rounded-xl bg-zinc-100 px-4 py-2.5 text-sm font-medium text-zinc-900 transition-colors hover:bg-white disabled:opacity-50"
              >
                {prune.isPending ? 'Pruning...' : 'Prune'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
