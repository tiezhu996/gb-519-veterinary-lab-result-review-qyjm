
import type { DomainRecord } from '../../types/domain';
import { EmptyState } from './EmptyState';
import { StatusBadge } from './StatusBadge';
import { RiskTag } from './RiskTag';
import { formatDate } from '../../utils/format';

interface ResultPanelProps {
  records: DomainRecord[];
  signoffView?: boolean;
}

export function ResultPanel({ records, signoffView = false }: ResultPanelProps) {
  if (!records.length) return <EmptyState message="暂无可展示的结果证据" />;
  return <div className="evidence-strip">{records.slice(0, 4).map((item) => {
    const latest = item.revisions?.at(-1);
    const specimenRisk = item.specimenRiskLevel || item.relatedRiskLevel || item.riskLevel;
    return <article key={item.id}>
      <div className="result-title"><strong>{item.code}</strong><StatusBadge status={item.status} /></div>
      <span>{item.name}</span>
      {signoffView
        ? <small className="result-risk">样本风险 <RiskTag level={(specimenRisk || item.riskLevel) as DomainRecord['riskLevel']} />{item.relatedCode ? `（${item.relatedCode}）` : ''}</small>
        : <small title={item.evidence}>{item.evidence || '尚未附加证据'}</small>}
      {signoffView && item.status === 'second_review' && <>
        <small>首名确认：{item.firstReviewBy || '-'} · {item.firstReviewAt ? formatDate(item.firstReviewAt) : '-'}</small>
        <small title={item.firstReviewReason}>原因：{item.firstReviewReason || '-'}</small>
      </>}
      {signoffView && <small className="muted">待办：{item.pendingTodo || '-'}</small>}
      <small>v{item.version} · {latest?.actor || item.reviewedBy || item.preparedBy || item.owner}</small>
      <code title={latest?.requestId}>{latest?.requestId || '待形成签发请求 ID'}</code>
    </article>;
  })}</div>;
}
