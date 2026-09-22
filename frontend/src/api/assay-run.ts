
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listAssayRun(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/assays?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createAssayRun(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/assays', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionAssayRun(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/assays/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
