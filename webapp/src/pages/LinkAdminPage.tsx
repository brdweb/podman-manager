import { useMemo, useState } from 'react';
import type { FormEvent, ReactNode } from 'react';
import type { ManagedLink, LinkPayload } from '../api/control';
import {
  useCreateLink,
  useDeleteLink,
  useImportHomepageLinks,
  useLinks,
  useUpdateLink,
} from '../hooks/useControl';
import { resolveIconSrc } from '../lib/icons';

const emptyForm: LinkPayload = {
  category_name: 'General',
  title: '',
  url: '',
  description: '',
  icon: '',
  target: '_blank',
  visibility_role: 'viewer',
  health_url: '',
  sort_order: 10,
  status: 'unknown',
  source: 'manual',
};

export function LinkAdminPage() {
  const { data, isLoading, error } = useLinks();
  const createLink = useCreateLink();
  const updateLink = useUpdateLink();
  const deleteLink = useDeleteLink();
  const importHomepage = useImportHomepageLinks();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<LinkPayload>(emptyForm);

  const categoryNames = useMemo(
    () => data?.categories.map((category) => category.name) ?? [],
    [data?.categories]
  );

  function edit(link: ManagedLink) {
    setEditingId(link.id);
    setForm({
      category_id: link.category_id,
      category_name: link.category_name,
      title: link.title,
      url: link.url,
      description: link.description ?? '',
      icon: link.icon ?? '',
      target: link.target,
      visibility_role: link.visibility_role,
      health_url: link.health_url ?? '',
      sort_order: link.sort_order,
      status: link.status,
      source: link.source ?? 'manual',
    });
  }

  function reset() {
    setEditingId(null);
    setForm(emptyForm);
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (editingId) {
      await updateLink.mutateAsync({ id: editingId, payload: form });
    } else {
      await createLink.mutateAsync(form);
    }
    reset();
  }

  const busy =
    createLink.isPending ||
    updateLink.isPending ||
    deleteLink.isPending ||
    importHomepage.isPending;

  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_400px]">
      <section className="space-y-5">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="text-sm font-medium uppercase tracking-[0.18em] text-emerald-400">
              Launchpad Admin
            </p>
            <h1 className="mt-2 text-3xl font-semibold tracking-tight">Managed links</h1>
          </div>
          <div className="flex gap-2">
            <a
              href="/api/v1/links/export?format=yaml"
              className="rounded-md border border-zinc-700 px-3 py-2 text-sm text-zinc-300 hover:border-zinc-500"
            >
              Export YAML
            </a>
            <button
              type="button"
              onClick={() => importHomepage.mutate()}
              disabled={busy}
              className="rounded-md bg-emerald-500 px-3 py-2 text-sm font-medium text-zinc-950 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Import Homepage
            </button>
          </div>
        </div>

        {importHomepage.data && (
          <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-4 text-sm text-emerald-200">
            Imported {importHomepage.data.imported} links ({importHomepage.data.created} created,{' '}
            {importHomepage.data.updated} updated, {importHomepage.data.skipped} skipped).
          </div>
        )}

        {error && (
          <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-300">
            {error.message}
          </div>
        )}

        <div className="overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900">
          <table className="min-w-full divide-y divide-zinc-800 text-sm">
            <thead className="bg-zinc-950/70 text-left text-xs uppercase tracking-wide text-zinc-500">
              <tr>
                <th className="px-4 py-3">Link</th>
                <th className="px-4 py-3">Group</th>
                <th className="px-4 py-3">Icon</th>
                <th className="px-4 py-3">Role</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-800">
              {isLoading ? (
                <tr>
                  <td className="px-4 py-6 text-zinc-500" colSpan={5}>
                    Loading links...
                  </td>
                </tr>
              ) : (
                data?.links.map((link) => (
                  <tr key={link.id} className="align-middle">
                    <td className="px-4 py-3">
                      <div className="font-medium text-zinc-100">{link.title}</div>
                      <div className="max-w-md truncate text-xs text-zinc-500">{link.url}</div>
                    </td>
                    <td className="px-4 py-3 text-zinc-400">{link.category_name}</td>
                    <td className="px-4 py-3">
                      <img
                        src={resolveIconSrc(link.icon, link.url)}
                        alt=""
                        className="h-8 w-8 rounded object-contain"
                        loading="lazy"
                      />
                    </td>
                    <td className="px-4 py-3 text-zinc-400">{link.visibility_role}</td>
                    <td className="px-4 py-3 text-right">
                      <button
                        type="button"
                        className="mr-2 rounded border border-zinc-700 px-2 py-1 text-xs text-zinc-300 hover:border-zinc-500"
                        onClick={() => edit(link)}
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        className="rounded border border-red-500/40 px-2 py-1 text-xs text-red-300 hover:border-red-400"
                        onClick={() => deleteLink.mutate(link.id)}
                        disabled={busy}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <aside className="rounded-lg border border-zinc-800 bg-zinc-900 p-5">
        <h2 className="text-lg font-semibold">{editingId ? 'Edit link' : 'Create link'}</h2>
        <form className="mt-5 space-y-4" onSubmit={submit}>
          <Field label="Title">
            <input
              value={form.title}
              onChange={(event) => setForm({ ...form, title: event.target.value })}
              required
              className="field"
            />
          </Field>
          <Field label="URL">
            <input
              value={form.url}
              onChange={(event) => setForm({ ...form, url: event.target.value })}
              required
              type="url"
              className="field"
            />
          </Field>
          <Field label="Group">
            <input
              value={form.category_name}
              onChange={(event) =>
                setForm({ ...form, category_id: undefined, category_name: event.target.value })
              }
              list="link-categories"
              required
              className="field"
            />
            <datalist id="link-categories">
              {categoryNames.map((name) => (
                <option key={name} value={name} />
              ))}
            </datalist>
          </Field>
          <Field label="Description">
            <textarea
              value={form.description}
              onChange={(event) => setForm({ ...form, description: event.target.value })}
              rows={3}
              className="field"
            />
          </Field>
          <Field label="Icon">
            <input
              value={form.icon}
              onChange={(event) => setForm({ ...form, icon: event.target.value })}
              placeholder="jellyfin.png or https://..."
              className="field"
            />
            <div className="mt-2 flex items-center gap-2 text-xs text-zinc-500">
              <img
                src={resolveIconSrc(form.icon, form.url || 'https://brdweb.com')}
                alt=""
                className="h-7 w-7 rounded object-contain"
              />
              Homepage-style slugs and full URLs are supported.
            </div>
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="Role">
              <select
                value={form.visibility_role}
                onChange={(event) =>
                  setForm({
                    ...form,
                    visibility_role: event.target.value as LinkPayload['visibility_role'],
                  })
                }
                className="field"
              >
                <option value="viewer">viewer</option>
                <option value="operator">operator</option>
                <option value="admin">admin</option>
              </select>
            </Field>
            <Field label="Order">
              <input
                value={form.sort_order}
                onChange={(event) =>
                  setForm({ ...form, sort_order: Number(event.target.value) || 0 })
                }
                type="number"
                className="field"
              />
            </Field>
          </div>
          <Field label="Health URL">
            <input
              value={form.health_url}
              onChange={(event) => setForm({ ...form, health_url: event.target.value })}
              type="url"
              className="field"
            />
          </Field>
          <div className="flex gap-2 pt-2">
            <button
              type="submit"
              disabled={busy}
              className="rounded-md bg-emerald-500 px-4 py-2 text-sm font-medium text-zinc-950 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {editingId ? 'Save changes' : 'Create link'}
            </button>
            {editingId && (
              <button
                type="button"
                onClick={reset}
                className="rounded-md border border-zinc-700 px-4 py-2 text-sm text-zinc-300"
              >
                Cancel
              </button>
            )}
          </div>
        </form>
      </aside>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block text-sm text-zinc-400">
      <span className="mb-1 block">{label}</span>
      {children}
    </label>
  );
}
