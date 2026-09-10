import type { ReactNode } from "react";
import { BrowserRouter, NavLink, Route, Routes } from "react-router";
import { AuthProvider, useAuth } from "./auth";

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <AuthProvider>
      <BrowserRouter>{children}</BrowserRouter>
    </AuthProvider>
  );
}

export function AppContent({ shell }: { shell: ReactNode }) {
  const auth = useAuth();
  if (auth.loading) {
    return <main className="center-shell">载入中...</main>;
  }
  if (!auth.user) {
    return <LoginPage />;
  }
  return shell;
}

function LoginPage() {
  return (
    <main className="login-page">
      <section className="login-panel">
        <span className="eyebrow">PRIVATE WORKSPACE</span>
        <h1>请从主站进入</h1>
        <p>图像生成工具使用主站登录信息自动进入。请回到主站菜单点击“图像生成”。</p>
      </section>
    </main>
  );
}

export function Shell({
  workspace,
  settings,
  history,
  adminUsers,
  forbidden
}: {
  workspace: ReactNode;
  settings: ReactNode;
  history: ReactNode;
  adminUsers: ReactNode;
  forbidden: ReactNode;
}) {
  const auth = useAuth();
  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <img src="/togoapi-logo.png" alt="" aria-hidden="true" />
          <strong>TogoAPIImg</strong>
        </div>
        <nav>
          <NavLink to="/" end>
            创作
          </NavLink>
          <NavLink to="/settings">设置</NavLink>
          <NavLink to="/history">历史</NavLink>
          {auth.user?.role === "admin" ? <NavLink to="/admin/users">用户</NavLink> : null}
        </nav>
      </aside>
      <section className="workspace">
        <Routes>
          <Route element={workspace} path="/" />
          <Route element={settings} path="/settings" />
          <Route element={history} path="/history" />
          <Route element={auth.user?.role === "admin" ? adminUsers : forbidden} path="/admin/users" />
        </Routes>
      </section>
    </main>
  );
}

export function PageHeader({ eyebrow, title, username }: { eyebrow: string; title: string; username: string }) {
  const auth = useAuth();
  return (
    <header>
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1>{title}</h1>
      </div>
      <div className="user-menu">
        <div className="user-badge">{username}</div>
        <button type="button" onClick={() => void auth.logout()}>
          退出
        </button>
      </div>
    </header>
  );
}

export function ForbiddenPage({ username }: { username: string }) {
  return (
    <>
      <PageHeader eyebrow="WEB MVP" title="用户管理" username={username} />
      <div className="error page-message">需要管理员权限</div>
    </>
  );
}
