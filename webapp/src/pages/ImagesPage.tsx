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
    try {
      await pull.mutateAsync({ host: pullHost, imageRef: pullImageRef });
      setIsPullModalOpen(false);
      setPullImageRef('');
    } catch (err) {
      console.error('Failed to pull image', err);
      alert('Failed to pull image. See console for details.');
    }
  };

  const handleRemove = async (host: string, id: string) => {
    if (confirm('Are you sure you want to remove this image?')) {
      try {
        await remove.mutateAsync({ host, id });
      } catch (err) {
        console.error('Failed to remove image', err);
        if (confirm('Failed to remove image. It might be in use. Force remove?')) {
          try {
            await remove.mutateAsync({ host, id, force: true });
          } catch (forceErr) {
            console.error('Failed to force remove image', forceErr);
            alert('Failed to force remove image.');
          }
        }
      }
    }
  };

  const handlePrune = async () => {
    if (confirm('Are you sure you want to prune unused images on all hosts?')) {
      for (const host of hosts) {
        try {
          await prune.mutateAsync({ host });
        } catch (err) {
          console.error(`Failed to prune images on ${host}`, err);
        }
      }
    }
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
              onClick={handlePrune}
              disabled={prune.isPending}
              className="rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm font-medium text-zinc-300 transition-colors hover:bg-zinc-800 disabled:opacity-50"
            >
              {prune.isPending ? 'Pruning...' : 'Prune Unused'}
            </button>
          </div>
        </div>
      </div>

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
                          onClick={() => handleRemove(image.host!, image.id)}
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
    </div>
  );
}
