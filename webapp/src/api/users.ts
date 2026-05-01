import { get, post, put, del } from './client';

export interface User {
  id: number;
  username: string;
  role: 'admin' | 'operator' | 'viewer';
  created_at: string;
  last_login?: string;
}

export interface CreateUserRequest {
  username: string;
  password: string;
  role: 'admin' | 'operator' | 'viewer';
}

export interface UpdateUserRequest {
  role: 'admin' | 'operator' | 'viewer';
}

export interface PasswordChangeRequest {
  current_password: string;
  new_password: string;
}

export async function listUsers(): Promise<User[]> {
  return get<User[]>('/api/admin/users');
}

export async function createUser(req: CreateUserRequest): Promise<User> {
  return post<User>('/api/admin/users', req);
}

export async function getUser(id: number): Promise<User> {
  return get<User>(`/api/admin/users/${id}`);
}

export async function updateUser(id: number, req: UpdateUserRequest): Promise<User> {
  return put<User>(`/api/admin/users/${id}`, req);
}

export async function deleteUser(id: number): Promise<void> {
  return del<void>(`/api/admin/users/${id}`);
}

export async function resetPassword(id: number, newPassword: string): Promise<void> {
  return post<void>(`/api/admin/users/${id}/reset-password`, { new_password: newPassword });
}

export async function changePassword(req: PasswordChangeRequest): Promise<void> {
  return post<void>('/api/users/me/password', req);
}
