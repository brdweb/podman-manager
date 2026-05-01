import { useState, type FormEvent, type ReactNode } from 'react';
import type { User } from '../api/users';
import {
  useCreateUser,
  useDeleteUser,
  useResetPassword,
  useUpdateUser,
  useUsers,
} from '../hooks/useUsers';
import { useToast } from '../components/Toast';

type Role = User['role'];

const roles: Role[] = ['admin', 'operator', 'viewer'];
const usernamePattern = /^[A-Za-z0-9_-]{3,64}$/;

export function UsersPage() {
  const { data: users, isLoading, error } = useUsers();
  const createUser = useCreateUser();
  const updateUser = useUpdateUser();
  const resetPassword = useResetPassword();
  const deleteUser = useDeleteUser();
  const { addToast } = useToast();

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [roleTarget, setRoleTarget] = useState<User | null>(null);
  const [passwordTarget, setPasswordTarget] = useState<User | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);
  const [openMenuId, setOpenMenuId] = useState<number | null>(null);

  async function handleCreate(username: string, password: string, role: Role) {
    try {
      await createUser.mutateAsync({ username, password, role });
      setIsCreateOpen(false);
      addToast(`Created user ${username}.`, 'success');
    } catch (err) {
      addToast(getErrorMessage(err, 'Failed to create user.'), 'error');
    }
  }

  async function handleUpdateRole(id: number, username: string, role: Role) {
    try {
      await updateUser.mutateAsync({ id, role });
      setRoleTarget(null);
      addToast(`Updated ${username} to ${role}.`, 'success');
    } catch (err) {
      addToast(getErrorMessage(err, 'Failed to update role.'), 'error');
    }
  }

  async function handleResetPassword(id: number, username: string, newPassword: string) {
    try {
      await resetPassword.mutateAsync({ id, newPassword });
      setPasswordTarget(null);
      addToast(`Reset password for ${username}.`, 'success');
    } catch (err) {
      addToast(getErrorMessage(err, 'Failed to reset password.'), 'error');
    }
  }

  async function handleDelete(id: number, username: string) {
    try {
      await deleteUser.mutateAsync(id);
      setDeleteTarget(null);
      addToast(`Deleted user ${username}.`, 'success');
    } catch (err) {
      addToast(getErrorMessage(err, 'Failed to delete user.'), 'error');
    }
  }

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-2xl font-bold">User Management</h1>
          <p className="mt-1 text-sm text-zinc-500">
            Create accounts, assign roles, and maintain access to Podman Manager.
          </p>
        </div>
        <button
          type="button"
          onClick={() => setIsCreateOpen(true)}
          className="rounded-xl bg-blue-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-blue-700"
        >
          Add User
        </button>
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
          <p className="font-medium text-red-400">Failed to load users</p>
          <p className="mt-1 text-sm text-red-400/60">{error.message}</p>
        </div>
      )}

      {users && (
        <div className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900/50">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-zinc-800 bg-zinc-900/50 text-xs uppercase tracking-wider text-zinc-500">
                <tr>
                  <th className="px-4 py-3 font-medium">Username</th>
                  <th className="px-4 py-3 font-medium">Role</th>
                  <th className="px-4 py-3 font-medium">Created</th>
                  <th className="px-4 py-3 font-medium">Last Login</th>
                  <th className="px-4 py-3 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-800/50">
                {users.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-4 py-8 text-center text-zinc-500">
                      No users exist yet.
                    </td>
                  </tr>
                ) : (
                  users.map((user) => (
                    <tr key={user.id} className="transition-colors hover:bg-zinc-800/30">
                      <td className="px-4 py-3 font-medium text-zinc-200">{user.username}</td>
                      <td className="px-4 py-3">
                        <RoleBadge role={user.role} />
                      </td>
                      <td className="px-4 py-3 text-zinc-400">{formatDate(user.created_at)}</td>
                      <td className="px-4 py-3 text-zinc-400">{formatDate(user.last_login)}</td>
                      <td className="px-4 py-3 text-right">
                        <div className="relative inline-flex">
                          <button
                            type="button"
                            onClick={() => setOpenMenuId(openMenuId === user.id ? null : user.id)}
                            className="rounded-md px-2 py-1 text-lg leading-none text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-100"
                            aria-label={`Actions for ${user.username}`}
                          >
                            ⋮
                          </button>
                          {openMenuId === user.id && (
                            <div className="absolute right-0 top-8 z-20 w-44 overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 py-1 text-left shadow-2xl">
                              <MenuButton
                                label="Change role"
                                onClick={() => {
                                  setRoleTarget(user);
                                  setOpenMenuId(null);
                                }}
                              />
                              <MenuButton
                                label="Reset password"
                                onClick={() => {
                                  setPasswordTarget(user);
                                  setOpenMenuId(null);
                                }}
                              />
                              <MenuButton
                                label="Delete user"
                                danger
                                onClick={() => {
                                  setDeleteTarget(user);
                                  setOpenMenuId(null);
                                }}
                              />
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {isCreateOpen && (
        <CreateUserModal
          isPending={createUser.isPending}
          onClose={() => setIsCreateOpen(false)}
          onSubmit={handleCreate}
        />
      )}

      {roleTarget && (
        <EditRoleModal
          user={roleTarget}
          isPending={updateUser.isPending}
          onClose={() => setRoleTarget(null)}
          onSubmit={handleUpdateRole}
        />
      )}

      {passwordTarget && (
        <ResetPasswordModal
          user={passwordTarget}
          isPending={resetPassword.isPending}
          onClose={() => setPasswordTarget(null)}
          onSubmit={handleResetPassword}
        />
      )}

      {deleteTarget && (
        <DeleteUserModal
          user={deleteTarget}
          isPending={deleteUser.isPending}
          onClose={() => setDeleteTarget(null)}
          onConfirm={handleDelete}
        />
      )}
    </div>
  );
}

function CreateUserModal({
  isPending,
  onClose,
  onSubmit,
}: {
  isPending: boolean;
  onClose: () => void;
  onSubmit: (username: string, password: string, role: Role) => Promise<void>;
}) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState<Role>('viewer');
  const [errors, setErrors] = useState<Record<string, string>>({});

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextErrors = validateUserForm(username, password);
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;
    await onSubmit(username, password, role);
  }

  return (
    <Modal title="Add User" description="Create a new account and assign an access role.">
      <form onSubmit={handleSubmit} className="mt-6 space-y-4">
        <TextField
          id="new-username"
          label="Username"
          value={username}
          onChange={setUsername}
          placeholder="operator1"
          error={errors.username}
        />
        <TextField
          id="new-password"
          label="Password"
          type="password"
          value={password}
          onChange={setPassword}
          placeholder="At least 8 characters"
          error={errors.password}
        />
        <RoleSelect value={role} onChange={setRole} />
        <ModalActions
          cancelLabel="Cancel"
          submitLabel={isPending ? 'Creating...' : 'Create User'}
          isPending={isPending}
          onCancel={onClose}
          submitClassName="bg-blue-600 text-white hover:bg-blue-700"
        />
      </form>
    </Modal>
  );
}

function EditRoleModal({
  user,
  isPending,
  onClose,
  onSubmit,
}: {
  user: User;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (id: number, username: string, role: Role) => Promise<void>;
}) {
  const [role, setRole] = useState<Role>(user.role);

  return (
    <Modal title="Change Role" description={`Update access permissions for ${user.username}.`}>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          void onSubmit(user.id, user.username, role);
        }}
        className="mt-6 space-y-4"
      >
        <RoleSelect value={role} onChange={setRole} />
        <ModalActions
          cancelLabel="Cancel"
          submitLabel={isPending ? 'Saving...' : 'Save Role'}
          isPending={isPending}
          onCancel={onClose}
          submitClassName="bg-blue-600 text-white hover:bg-blue-700"
        />
      </form>
    </Modal>
  );
}

function ResetPasswordModal({
  user,
  isPending,
  onClose,
  onSubmit,
}: {
  user: User;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (id: number, username: string, newPassword: string) => Promise<void>;
}) {
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextErrors = validatePasswordForm(password, confirmPassword);
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;
    await onSubmit(user.id, user.username, password);
  }

  return (
    <Modal title="Reset Password" description={`Set a new password for ${user.username}.`}>
      <form onSubmit={handleSubmit} className="mt-6 space-y-4">
        <TextField
          id="reset-password"
          label="New Password"
          type="password"
          value={password}
          onChange={setPassword}
          placeholder="At least 8 characters"
          error={errors.password}
        />
        <TextField
          id="confirm-password"
          label="Confirm Password"
          type="password"
          value={confirmPassword}
          onChange={setConfirmPassword}
          placeholder="Repeat the new password"
          error={errors.confirmPassword}
        />
        <ModalActions
          cancelLabel="Cancel"
          submitLabel={isPending ? 'Resetting...' : 'Reset Password'}
          isPending={isPending}
          onCancel={onClose}
          submitClassName="bg-blue-600 text-white hover:bg-blue-700"
        />
      </form>
    </Modal>
  );
}

function DeleteUserModal({
  user,
  isPending,
  onClose,
  onConfirm,
}: {
  user: User;
  isPending: boolean;
  onClose: () => void;
  onConfirm: (id: number, username: string) => Promise<void>;
}) {
  return (
    <Modal title="Delete User" description="This action cannot be undone.">
      <p className="mt-4 text-sm text-zinc-400">
        Delete <span className="font-mono text-zinc-200">{user.username}</span> from Podman Manager?
      </p>
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
          type="button"
          onClick={() => void onConfirm(user.id, user.username)}
          disabled={isPending}
          className="rounded-xl bg-red-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-red-500 disabled:opacity-50"
        >
          {isPending ? 'Deleting...' : 'Delete User'}
        </button>
      </div>
    </Modal>
  );
}

function Modal({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl">
        <h2 className="text-xl font-bold text-zinc-100">{title}</h2>
        <p className="mt-2 text-sm text-zinc-400">{description}</p>
        {children}
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
  type = 'text',
  error,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  type?: 'text' | 'password';
  error?: string;
}) {
  return (
    <div>
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-zinc-300">
        {label}
      </label>
      <input
        id={id}
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors placeholder:text-zinc-600 focus:border-zinc-600"
        placeholder={placeholder}
      />
      {error && <p className="mt-1.5 text-xs text-red-300">{error}</p>}
    </div>
  );
}

function RoleSelect({ value, onChange }: { value: Role; onChange: (role: Role) => void }) {
  return (
    <div>
      <label htmlFor="role" className="mb-1.5 block text-sm font-medium text-zinc-300">
        Role
      </label>
      <select
        id="role"
        value={value}
        onChange={(event) => onChange(event.target.value as Role)}
        className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors focus:border-zinc-600"
      >
        {roles.map((role) => (
          <option key={role} value={role}>
            {toTitleCase(role)}
          </option>
        ))}
      </select>
    </div>
  );
}

function ModalActions({
  cancelLabel,
  submitLabel,
  isPending,
  onCancel,
  submitClassName,
}: {
  cancelLabel: string;
  submitLabel: string;
  isPending: boolean;
  onCancel: () => void;
  submitClassName: string;
}) {
  return (
    <div className="mt-6 flex justify-end gap-3">
      <button
        type="button"
        onClick={onCancel}
        disabled={isPending}
        className="rounded-xl px-4 py-2.5 text-sm font-medium text-zinc-400 transition-colors hover:text-zinc-200 disabled:opacity-50"
      >
        {cancelLabel}
      </button>
      <button
        type="submit"
        disabled={isPending}
        className={`rounded-xl px-4 py-2.5 text-sm font-medium transition-colors disabled:opacity-50 ${submitClassName}`}
      >
        {submitLabel}
      </button>
    </div>
  );
}

function MenuButton({ label, onClick, danger = false }: { label: string; onClick: () => void; danger?: boolean }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`block w-full px-3 py-2 text-left text-sm transition-colors hover:bg-zinc-900 ${
        danger ? 'text-red-300 hover:text-red-200' : 'text-zinc-300 hover:text-zinc-100'
      }`}
    >
      {label}
    </button>
  );
}

function RoleBadge({ role }: { role: Role }) {
  const styles: Record<Role, string> = {
    admin: 'border-rose-500/30 bg-rose-500/15 text-rose-300',
    operator: 'border-blue-500/30 bg-blue-500/15 text-blue-300',
    viewer: 'border-zinc-500/30 bg-zinc-500/15 text-zinc-300',
  };

  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium ${styles[role]}`}>
      {toTitleCase(role)}
    </span>
  );
}

function validateUserForm(username: string, password: string): Record<string, string> {
  return {
    ...validateUsername(username),
    ...validatePassword(password),
  };
}

function validatePasswordForm(password: string, confirmPassword: string): Record<string, string> {
  const errors = validatePassword(password);
  if (password !== confirmPassword) {
    errors.confirmPassword = 'Passwords do not match.';
  }
  return errors;
}

function validateUsername(username: string): Record<string, string> {
  if (!usernamePattern.test(username)) {
    return {
      username: 'Use 3-64 characters: letters, numbers, underscores, or hyphens.',
    };
  }
  return {};
}

function validatePassword(password: string): Record<string, string> {
  if (password.length < 8) {
    return { password: 'Password must be at least 8 characters.' };
  }
  return {};
}

function formatDate(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function toTitleCase(value: string) {
  return `${value.charAt(0).toUpperCase()}${value.slice(1)}`;
}

function getErrorMessage(err: unknown, fallback: string) {
  return err instanceof Error ? err.message : fallback;
}
