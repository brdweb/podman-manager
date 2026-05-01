import { useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useCreateVolume, useRemoveVolume, useVolumes } from '../hooks/useContainers';
import { formatTimestamp } from '../lib/format';
import type { CreateVolumePayload, Volume } from '../types/api';

interface KeyValueEntry {
  id: number;
  key: string;
  value: string;
}

const emptyEntry = (id: number): KeyValueEntry => ({ id, key: '', value: '' });

export function VolumesPage() {
  const { hostId } = useParams<{ hostId: string }>();
  const host = hostId ?? '';
  const { data: volumes, isLoading, error } = useVolumes(host);
  const createVolume = useCreateVolume();
  const removeVolume = useRemoveVolume();
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<Volume | null>(null);
  const [notice, setNotice] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [createError, setCreateError] = useState<string | null>(null);
  const [removeError, setRemoveError] = useState<string | null>(null);

  if (!hostId) {
    return <p className="text-red-400">No host specified</p>;
  }

  const handleCreate = async (payload: CreateVolumePayload) => {
    setNotice(null);
    setCreateError(null);
    try {
      await createVolume.mutateAsync({ host, payload });
      setIsCreateModalOpen(false);
      setNotice({ type: 'success', message: `Created volume ${payload.name}.` });
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Failed to create volume.');
    }
  };

  const handleRemove = async () => {
    if (!removeTarget) return;
    setNotice(null);
    setRemoveError(null);
    try {
      await removeVolume.mutateAsync({ host, name: removeTarget.name });
      setNotice({ type: 'success', message: `Removed volume ${removeTarget.name}.` });
      setRemoveTarget(null);
    } catch (err) {
      setRemoveError(err instanceof Error ? err.message : 'Failed to remove volume.');
    }
  };

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <div className="mb-2 flex items-center gap-3 text-sm">
            <Link to={`/hosts/${encodeURIComponent(hostId)}`} className="text-zinc-500 transition-colors hover:text-zinc-300">
              &larr; {hostId}
            </Link>
            <span className="text-zinc-700">/</span>
            <span className="text-zinc-500">Volumes</span>
          </div>
          <h1 className="text-2xl font-bold">Volumes</h1>
          <p className="mt-1 text-sm text-zinc-500">
            Manage persistent Podman volumes and driver metadata on this host.
          </p>
        </div>

        <button
          type="button"
          onClick={() => setIsCreateModalOpen(true)}
          className="rounded-xl bg-zinc-100 px-4 py-2.5 text-sm font-medium text-zinc-900 transition-colors hover:bg-white"
        >
          Create Volume
        </button>
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
          <p className="font-medium text-red-400">Failed to load volumes</p>
          <p className="mt-1 text-sm text-red-400/60">{error.message}</p>
        </div>
      )}

      {volumes && (
        <VolumeTable
          volumes={volumes}
          isRemoving={removeVolume.isPending}
          onRemove={(volume) => {
            setRemoveError(null);
            setRemoveTarget(volume);
          }}
        />
      )}

      {isCreateModalOpen && (
        <CreateVolumeModal
          isPending={createVolume.isPending}
          error={createError}
          onClose={() => {
            setCreateError(null);
            setIsCreateModalOpen(false);
          }}
          onSubmit={(payload) => void handleCreate(payload)}
        />
      )}

      {removeTarget && (
        <DeleteVolumeDialog
          volume={removeTarget}
          isPending={removeVolume.isPending}
          error={removeError}
          onClose={() => {
            setRemoveError(null);
            setRemoveTarget(null);
          }}
          onConfirm={() => void handleRemove()}
        />
      )}
    </div>
  );
}

function VolumeTable({
  volumes,
  isRemoving,
  onRemove,
}: {
  volumes: Volume[];
  isRemoving: boolean;
  onRemove: (volume: Volume) => void;
}) {
  if (volumes.length === 0) {
    return <p className="py-12 text-center text-zinc-500">No volumes exist on this host.</p>;
  }

  return (
    <div className="overflow-x-auto rounded-2xl border border-zinc-800 bg-zinc-950/70">
      <table className="w-full text-left">
        <thead className="bg-zinc-900/80 text-sm text-zinc-400">
          <tr>
            <th className="px-4 py-3 font-medium">Name</th>
            <th className="px-4 py-3 font-medium">Driver</th>
            <th className="px-4 py-3 font-medium">Labels</th>
            <th className="px-4 py-3 font-medium">Created</th>
            <th className="px-4 py-3 font-medium text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {volumes.map((volume) => (
            <tr key={volume.name} className="border-b border-zinc-800/80 transition-colors hover:bg-zinc-900/60">
              <td className="px-4 py-3">
                <span className="font-medium text-zinc-100">{volume.name}</span>
                {volume.mountpoint && (
                  <p className="mt-1 max-w-sm truncate font-mono text-xs text-zinc-500">
                    {volume.mountpoint}
                  </p>
                )}
              </td>
              <td className="px-4 py-3 text-sm text-zinc-300">{volume.driver || '-'}</td>
              <td className="px-4 py-3 text-xs text-zinc-400">
                <LabelList labels={volume.labels} />
              </td>
              <td className="px-4 py-3 text-sm text-zinc-400">{formatTimestamp(volume.createdAt)}</td>
              <td className="px-4 py-3 text-right">
                <button
                  type="button"
                  onClick={() => onRemove(volume)}
                  disabled={isRemoving}
                  className="rounded-md px-2 py-1 text-xs font-medium text-red-400 transition-colors hover:bg-red-400/10 disabled:opacity-50"
                >
                  Delete
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function LabelList({ labels }: { labels?: Record<string, string> }) {
  const entries = Object.entries(labels ?? {});
  if (entries.length === 0) {
    return <span className="text-zinc-600">-</span>;
  }

  return (
    <div className="max-w-sm space-y-1">
      {entries.map(([key, value]) => (
        <p key={key} className="break-all">
          <span className="font-mono text-zinc-200">{key}</span>
          <span className="mx-2 text-zinc-700">=</span>
          <span className="font-mono text-zinc-500">{value}</span>
        </p>
      ))}
    </div>
  );
}

function CreateVolumeModal({
  isPending,
  error,
  onClose,
  onSubmit,
}: {
  isPending: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (payload: CreateVolumePayload) => void;
}) {
  const [name, setName] = useState('');
  const [driver, setDriver] = useState('local');
  const [labels, setLabels] = useState<KeyValueEntry[]>(() => [emptyEntry(1)]);
  const [options, setOptions] = useState<KeyValueEntry[]>(() => [emptyEntry(1)]);
  const [nextLabelId, setNextLabelId] = useState(2);
  const [nextOptionId, setNextOptionId] = useState(2);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) return;

    const payload: CreateVolumePayload = {
      name: trimmedName,
      driver: driver.trim() || 'local',
    };
    const labelRecord = entriesToRecord(labels);
    const optionRecord = entriesToRecord(options);

    if (Object.keys(labelRecord).length > 0) {
      payload.labels = labelRecord;
    }
    if (Object.keys(optionRecord).length > 0) {
      payload.options = optionRecord;
    }

    onSubmit(payload);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
        <h2 className="text-xl font-bold text-zinc-100">Create Volume</h2>
        <p className="mt-2 text-sm text-zinc-400">
          Define a Podman volume for this host. Labels and driver options are optional.
        </p>

        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <TextField
              id="volume-name"
              label="Name"
              value={name}
              onChange={setName}
              placeholder="app_data"
              required
            />
            <TextField
              id="volume-driver"
              label="Driver"
              value={driver}
              onChange={setDriver}
              placeholder="local"
            />
          </div>

          <KeyValueEditor
            title="Labels"
            entries={labels}
            nextId={nextLabelId}
            onEntriesChange={setLabels}
            onNextIdChange={setNextLabelId}
          />
          <KeyValueEditor
            title="Options"
            entries={options}
            nextId={nextOptionId}
            onEntriesChange={setOptions}
            onNextIdChange={setNextOptionId}
          />

          {error && (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
              {error}
            </div>
          )}

          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              disabled={isPending}
              className="rounded-xl px-4 py-2.5 text-sm font-medium text-zinc-400 transition-colors hover:text-zinc-200 disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isPending || !name.trim()}
              className="rounded-xl bg-zinc-100 px-4 py-2.5 text-sm font-medium text-zinc-900 transition-colors hover:bg-white disabled:opacity-50"
            >
              {isPending ? 'Creating...' : 'Create Volume'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function TextField({
  id,
  label,
  value,
  onChange,
  placeholder,
  required = false,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  required?: boolean;
}) {
  return (
    <div>
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-zinc-300">
        {label}
      </label>
      <input
        id={id}
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        required={required}
        className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
      />
    </div>
  );
}

function KeyValueEditor({
  title,
  entries,
  nextId,
  onEntriesChange,
  onNextIdChange,
}: {
  title: string;
  entries: KeyValueEntry[];
  nextId: number;
  onEntriesChange: (entries: KeyValueEntry[]) => void;
  onNextIdChange: (value: number | ((current: number) => number)) => void;
}) {
  const singularTitle = title.endsWith('s') ? title.slice(0, -1) : title;

  const addEntry = () => {
    onEntriesChange([...entries, emptyEntry(nextId)]);
    onNextIdChange((current) => current + 1);
  };

  const updateEntry = (id: number, field: 'key' | 'value', value: string) => {
    onEntriesChange(entries.map((entry) => (entry.id === id ? { ...entry, [field]: value } : entry)));
  };

  const removeEntry = (id: number) => {
    if (entries.length === 1) {
      onEntriesChange([emptyEntry(nextId)]);
      onNextIdChange((current) => current + 1);
      return;
    }
    onEntriesChange(entries.filter((entry) => entry.id !== id));
  };

  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-4">
      <div className="mb-3 flex items-center justify-between gap-4">
        <p className="text-sm font-medium text-zinc-300">{title}</p>
        <button
          type="button"
          onClick={addEntry}
          className="rounded-md bg-zinc-800 px-3 py-1.5 text-xs font-medium text-zinc-300 transition-colors hover:bg-zinc-700"
        >
          Add {singularTitle}
        </button>
      </div>
      <div className="space-y-2">
        {entries.map((entry) => (
          <div key={entry.id} className="grid gap-2 md:grid-cols-[1fr_1fr_auto]">
            <input
              type="text"
              value={entry.key}
              onChange={(event) => updateEntry(entry.id, 'key', event.target.value)}
              placeholder="key"
              className="w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
              aria-label={`${singularTitle} key`}
            />
            <input
              type="text"
              value={entry.value}
              onChange={(event) => updateEntry(entry.id, 'value', event.target.value)}
              placeholder="value"
              className="w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
              aria-label={`${singularTitle} value`}
            />
            <button
              type="button"
              onClick={() => removeEntry(entry.id)}
              className="rounded-xl px-3 py-2 text-sm font-medium text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-red-400"
            >
              Remove
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

function DeleteVolumeDialog({
  volume,
  isPending,
  error,
  onClose,
  onConfirm,
}: {
  volume: Volume;
  isPending: boolean;
  error: string | null;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
        <h3 className="text-lg font-medium text-zinc-100">Delete Volume</h3>
        <p className="mt-2 text-sm text-zinc-400">
          Delete <span className="font-mono text-zinc-300">{volume.name}</span>? Containers using this volume may lose persistent data.
        </p>
        {error && (
          <div className="mt-4 rounded-xl border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
            {error}
          </div>
        )}
        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            disabled={isPending}
            className="rounded-lg px-4 py-2 text-sm font-medium text-zinc-300 transition-colors hover:bg-zinc-900 hover:text-white disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={isPending}
            className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-500 disabled:opacity-50"
          >
            {isPending ? 'Deleting...' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  );
}

function entriesToRecord(entries: KeyValueEntry[]): Record<string, string> {
  return entries.reduce<Record<string, string>>((acc, entry) => {
    const key = entry.key.trim();
    if (key) {
      acc[key] = entry.value.trim();
    }
    return acc;
  }, {});
}
