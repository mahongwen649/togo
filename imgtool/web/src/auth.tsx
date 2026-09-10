import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { CurrentUser } from "./api";
import { getMe, login as loginRequest, logout as logoutRequest } from "./api";

type AuthState = {
  user?: CurrentUser;
  loading: boolean;
  error?: string;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthState | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<CurrentUser>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  useEffect(() => {
    let alive = true;
    getMe()
      .then((payload) => {
        if (!alive) return;
        if (payload.ok) setUser(payload.data.user);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  const value = useMemo<AuthState>(
    () => ({
      user,
      loading,
      error,
      async login(username, password) {
        setError(undefined);
        const payload = await loginRequest(username, password);
        if (!payload.ok) {
          setError(payload.error?.message ?? "登录失败");
          return;
        }
        setUser(payload.data.user);
      },
      async logout() {
        await logoutRequest();
        setUser(undefined);
      }
    }),
    [error, loading, user]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth must be used within AuthProvider");
  return context;
}
