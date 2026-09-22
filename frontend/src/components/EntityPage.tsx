import { useEffect, useMemo, useState } from 'react';
import type { EntityConfig, DomainRecord } from '../types/domain';
import type { EntityStore } from '../stores/factory';
import { nextStatus, formatDate } from '../utils/format';
import { StatusBadge } from './common/StatusBadge';
import { RiskTag } from './common/RiskTag';
import { ResultPanel } from './common/ResultPanel';
import { EmptyState } from './common/EmptyState';
import { MetricCard } from './common/MetricCard';
import { ConfirmDialog } from './common/ConfirmDialog';
import { UiButton } from './common/UiButton';
import { useAuth } from '../hooks/useAuth';

interface EntityPageProps {
  config: EntityConfig;
  useStore: EntityStore;
  showRiskTags?: boolean;
  showResultPanel?: boolean;
}

const HIGH_RISK_LEVELS = ['high', 'critical'] as const;
const isHighRiskLevel = (level?: string) => HIGH_RISK_LEVELS.includes(level as typeof HIGH_RISK_LEVELS[number]);

// Linked-specimen risk drives grading; fall back to the first-review snapshot
// once the record has already entered second-level review.
const effectiveSpecimenRisk = (item: DomainRecord) => item.specimenRiskLevel || item.relatedRiskLevel || '';

export function EntityPage({ config, useStore, showRiskTags = false, showResultPanel = false }: EntityPageProps) {
  const { items, meta, loading, error, load, createRecord, transition } = useStore();
  const { session, hasRole } = useAuth();
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [pending, setPending] = useState<{ item: DomainRecord; status: string } | null>(null);

  useEffect(() => { void load(config.path); }, [config.path, load]);
  const highRisk = useMemo(() => items.filter((item) => ['high', 'critical'].includes(item.riskLevel)).length, [items]);
  const canOperate = hasRole('operator');
  const isSignoff = config.key === 'resultSignoff';

  const createDemo = async () => {
    const now = Date.now();
    await createRecord(config.path, {
      code: `${config.key.toUpperCase()}-${now.toString().slice(-6)}`,
      name: `新增${config.label}`,
      description: '通过前端工作台创建的业务记录',
      facility: '默认检验区', owner: session?.displayName || '现场操作员', category: '常规', riskLevel: 'medium',
      metricValue: 25, metricUnit: 'unit', effectiveAt: new Date().toISOString(), evidence: '已完成创建前证据核对',
      // Signoff drafts must reference an existing specimen code for risk grading.
      relatedCode: isSignoff ? 'S-001' : '',
    });
    setSearch('');
    setShowCreate(false);
  };

  const requiresPreparer = (item: DomainRecord) => isSignoff && item.status === 'draft';
  const isOriginalPreparer = (item: DomainRecord) => Boolean(session?.username && session.username === item.preparedBy);
  const isFirstReviewer = (item: DomainRecord) => Boolean(session?.username && session.username === item.firstReviewBy);

  // Next signoff status branches by linked-specimen risk: high/critical first
  // enters second_review, ordinary risk stays a single different-person sign.
  const nextActionStatus = (item: DomainRecord): string | null => {
    if (!isSignoff) return nextStatus(item.status, config.primaryTransitions);
    if (item.status === 'peer_review') {
      return isHighRiskLevel(effectiveSpecimenRisk(item)) ? 'second_review' : 'signed';
    }
    return nextStatus(item.status, config.primaryTransitions);
  };

  const requiresReviewerAction = (item: DomainRecord) => isSignoff && (item.status === 'peer_review' || item.status === 'second_review');

  const canAdvance = (item: DomainRecord) => {
    if (!canOperate) return false;
    if (requiresPreparer(item)) return isOriginalPreparer(item);
    if (requiresReviewerAction(item)) {
      if (!hasRole('reviewer')) return false;
      if (isOriginalPreparer(item)) return false;
      // The second-level reviewer must differ from the first confirmation.
      if (item.status === 'second_review' && isFirstReviewer(item)) return false;
      return true;
    }
    return true;
  };

  const unavailableReason = (item: DomainRecord) => {
    if (!canOperate) return '只读';
    if (requiresPreparer(item) && !isOriginalPreparer(item)) return '等待制单人';
    if (requiresReviewerAction(item)) {
      if (!hasRole('reviewer')) return item.status === 'second_review' ? '等待二级复核员' : '等待复核员';
      if (isOriginalPreparer(item)) return '需异人复核';
      if (item.status === 'second_review' && isFirstReviewer(item)) return '需第二名不同复核员';
    }
    return '流程结束';
  };

  const confirmTransition = async () => {
    if (!pending) return;
    await transition(config.path, pending.item, pending.status);
    setSearch('');
    setPending(null);
  };

  const actionLabel = (status: string) => status === 'second_review' ? '首名确认并待二级复核' : `推进至 ${status}`;

  return <main className="workspace">
    <header className="page-header"><div><p className="eyebrow">业务工作台</p><h1>{config.label}</h1><p>统一管理{config.label}的状态、风险、证据与责任人。</p></div>{canOperate ? <UiButton onClick={() => setShowCreate(true)}>新增{config.label}</UiButton> : <span className="access-note">只读权限</span>}</header>
    <section className="metrics"><MetricCard label="记录总数" value={meta.total} detail="当前筛选范围"/><MetricCard label="高风险" value={highRisk} detail="需要优先复核"/><MetricCard label="状态种类" value={new Set(items.map((item) => item.status)).size} detail="状态机覆盖"/></section>
    {showResultPanel && <section className="result-section"><header><h2>结果与版本证据</h2><span>签发版本、操作者和请求 ID 可追溯</span></header><ResultPanel records={items} signoffView={isSignoff} /></section>}
    <section className="toolbar"><input aria-label="搜索" placeholder={`搜索${config.label}编码或名称`} value={search} onChange={(event) => setSearch(event.target.value)} /><UiButton onClick={() => void load(config.path, search)}>查询</UiButton><button className="link-button" onClick={() => { setSearch(''); void load(config.path); }}>重置</button></section>
    {error && <div className="alert" role="alert">{error}</div>}
    <section className="table-shell" aria-busy={loading}><table><thead><tr>
      <th>编码</th><th>名称</th><th>状态</th><th>{isSignoff ? '样本风险' : '风险'}</th><th>责任人</th>{isSignoff && <th>复核与待办</th>}<th>指标</th><th>更新时间</th><th>操作</th>
    </tr></thead><tbody>
      {items.map((item) => {
        const next = nextActionStatus(item);
        const riskLevel = (isSignoff ? (effectiveSpecimenRisk(item) || item.riskLevel) : item.riskLevel) as DomainRecord['riskLevel'];
        return <tr key={item.id}><td><strong>{item.code}</strong>{isSignoff && item.relatedCode && <small>关联样本 {item.relatedCode}</small>}</td><td>{item.name}<small>{item.facility}</small></td><td><StatusBadge status={item.status}/></td><td>{showRiskTags || isSignoff ? <RiskTag level={riskLevel}/> : riskLevel}</td><td>{item.owner}</td>{isSignoff && <td>
          {item.status === 'peer_review' && <small>{item.pendingTodo || '等待复核决定'}</small>}
          {item.status === 'second_review' && <>
            <small className="muted">待办：{item.pendingTodo || '待二级复核'}</small>
            <small>首名确认：{item.firstReviewBy || '-'} · {item.firstReviewAt ? formatDate(item.firstReviewAt) : '-'}</small>
            <small title={item.firstReviewReason}>原因：{item.firstReviewReason || '-'}</small>
          </>}
          {item.status === 'draft' && <small className="muted">{item.pendingTodo || '草稿编辑中'}</small>}
          {(item.status === 'signed' || item.status === 'rejected') && <small title={item.reviewReason}>签发：{item.reviewedBy || '-'}{item.firstReviewBy ? ` / 首名 ${item.firstReviewBy}` : ''}</small>}
        </td>}<td>{item.metricValue} {item.metricUnit}</td><td>{formatDate(item.updatedAt)}</td><td>{next && canAdvance(item) ? <button className="table-action" onClick={() => setPending({ item, status: next })}>{actionLabel(next)}</button> : <span className="muted">{next ? unavailableReason(item) : '流程结束'}</span>}</td></tr>;
      })}
      {!items.length && !loading && <EmptyState message="暂无记录" colSpan={isSignoff ? 9 : 8} />}
    </tbody></table>{loading && <div className="loading">正在同步业务数据…</div>}</section>
    <ConfirmDialog open={showCreate} title={`新增${config.label}`} onCancel={() => setShowCreate(false)} onConfirm={() => void createDemo().catch(() => undefined)}><p>将创建一条包含完整责任人、风险和证据信息的演示记录{isSignoff ? '（关联样本 S-001，普通风险单次复核）' : ''}。</p></ConfirmDialog>
    <ConfirmDialog open={Boolean(pending)} title="确认状态迁移" onCancel={() => setPending(null)} onConfirm={() => void confirmTransition().catch(() => undefined)}><p>状态迁移会写入不可覆盖的版本与审计日志。</p><strong>{pending?.item.status} → {pending?.status}</strong></ConfirmDialog>
  </main>;
}
