import type { DomainRecord } from '../../types/domain';

const labels: Record<DomainRecord['riskLevel'], string> = {
  low: '低风险',
  medium: '中风险',
  high: '高风险',
  critical: '关键风险',
};

export function RiskTag({ level }: { level: DomainRecord['riskLevel'] }) {
  return <span className={`risk-tag risk-tag--${level}`}><i aria-hidden="true" />{labels[level]}</span>;
}
