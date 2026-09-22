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
  const isReviewerLike = hasRole('reviewer');

  const createDemo = async () => {
    const now = Date.now();
    await createRecord(config.path, {
      code: `${config.key.toUpperCase()}-${now.toString().slice(-6)}`,
      name: `新增${config.label}`,
      description: '通过前端工作台创建的业务记录',
      facility: '默认检验区', owner: session?.displayName || '现场操作员', category: '常规', riskLevel: 'medium',
      metricValue: 25, metricUnit: 'unit', effectiveAt: new Date().toISOString(), evidence: '已完成创建前证据核对',
      // Signoff review is graded by the linked specimen; point demo drafts at a
      // seeded normal-risk specimen so the single-approval path stays demoable.
      relatedCode: isSignoff ? 'S-002' : '',
    });
    setSearch('');
    setShowCreate(false);
  };

  const isOriginalPreparer = (item: DomainRecord) => Boolean(session?.username && session.username === item.preparedBy);
  const isFirstReviewer = (item: DomainRecord) => Boolean(session?.username && session.username === item.firstReviewedBy);
  const linkedHighRisk = (item: DomainRecord) => HIGH_RISK_LEVELS.includes((item.linkedSpecimenRiskLevel || '') as typeof HIGH_RISK_LEVELS[number]);

  // Review gating for risk-graded signoff:
  // draft -> only the preparer submits; peer_review -> a different reviewer/admin
  // confirms (missing linked specimen blocks everyone); secondary_review -> a
  // second reviewer different from BOTH the preparer and the first reviewer.
  const canAdvance = (item: DomainRecord) => {
    if (!canOperate) return false;
    if (!isSignoff) return true;
    if (item.status === 'draft') return isOriginalPreparer(item);
    if (item.status === 'peer_review') {
      if (item.linkedSpecimenMissing) return false;
      return isReviewerLike && !isOriginalPreparer(item);
    }
    if (item.status === 'secondary_review') {
      return isReviewerLike && !isOriginalPreparer(item) && !isFirstReviewer(item);
    }
    return false;
  };

  const unavailableReason = (item: DomainRecord) => {
    if (!isSignoff) return '流程结束';
    if (item.status === 'draft') return isOriginalPreparer(item) ? '流程结束' : '等待制单人';
    if (item.status === 'peer_review') {
      if (item.linkedSpecimenMissing) return '关联样本缺失，拒绝复核';
      if (!canOperate) return '只读';
      if (!isReviewerLike) return '等待复核员';
      if (isOriginalPreparer(item)) return '需异人复核';
      return '等待复核员';
    }
    if (item.status === 'secondary_review') {
      if (!canOperate) return '只读';
      if (!isReviewerLike) return '待二级复核';
      if (isOriginalPreparer(item)) return '需异人复核';
      if (isFirstReviewer(item)) return '不可由首名确认人重复签发';
      return '待二级复核';
    }
    return '流程结束';
  };

  // The first reviewer always requests signed; the server downgrades high/critical
  // linked specimens to secondary_review. The label makes that outcome explicit.
  const actionLabel = (item: DomainRecord, fallback: string | null) => {
    if (!isSignoff) return fallback ? `推进至 ${fallback}` : '流程结束';
    if (item.status === 'peer_review') return linkedHighRisk(item) ? '首位确认（高风险）' : '批准签发';
    if (item.status === 'secondary_review') return '二级批准签发';
    return fallback ? `推进至 ${fallback}` : '流程结束';
  };

  const displayTarget = (item: DomainRecord, requested: string) => {
    if (isSignoff && item.status === 'peer_review' && requested === 'signed' && linkedHighRisk(item)) {
      return 'secondary_review（待二级复核）';
    }
    return requested;
  };

  const reviewQueue = useMemo(
    () => isSignoff ? items.filter((item) => item.status === 'peer_review' || item.status === 'secondary_review') : [],
    [isSignoff, items],
  );

  const todoFor = (item: DomainRecord): string => {
    if (item.status === 'peer_review') {
      if (item.linkedSpecimenMissing) return '关联样本缺失，复核已被拒绝，请退回制单人补全关联';
      return linkedHighRisk(item) ? '待首位复核员确认，确认后进入二级复核' : '待复核员单次批准签发';
    }
    if (item.status === 'secondary_review') {
      return `待第二名不同复核员批准签发（首名确认：${item.firstReviewedBy || '未知'}）`;
    }
    return '—';
  };

  const confirmTransition = async () => {
    if (!pending) return;
    await transition(config.path, pending.item, pending.status);
    setSearch('');
    setPending(null);
  };

  return <main className="workspace">
    <header className="page-header"><div><p className="eyebrow">业务工作台</p><h1>{config.label}</h1><p>统一管理{config.label}的状态、风险、证据与责任人。</p></div>{canOperate ? <UiButton onClick={() => setShowCreate(true)}>新增{config.label}</UiButton> : <span className="access-note">只读权限</span>}</header>
    <section className="metrics"><MetricCard label="记录总数" value={meta.total} detail="当前筛选范围"/><MetricCard label="高风险" value={highRisk} detail="需要优先复核"/><MetricCard label="状态种类" value={new Set(items.map((item) => item.status)).size} detail="状态机覆盖"/></section>
    {isSignoff && <section className="review-queue" data-testid="signoff-review-queue">
      <header><h2>复核待办（按关联样本风险分级）</h2><span>样本风险 · 首名确认 · 待办 · 原因，刷新后自动回读</span></header>
      {reviewQueue.length === 0 && <EmptyState message="暂无待复核签发记录" />}
      <div className="review-grid">
        {reviewQueue.map((item) => <article key={item.id} className="review-card">
          <div className="result-title"><strong>{item.code}</strong><StatusBadge status={item.status} /></div>
          <span>{item.name}</span>
          <dl className="review-fields">
            <div><dt>样本风险</dt><dd>{item.linkedSpecimenMissing
              ? <em className="risk-missing">关联样本缺失（{item.relatedCode || '未关联'}）</em>
              : <RiskTag level={(item.linkedSpecimenRiskLevel || item.riskLevel) as DomainRecord['riskLevel']} />}</dd></div>
            <div><dt>签发风险</dt><dd><RiskTag level={item.riskLevel} /></dd></div>
            <div><dt>首名确认</dt><dd>{item.firstReviewedBy
              ? <>{item.firstReviewedBy} · {formatDate(item.firstReviewedAt || item.updatedAt)}</>
              : <em className="muted">尚未确认</em>}</dd></div>
            <div><dt>待办</dt><dd>{todoFor(item)}</dd></div>
            <div><dt>原因</dt><dd>{item.firstReviewReason || item.reviewReason || <em className="muted">暂无</em>}</dd></div>
          </dl>
          {canAdvance(item)
            ? <button className="table-action" onClick={() => setPending({ item, status: 'signed' })}>{actionLabel(item, 'signed')}</button>
            : <span className="muted">{unavailableReason(item)}</span>}
        </article>)}
      </div>
    </section>}
    {showResultPanel && <section className="result-section"><header><h2>结果与版本证据</h2><span>签发版本、操作者和请求 ID 可追溯</span></header><ResultPanel records={items} /></section>}
    <section className="toolbar"><input aria-label="搜索" placeholder={`搜索${config.label}编码或名称`} value={search} onChange={(event) => setSearch(event.target.value)} /><UiButton onClick={() => void load(config.path, search)}>查询</UiButton><button className="link-button" onClick={() => { setSearch(''); void load(config.path); }}>重置</button></section>
    {error && <div className="alert" role="alert">{error}</div>}
    <section className="table-shell" aria-busy={loading}><table><thead><tr><th>编码</th><th>名称</th><th>状态</th><th>风险</th><th>责任人</th><th>指标</th><th>更新时间</th><th>操作</th></tr></thead><tbody>
      {items.map((item) => { const next = nextStatus(item.status, config.primaryTransitions); return <tr key={item.id}><td><strong>{item.code}</strong></td><td>{item.name}<small>{item.facility}</small></td><td><StatusBadge status={item.status}/></td><td>{showRiskTags || isSignoff ? <RiskTag level={item.riskLevel}/> : item.riskLevel}</td><td>{item.owner}</td><td>{item.metricValue} {item.metricUnit}</td><td>{formatDate(item.updatedAt)}</td><td>{next && canAdvance(item) ? <button className="table-action" onClick={() => setPending({ item, status: next })}>{actionLabel(item, next)}</button> : <span className="muted">{next ? unavailableReason(item) : '流程结束'}</span>}</td></tr>; })}
      {!items.length && !loading && <EmptyState message="暂无记录" colSpan={8} />}
    </tbody></table>{loading && <div className="loading">正在同步业务数据…</div>}</section>
    <ConfirmDialog open={showCreate} title={`新增${config.label}`} onCancel={() => setShowCreate(false)} onConfirm={() => void createDemo().catch(() => undefined)}><p>将创建一条包含完整责任人、风险和证据信息的演示记录。</p></ConfirmDialog>
    <ConfirmDialog open={Boolean(pending)} title="确认状态迁移" onCancel={() => setPending(null)} onConfirm={() => void confirmTransition().catch(() => undefined)}><p>状态迁移会写入不可覆盖的版本与审计日志。</p><strong>{pending?.item.status} → {pending ? displayTarget(pending.item, pending.status) : ''}</strong></ConfirmDialog>
  </main>;
}
