
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listAnimalCase(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/cases?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createAnimalCase(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/cases', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionAnimalCase(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/cases/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
