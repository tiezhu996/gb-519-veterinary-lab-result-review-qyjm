
export function EmptyState({ message = '暂无数据', colSpan }: { message?: string; colSpan?: number }) {
  if (colSpan) return <tr><td colSpan={colSpan} className="empty">{message}</td></tr>;
  return <div className="empty">{message}</div>;
}
