import type { EntityConfig } from './domain';

export type SpecimenState = 'received' | 'testing' | 'hold' | 'released' | 'disposed';
export const ALL_SPECIMEN_STATE: readonly SpecimenState[] = ['received', 'testing', 'hold', 'released', 'disposed'];
export type SignoffState = 'draft' | 'peer_review' | 'signed' | 'rejected';
export const ALL_SIGNOFF_STATE: readonly SignoffState[] = ['draft', 'peer_review', 'signed', 'rejected'];

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
  { key: 'animalCase', path: 'cases', label: '动物样本来源', statuses: ['registered', 'sampling', 'testing', 'closed'] as const, primaryTransitions: { registered: 'sampling', sampling: 'testing', testing: 'closed' } },
  { key: 'specimen', path: 'specimens', label: '检验样本', statuses: ['received', 'testing', 'hold', 'released', 'disposed'] as const, primaryTransitions: { received: 'testing', testing: 'released', hold: 'released', released: 'disposed' } },
  { key: 'assayRun', path: 'assays', label: '检测运行', statuses: ['planned', 'running', 'validated', 'invalid'] as const, primaryTransitions: { planned: 'running', running: 'validated' } },
  { key: 'resultSignoff', path: 'signoff', label: '结果签发', statuses: ['draft', 'peer_review', 'signed', 'rejected'] as const, primaryTransitions: { draft: 'peer_review', peer_review: 'signed' } }
];
