
import { NavLink, Outlet } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
const navigation = [{ to: '/cases', label: '动物样本来源' }, { to: '/specimens', label: '检验样本' }, { to: '/assays', label: '检测运行' }, { to: '/signoff', label: '结果签发' }, { to: '/audit', label: '审计记录', reviewerOnly: true }];
export default function App() {
  const { session, loading, signOut, hasRole } = useAuth();
  if (loading) return <div className="app-loading">正在建立安全会话…</div>;
  return <div className="app-shell"><aside><div className="brand"><span>CONTROL DESK</span><strong>兽医检验样本结果复核</strong></div><nav>{navigation.filter((item) => !item.reviewerOnly || hasRole('reviewer')).map((item) => <NavLink key={item.to} to={item.to}>{item.label}</NavLink>)}</nav><div className="user-panel"><span>{session?.displayName}</span><small>{session?.role}</small><button onClick={signOut}>退出会话</button></div></aside><section className="content"><header className="topbar"><span>检验运行态势</span><span className="live-dot">服务已连接</span></header><Outlet /></section></div>;
}
