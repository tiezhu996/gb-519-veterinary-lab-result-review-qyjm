
import type { DomainRecord } from '../../types/domain';
import { EmptyState } from './EmptyState';
import { StatusBadge } from './StatusBadge';

export function ResultPanel({ records }: { records: DomainRecord[] }) {
  if (!records.length) return <EmptyState message="暂无可展示的结果证据" />;
  return <div className="evidence-strip">{records.slice(0, 4).map((item) => {
    const latest = item.revisions?.at(-1);
    return <article key={item.id}>
      <div className="result-title"><strong>{item.code}</strong><StatusBadge status={item.status} /></div>
      <span>{item.name}</span>
      <small title={item.evidence}>{item.evidence || '尚未附加证据'}</small>
      <small>v{item.version} · {latest?.actor || item.reviewedBy || item.preparedBy || item.owner}</small>
      <code title={latest?.requestId}>{latest?.requestId || '待形成签发请求 ID'}</code>
    </article>;
  })}</div>;
}
