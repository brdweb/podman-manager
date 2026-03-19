import { get, post, postJSON } from './client';
import type { SessionState } from '../types/api';

export function getSession(): Promise<SessionState> {
  return get<SessionState>('/api/auth/session');
}

export function login(username: string, password: string): Promise<SessionState> {
  return postJSON<SessionState>('/api/auth/login', { username, password });
}

export function logout(): Promise<{ success: boolean }> {
  return post<{ success: boolean }>('/api/auth/logout');
}
