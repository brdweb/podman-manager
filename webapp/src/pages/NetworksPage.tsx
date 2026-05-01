import { useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useCreateNetwork, useNetworks, useRemoveNetwork } from '../hooks/useContainers';
import type { CreateNetworkPayload, Network } from '../types/api';

interface LabelEntry {
  id: number;
  key: string;
  value: string;
}

const emptyLabel = (id: number): LabelEntry => ({ id, key: '', value: '' });

export function NetworksPage() {
  const { hostId } = useParams<{ hostId: string }>();
  const host = hostId ?? '';
  const { data: networks, isLoading, error } = useNetworks(host);
  const createNetwork = useCreateNetwork();
  const removeNetwork = useRemoveNetwork();
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<Network | null>(null);
  const [notice, setNotice] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [createError, setCreateError] = useState<string | null>(null);
  const [removeError, setRemoveError] = useState<string | null>(null);

  if (!hostId) {
    return <p className="text-red-400">No host specified</p>;
  }

  const handleCreate = async (payload: CreateNetworkPayload) => {
    setNotice(null);
    setCreateError(null);
    try {
      await createNetwork.mutateAsync({ host, payload });
      setIsCreateModalOpen(false);
      setNotice({ type: 'success', message: `Created network ${payload.name}.` });
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Failed to create network.');
    }
  };

  const handleRemove = async () => {
    if (!removeTarget) return;
    setNotice(null);
    setRemoveError(null);
    try {
      await removeNetwork.mutateAsync({ host, name: removeTarget.name });
      setNotice({ type: 'success', message: `Removed network ${removeTarget.name}.` });
      setRemoveTarget(null);
    } catch (err) {
      setRemoveError(err instanceof Error ? err.message : 'Failed to remove network.');
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
            <span className="text-zinc-500">Networks</span>
          </div>
          <h1 className="text-2xl font-bold">Networks</h1>
          <p className="mt-1 text-sm text-zinc-500">
            Manage Podman networks and address ranges on this host.
          </p>
        </div>

        <button
          type="button"
          onClick={() => setIsCreateModalOpen(true)}
          className="rounded-xl bg-zinc-100 px-4 py-2.5 text-sm font-medium text-zinc-900 transition-colors hover:bg-white"
        >
          Create Network
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
          <p className="font-medium text-red-400">Failed to load networks</p>
          <p className="mt-1 text-sm text-red-400/60">{error.message}</p>
        </div>
      )}

      {networks && (
        <NetworkTable
          networks={networks}
          isRemoving={removeNetwork.isPending}
          onRemove={(network) => {
            setRemoveError(null);
            setRemoveTarget(network);
          }}
        />
      )}

      {isCreateModalOpen && (
        <CreateNetworkModal
          isPending={createNetwork.isPending}
          error={createError}
          onClose={() => {
            setCreateError(null);
            setIsCreateModalOpen(false);
          }}
          onSubmit={(payload) => void handleCreate(payload)}
        />
      )}

      {removeTarget && (
        <DeleteNetworkDialog
          network={removeTarget}
          isPending={removeNetwork.isPending}
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

function NetworkTable({
  networks,
  isRemoving,
  onRemove,
}: {
  networks: Network[];
  isRemoving: boolean;
  onRemove: (network: Network) => void;
}) {
  if (networks.length === 0) {
    return <p className="py-12 text-center text-zinc-500">No networks exist on this host.</p>;
  }

  return (
    <div className="overflow-x-auto rounded-2xl border border-zinc-800 bg-zinc-950/70">
      <table className="w-full text-left">
        <thead className="bg-zinc-900/80 text-sm text-zinc-400">
          <tr>
            <th className="px-4 py-3 font-medium">Name</th>
            <th className="px-4 py-3 font-medium">Driver</th>
            <th className="px-4 py-3 font-medium">Subnet</th>
            <th className="px-4 py-3 font-medium">Internal</th>
            <th className="px-4 py-3 font-medium">Labels</th>
            <th className="px-4 py-3 font-medium text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {networks.map((network) => (
            <tr key={network.name} className="border-b border-zinc-800/80 transition-colors hover:bg-zinc-900/60">
              <td className="px-4 py-3">
                <span className="font-medium text-zinc-100">{network.name}</span>
              </td>
              <td className="px-4 py-3 text-sm text-zinc-300">{network.driver || '-'}</td>
              <td className="px-4 py-3 text-sm text-zinc-400">
                <SubnetList network={network} />
              </td>
              <td className="px-4 py-3">
                <span
                  className={`inline-flex rounded-full border px-2.5 py-0.5 text-xs font-medium ${
                    network.internal
                      ? 'border-amber-500/30 bg-amber-500/15 text-amber-400'
                      : 'border-zinc-500/30 bg-zinc-500/15 text-zinc-400'
                  }`}
                >
                  {network.internal ? 'Yes' : 'No'}
                </span>
              </td>
              <td className="px-4 py-3 text-xs text-zinc-400">
                <LabelList labels={network.labels} />
              </td>
              <td className="px-4 py-3 text-right">
                <button
                  type="button"
                  onClick={() => onRemove(network)}
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

function SubnetList({ network }: { network: Network }) {
  if (!network.subnets?.length) {
    return <span className="text-zinc-600">-</span>;
  }

  return (
    <div className="space-y-1">
      {network.subnets.map((entry) => (
        <p key={`${entry}-${network.gateway ?? ''}`} className="font-mono">
          <span className="text-zinc-300">{entry}</span>
          {network.gateway && <span className="ml-2 text-zinc-600">gw {network.gateway}</span>}
        </p>
      ))}
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

function CreateNetworkModal({
  isPending,
  error,
  onClose,
  onSubmit,
}: {
  isPending: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (payload: CreateNetworkPayload) => void;
}) {
  const [name, setName] = useState('');
  const [driver, setDriver] = useState('bridge');
  const [subnet, setSubnet] = useState('');
  const [gateway, setGateway] = useState('');
  const [internal, setInternal] = useState(false);
  const [labels, setLabels] = useState<LabelEntry[]>(() => [emptyLabel(1)]);
  const [nextLabelId, setNextLabelId] = useState(2);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) return;

    const payload: CreateNetworkPayload = {
      name: trimmedName,
      driver: driver.trim() || 'bridge',
      internal,
    };

    const trimmedSubnet = subnet.trim();
    const trimmedGateway = gateway.trim();
    if (trimmedSubnet) {
      payload.subnets = [trimmedSubnet];
    }
    if (trimmedGateway) {
      payload.gateway = trimmedGateway;
    }

    const labelRecord = labels.reduce<Record<string, string>>((acc, entry) => {
      const key = entry.key.trim();
      if (key) {
        acc[key] = entry.value.trim();
      }
      return acc;
    }, {});

    if (Object.keys(labelRecord).length > 0) {
      payload.labels = labelRecord;
    }

    onSubmit(payload);
  };

  const addLabel = () => {
    setLabels((current) => [...current, emptyLabel(nextLabelId)]);
    setNextLabelId((current) => current + 1);
  };

  const updateLabel = (id: number, field: 'key' | 'value', value: string) => {
    setLabels((current) =>
      current.map((entry) => (entry.id === id ? { ...entry, [field]: value } : entry))
    );
  };

  const removeLabel = (id: number) => {
    setLabels((current) => (current.length === 1 ? [emptyLabel(nextLabelId)] : current.filter((entry) => entry.id !== id)));
    if (labels.length === 1) {
      setNextLabelId((current) => current + 1);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="w-full max-w-2xl rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
        <h2 className="text-xl font-bold text-zinc-100">Create Network</h2>
        <p className="mt-2 text-sm text-zinc-400">
          Define a Podman network for this host. Subnet and gateway are optional unless you need a fixed address range.
        </p>

        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <TextField
              id="network-name"
              label="Name"
              value={name}
              onChange={setName}
              placeholder="app_net"
              required
            />
            <TextField
              id="network-driver"
              label="Driver"
              value={driver}
              onChange={setDriver}
              placeholder="bridge"
            />
            <TextField
              id="network-subnet"
              label="Subnet"
              value={subnet}
              onChange={setSubnet}
              placeholder="172.20.0.0/16"
            />
            <TextField
              id="network-gateway"
              label="Gateway"
              value={gateway}
              onChange={setGateway}
              placeholder="172.20.0.1"
            />
          </div>

          <label className="flex cursor-pointer items-center gap-2 text-sm text-zinc-300">
            <input
              type="checkbox"
              checked={internal}
              onChange={(event) => setInternal(event.target.checked)}
              className="h-4 w-4 rounded border-zinc-700 bg-zinc-950 text-emerald-500"
            />
            Internal network
          </label>

          <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-4">
            <div className="mb-3 flex items-center justify-between gap-4">
              <p className="text-sm font-medium text-zinc-300">Labels</p>
              <button
                type="button"
                onClick={addLabel}
                className="rounded-md bg-zinc-800 px-3 py-1.5 text-xs font-medium text-zinc-300 transition-colors hover:bg-zinc-700"
              >
                Add Label
              </button>
            </div>
            <div className="space-y-2">
              {labels.map((label) => (
                <div key={label.id} className="grid gap-2 md:grid-cols-[1fr_1fr_auto]">
                  <input
                    type="text"
                    value={label.key}
                    onChange={(event) => updateLabel(label.id, 'key', event.target.value)}
                    placeholder="key"
                    className="w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
                    aria-label="Label key"
                  />
                  <input
                    type="text"
                    value={label.value}
                    onChange={(event) => updateLabel(label.id, 'value', event.target.value)}
                    placeholder="value"
                    className="w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
                    aria-label="Label value"
                  />
                  <button
                    type="button"
                    onClick={() => removeLabel(label.id)}
                    className="rounded-xl px-3 py-2 text-sm font-medium text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-red-400"
                  >
                    Remove
                  </button>
                </div>
              ))}
            </div>
          </div>

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
              {isPending ? 'Creating...' : 'Create Network'}
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

function DeleteNetworkDialog({
  network,
  isPending,
  error,
  onClose,
  onConfirm,
}: {
  network: Network;
  isPending: boolean;
  error: string | null;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
        <h3 className="text-lg font-medium text-zinc-100">Delete Network</h3>
        <p className="mt-2 text-sm text-zinc-400">
          Delete <span className="font-mono text-zinc-300">{network.name}</span>? Containers attached to this network may lose connectivity.
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
