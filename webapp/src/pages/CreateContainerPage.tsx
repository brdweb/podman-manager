import { useState, type ReactNode } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useCreateContainer } from '../hooks/useContainers';
import type { CreateContainerPayload } from '../types/api';
import { useToast } from '../components/Toast';

type StepKey = 'basic' | 'networking' | 'storage' | 'advanced';

interface PortRow {
  hostPort: string;
  containerPort: string;
  protocol: string;
}

interface VolumeRow {
  volumeName: string;
  containerPath: string;
}

interface KeyValueRow {
  key: string;
  value: string;
}

const steps: Array<{ key: StepKey; label: string; description: string }> = [
  { key: 'basic', label: 'Basic', description: 'Name and image' },
  { key: 'networking', label: 'Networking', description: 'Network and ports' },
  { key: 'storage', label: 'Storage', description: 'Volume mounts' },
  { key: 'advanced', label: 'Advanced', description: 'Runtime options' },
];

const fieldClass =
  'w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600';

export function CreateContainerPage() {
  const { hostId } = useParams<{ hostId: string }>();
  const navigate = useNavigate();
  const createContainer = useCreateContainer();
  const { addToast } = useToast();
  const [stepIndex, setStepIndex] = useState(0);
  const [name, setName] = useState('');
  const [image, setImage] = useState('');
  const [network, setNetwork] = useState('bridge');
  const [ports, setPorts] = useState<PortRow[]>([]);
  const [volumes, setVolumes] = useState<VolumeRow[]>([]);
  const [envVars, setEnvVars] = useState<KeyValueRow[]>([]);
  const [labels, setLabels] = useState<KeyValueRow[]>([]);
  const [restartPolicy, setRestartPolicy] = useState('no');
  const [memoryMB, setMemoryMB] = useState('');
  const [cpuShares, setCpuShares] = useState('');
  const [formError, setFormError] = useState<string | null>(null);

  if (!hostId) {
    return <p className="text-red-400">No host specified</p>;
  }

  const host = hostId;
  const currentStep = steps[stepIndex];
  const isFinalStep = stepIndex === steps.length - 1;
  const canContinue = name.trim() !== '' && image.trim() !== '';

  function goNext() {
    setFormError(null);
    if (!canContinue) {
      setFormError('Container name and image are required.');
      setStepIndex(0);
      return;
    }
    setStepIndex((current) => Math.min(current + 1, steps.length - 1));
  }

  function goBack() {
    setFormError(null);
    setStepIndex((current) => Math.max(current - 1, 0));
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError(null);

    if (!canContinue) {
      setFormError('Container name and image are required.');
      setStepIndex(0);
      return;
    }

    const payload = buildPayload({
      name,
      image,
      network,
      ports,
      volumes,
      envVars,
      labels,
      restartPolicy,
      memoryMB,
      cpuShares,
    });

    try {
      const result = await createContainer.mutateAsync({ host, payload });
      addToast(result.message ?? `Container ${name.trim()} created on ${host}.`, 'success');
      window.setTimeout(() => {
        navigate(`/hosts/${encodeURIComponent(host)}`);
      }, 700);
    } catch (error) {
      addToast(error instanceof Error ? error.message : 'Failed to create container.', 'error');
    }
  }

  return (
    <div>
      <div className="mb-6 flex items-center gap-3">
        <Link
          to={`/hosts/${encodeURIComponent(host)}`}
          className="text-zinc-500 transition-colors hover:text-zinc-300"
        >
          &larr; {host}
        </Link>
        <span className="text-zinc-700">/</span>
        <h1 className="text-2xl font-bold">Create Container</h1>
      </div>

      <div className="grid gap-6 lg:grid-cols-[18rem,1fr]">
        <aside className="rounded-2xl border border-zinc-800 bg-zinc-900 p-4">
          <p className="mb-4 px-2 text-xs uppercase tracking-[0.2em] text-zinc-500">
            Wizard
          </p>
          <div className="space-y-2">
            {steps.map((step, index) => {
              const isActive = index === stepIndex;
              const isComplete = index < stepIndex;
              return (
                <button
                  key={step.key}
                  type="button"
                  onClick={() => setStepIndex(index)}
                  className={`w-full rounded-xl border px-4 py-3 text-left transition-colors ${
                    isActive
                      ? 'border-blue-500/60 bg-blue-500/10 text-white'
                      : isComplete
                        ? 'border-emerald-500/30 bg-emerald-500/10 text-zinc-200 hover:border-emerald-500/50'
                        : 'border-zinc-800 bg-zinc-950/60 text-zinc-400 hover:border-zinc-700 hover:text-zinc-200'
                  }`}
                >
                  <span className="block text-sm font-medium">{step.label}</span>
                  <span className="mt-1 block text-xs text-zinc-500">{step.description}</span>
                </button>
              );
            })}
          </div>
        </aside>

        <form onSubmit={handleSubmit} className="rounded-2xl border border-zinc-800 bg-zinc-900 p-6">
          <div className="mb-6 border-b border-zinc-800 pb-5">
            <p className="text-xs uppercase tracking-[0.2em] text-zinc-500">
              Step {stepIndex + 1} of {steps.length}
            </p>
            <h2 className="mt-2 text-xl font-semibold text-zinc-100">{currentStep.label}</h2>
            <p className="mt-1 text-sm text-zinc-500">Configure {currentStep.description.toLowerCase()} for this container.</p>
          </div>

          {currentStep.key === 'basic' && (
            <div className="space-y-5">
              <Field label="Container name" htmlFor="container-name">
                <input
                  id="container-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder="web-proxy"
                  required
                  className={fieldClass}
                />
              </Field>

              <Field label="Image" htmlFor="container-image">
                <input
                  id="container-image"
                  value={image}
                  onChange={(event) => setImage(event.target.value)}
                  placeholder="docker.io/library/nginx:latest"
                  required
                  className={fieldClass}
                />
              </Field>
            </div>
          )}

          {currentStep.key === 'networking' && (
            <div className="space-y-6">
              <Field label="Network" htmlFor="container-network">
                <input
                  id="container-network"
                  value={network}
                  onChange={(event) => setNetwork(event.target.value)}
                  placeholder="bridge"
                  className={fieldClass}
                />
              </Field>

              <PortMappings rows={ports} onChange={setPorts} />
            </div>
          )}

          {currentStep.key === 'storage' && (
            <VolumeMounts rows={volumes} onChange={setVolumes} />
          )}

          {currentStep.key === 'advanced' && (
            <div className="space-y-6">
              <KeyValueRows
                title="Environment variables"
                addLabel="Add variable"
                keyPlaceholder="KEY"
                valuePlaceholder="value"
                rows={envVars}
                onChange={setEnvVars}
              />

              <KeyValueRows
                title="Labels"
                addLabel="Add label"
                keyPlaceholder="com.example.label"
                valuePlaceholder="value"
                rows={labels}
                onChange={setLabels}
              />

              <div className="grid gap-4 md:grid-cols-3">
                <Field label="Restart policy" htmlFor="restart-policy">
                  <select
                    id="restart-policy"
                    value={restartPolicy}
                    onChange={(event) => setRestartPolicy(event.target.value)}
                    className={fieldClass}
                  >
                    <option value="no">no</option>
                    <option value="always">always</option>
                    <option value="on-failure">on-failure</option>
                    <option value="unless-stopped">unless-stopped</option>
                  </select>
                </Field>

                <Field label="Memory limit (MB)" htmlFor="memory-limit">
                  <input
                    id="memory-limit"
                    type="number"
                    min="1"
                    value={memoryMB}
                    onChange={(event) => setMemoryMB(event.target.value)}
                    placeholder="512"
                    className={fieldClass}
                  />
                </Field>

                <Field label="CPU shares" htmlFor="cpu-shares">
                  <input
                    id="cpu-shares"
                    type="number"
                    min="1"
                    value={cpuShares}
                    onChange={(event) => setCpuShares(event.target.value)}
                    placeholder="1024"
                    className={fieldClass}
                  />
                </Field>
              </div>
            </div>
          )}

          {formError && (
            <div className="mt-6 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-300">
              {formError}
            </div>
          )}

          <div className="mt-8 flex flex-col-reverse gap-3 border-t border-zinc-800 pt-6 sm:flex-row sm:items-center sm:justify-between">
            <button
              type="button"
              onClick={goBack}
              disabled={stepIndex === 0 || createContainer.isPending}
              className="rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm font-medium text-zinc-300 transition-colors hover:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Back
            </button>

            <div className="flex justify-end gap-3">
              <Link
                to={`/hosts/${encodeURIComponent(host)}`}
                className="rounded-xl px-4 py-2.5 text-sm font-medium text-zinc-400 transition-colors hover:text-zinc-200"
              >
                Cancel
              </Link>

              {isFinalStep ? (
                <button
                  type="submit"
                  disabled={createContainer.isPending || !canContinue}
                  className="rounded-xl bg-blue-600 px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {createContainer.isPending ? 'Creating...' : 'Create Container'}
                </button>
              ) : (
                <button
                  type="button"
                  onClick={goNext}
                  disabled={createContainer.isPending}
                  className="rounded-xl bg-blue-600 px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Continue
                </button>
              )}
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}

function PortMappings({ rows, onChange }: { rows: PortRow[]; onChange: (rows: PortRow[]) => void }) {
  return (
    <DynamicPanel
      title="Port mappings"
      description="Map host ports to ports exposed inside the container."
      addLabel="Add port"
      onAdd={() => onChange([...rows, { hostPort: '', containerPort: '', protocol: 'tcp' }])}
      isEmpty={rows.length === 0}
      emptyMessage="No ports will be published."
    >
      {rows.map((row, index) => (
        <div key={index} className="grid gap-3 rounded-xl border border-zinc-800 bg-zinc-950/60 p-4 md:grid-cols-[1fr,1fr,9rem,auto]">
          <input
            type="number"
            min="1"
            value={row.hostPort}
            onChange={(event) => updateRow(rows, onChange, index, { hostPort: event.target.value })}
            placeholder="Host port"
            className={fieldClass}
          />
          <input
            type="number"
            min="1"
            value={row.containerPort}
            onChange={(event) => updateRow(rows, onChange, index, { containerPort: event.target.value })}
            placeholder="Container port"
            className={fieldClass}
          />
          <select
            value={row.protocol}
            onChange={(event) => updateRow(rows, onChange, index, { protocol: event.target.value })}
            className={fieldClass}
          >
            <option value="tcp">tcp</option>
            <option value="udp">udp</option>
          </select>
          <RemoveButton onClick={() => removeRow(rows, onChange, index)} />
        </div>
      ))}
    </DynamicPanel>
  );
}

function VolumeMounts({ rows, onChange }: { rows: VolumeRow[]; onChange: (rows: VolumeRow[]) => void }) {
  return (
    <DynamicPanel
      title="Volume mounts"
      description="Attach named volumes to container paths."
      addLabel="Add mount"
      onAdd={() => onChange([...rows, { volumeName: '', containerPath: '' }])}
      isEmpty={rows.length === 0}
      emptyMessage="No volumes will be mounted."
    >
      {rows.map((row, index) => (
        <div key={index} className="grid gap-3 rounded-xl border border-zinc-800 bg-zinc-950/60 p-4 md:grid-cols-[1fr,1fr,auto]">
          <input
            value={row.volumeName}
            onChange={(event) => updateRow(rows, onChange, index, { volumeName: event.target.value })}
            placeholder="Volume name"
            className={fieldClass}
          />
          <input
            value={row.containerPath}
            onChange={(event) => updateRow(rows, onChange, index, { containerPath: event.target.value })}
            placeholder="/container/path"
            className={fieldClass}
          />
          <RemoveButton onClick={() => removeRow(rows, onChange, index)} />
        </div>
      ))}
    </DynamicPanel>
  );
}

function KeyValueRows({
  title,
  addLabel,
  keyPlaceholder,
  valuePlaceholder,
  rows,
  onChange,
}: {
  title: string;
  addLabel: string;
  keyPlaceholder: string;
  valuePlaceholder: string;
  rows: KeyValueRow[];
  onChange: (rows: KeyValueRow[]) => void;
}) {
  return (
    <DynamicPanel
      title={title}
      description="Only complete key-value pairs are included."
      addLabel={addLabel}
      onAdd={() => onChange([...rows, { key: '', value: '' }])}
      isEmpty={rows.length === 0}
      emptyMessage={`No ${title.toLowerCase()} configured.`}
    >
      {rows.map((row, index) => (
        <div key={index} className="grid gap-3 rounded-xl border border-zinc-800 bg-zinc-950/60 p-4 md:grid-cols-[1fr,1fr,auto]">
          <input
            value={row.key}
            onChange={(event) => updateRow(rows, onChange, index, { key: event.target.value })}
            placeholder={keyPlaceholder}
            className={fieldClass}
          />
          <input
            value={row.value}
            onChange={(event) => updateRow(rows, onChange, index, { value: event.target.value })}
            placeholder={valuePlaceholder}
            className={fieldClass}
          />
          <RemoveButton onClick={() => removeRow(rows, onChange, index)} />
        </div>
      ))}
    </DynamicPanel>
  );
}

function DynamicPanel({
  title,
  description,
  addLabel,
  onAdd,
  isEmpty,
  emptyMessage,
  children,
}: {
  title: string;
  description: string;
  addLabel: string;
  onAdd: () => void;
  isEmpty: boolean;
  emptyMessage: string;
  children: ReactNode;
}) {
  return (
    <section className="rounded-2xl border border-zinc-800 bg-zinc-950/40 p-5">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h3 className="text-sm font-semibold text-zinc-100">{title}</h3>
          <p className="mt-1 text-sm text-zinc-500">{description}</p>
        </div>
        <button
          type="button"
          onClick={onAdd}
          className="rounded-xl border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm font-medium text-zinc-300 transition-colors hover:bg-zinc-800"
        >
          {addLabel}
        </button>
      </div>
      {isEmpty ? <p className="rounded-xl border border-dashed border-zinc-800 p-4 text-sm text-zinc-500">{emptyMessage}</p> : <div className="space-y-3">{children}</div>}
    </section>
  );
}

function Field({ label, htmlFor, children }: { label: string; htmlFor: string; children: ReactNode }) {
  return (
    <label htmlFor={htmlFor} className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
        {label}
      </span>
      {children}
    </label>
  );
}

function RemoveButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-xl px-3 py-2 text-sm font-medium text-red-400 transition-colors hover:bg-red-400/10"
    >
      Remove
    </button>
  );
}

function updateRow<T>(rows: T[], onChange: (rows: T[]) => void, index: number, patch: Partial<T>) {
  onChange(rows.map((row, rowIndex) => (rowIndex === index ? { ...row, ...patch } : row)));
}

function removeRow<T>(rows: T[], onChange: (rows: T[]) => void, index: number) {
  onChange(rows.filter((_row, rowIndex) => rowIndex !== index));
}

function buildPayload({
  name,
  image,
  network,
  ports,
  volumes,
  envVars,
  labels,
  restartPolicy,
  memoryMB,
  cpuShares,
}: {
  name: string;
  image: string;
  network: string;
  ports: PortRow[];
  volumes: VolumeRow[];
  envVars: KeyValueRow[];
  labels: KeyValueRow[];
  restartPolicy: string;
  memoryMB: string;
  cpuShares: string;
}): CreateContainerPayload {
  const payload: CreateContainerPayload = {
    name: name.trim(),
    image: image.trim(),
  };
  const trimmedNetwork = network.trim();
  const parsedPorts = ports
    .map((port) => ({
      hostPort: Number(port.hostPort),
      containerPort: Number(port.containerPort),
      protocol: port.protocol,
    }))
    .filter((port) => port.hostPort > 0 && port.containerPort > 0);
  const parsedVolumes = volumes
    .map((volume) => ({
      volumeName: volume.volumeName.trim(),
      containerPath: volume.containerPath.trim(),
    }))
    .filter((volume) => volume.volumeName && volume.containerPath);
  const parsedEnvVars = toRecord(envVars);
  const parsedLabels = toRecord(labels);
  const parsedMemory = Number(memoryMB);
  const parsedCpuShares = Number(cpuShares);

  if (trimmedNetwork) payload.network = trimmedNetwork;
  if (parsedPorts.length > 0) payload.ports = parsedPorts;
  if (parsedVolumes.length > 0) payload.volumes = parsedVolumes;
  if (Object.keys(parsedEnvVars).length > 0) payload.envVars = parsedEnvVars;
  if (Object.keys(parsedLabels).length > 0) payload.labels = parsedLabels;
  if (restartPolicy) payload.restartPolicy = restartPolicy;
  if (parsedMemory > 0) payload.memoryMB = parsedMemory;
  if (parsedCpuShares > 0) payload.cpuShares = parsedCpuShares;

  return payload;
}

function toRecord(rows: KeyValueRow[]): Record<string, string> {
  return rows.reduce<Record<string, string>>((record, row) => {
    const key = row.key.trim();
    if (key) record[key] = row.value;
    return record;
  }, {});
}
