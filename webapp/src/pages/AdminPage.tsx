import { useState, type ReactNode } from 'react';
import { useConfig, useSaveConfig } from '../hooks/useAdmin';
import type { ConfigResponse } from '../types/api';

export function AdminPage() {
  const { data, isLoading, error } = useConfig();

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold">Admin</h1>
        <p className="mt-1 text-sm text-zinc-500">
          Edit the backend YAML configuration, update login settings, and apply changes immediately.
        </p>
      </div>

      {isLoading && (
        <div className="animate-pulse space-y-4">
          <div className="h-36 rounded-xl bg-zinc-800" />
          <div className="h-80 rounded-xl bg-zinc-800" />
        </div>
      )}

      {error && (
        <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center">
          <p className="text-red-400 font-medium">Failed to load configuration</p>
          <p className="text-red-400/60 text-sm mt-1">{error.message}</p>
        </div>
      )}

      {data && (
        <AdminForm
          key={`${data.path}:${data.auth.enabled}:${data.auth.username ?? ''}:${data.yaml}`}
          data={data}
        />
      )}
    </div>
  );
}

function AdminForm({ data }: { data: ConfigResponse }) {
  const saveConfig = useSaveConfig();
  const [yaml, setYaml] = useState(data.yaml);
  const [authEnabled, setAuthEnabled] = useState(data.auth.enabled);
  const [username, setUsername] = useState(data.auth.username ?? '');
  const [password, setPassword] = useState('');
  const [successMessage, setSuccessMessage] = useState('');

  const isDirty =
    yaml !== data.yaml ||
    authEnabled !== data.auth.enabled ||
    username !== (data.auth.username ?? '') ||
    password !== '';

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSuccessMessage('');

    const result = await saveConfig.mutateAsync({
      yaml,
      auth: {
        enabled: authEnabled,
        username,
        password,
      },
    });

    setPassword('');
    setSuccessMessage(result.message);
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <div className="rounded-2xl border border-zinc-800 bg-zinc-900 p-6">
        <p className="text-xs uppercase tracking-[0.24em] text-zinc-500">Config File</p>
        <p className="mt-2 font-mono text-sm text-zinc-300">{data.path}</p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[1.1fr,1.6fr]">
        <section className="rounded-2xl border border-zinc-800 bg-zinc-900 p-6">
          <div className="mb-5">
            <h2 className="text-lg font-semibold text-zinc-100">Login</h2>
            <p className="mt-1 text-sm text-zinc-500">
              When enabled, the standalone web app requires a username and password.
            </p>
          </div>

          <label className="mb-4 flex items-center gap-3 rounded-xl border border-zinc-800 bg-zinc-950/60 p-4">
            <input
              type="checkbox"
              checked={authEnabled}
              onChange={(event) => setAuthEnabled(event.target.checked)}
              className="h-4 w-4 rounded border-zinc-700 bg-zinc-950 text-emerald-500"
            />
            <span className="text-sm text-zinc-200">Require login for the standalone app</span>
          </label>

          <div className="space-y-4">
            <Field label="Username">
              <input
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                className="w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
                placeholder="admin"
              />
            </Field>

            <Field label={data.auth.has_password ? 'New Password' : 'Password'}>
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                className="w-full rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
                placeholder={
                  data.auth.has_password
                    ? 'Leave blank to keep the current password'
                    : 'Set an admin password'
                }
              />
            </Field>
          </div>
        </section>

        <section className="rounded-2xl border border-zinc-800 bg-zinc-900 p-6">
          <div className="mb-5 flex items-center justify-between gap-4">
            <div>
              <h2 className="text-lg font-semibold text-zinc-100">YAML Settings</h2>
              <p className="mt-1 text-sm text-zinc-500">
                Save applies the new config immediately.
              </p>
            </div>
            <div className="text-xs text-zinc-500">
              {yaml.split('\n').length} lines
            </div>
          </div>

          <textarea
            value={yaml}
            onChange={(event) => setYaml(event.target.value)}
            className="min-h-[28rem] w-full rounded-2xl border border-zinc-800 bg-zinc-950 p-4 font-mono text-sm leading-6 text-zinc-100 outline-none transition-colors focus:border-zinc-600"
            spellCheck={false}
          />
        </section>
      </div>

      {saveConfig.isError && (
        <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-300">
          {(saveConfig.error as Error).message}
        </div>
      )}

      {successMessage && (
        <div className="rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-4 text-sm text-emerald-300">
          {successMessage}
        </div>
      )}

      <div className="flex justify-end">
        <button
          type="submit"
          disabled={!isDirty || saveConfig.isPending}
          className="rounded-xl bg-emerald-600 px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {saveConfig.isPending ? 'Saving...' : 'Save and Apply'}
        </button>
      </div>
    </form>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
        {label}
      </span>
      {children}
    </label>
  );
}
