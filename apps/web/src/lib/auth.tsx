"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { api, clearSession, getSession } from "./api";
import type { User } from "./types";

interface AuthState {
  user: User | null;
  loading: boolean;
  isStaff: boolean;
  logout: () => Promise<void>;
  reload: () => Promise<void>;
}

const Ctx = createContext<AuthState>({ user: null, loading: true, isStaff: false, logout: async () => {}, reload: async () => {} });

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    if (!getSession()) {
      setUser(null);
      setLoading(false);
      return;
    }
    try {
      setUser(await api<User>("/me"));
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
    const on = () => void reload();
    window.addEventListener("auth:change", on);
    window.addEventListener("storage", on);
    return () => {
      window.removeEventListener("auth:change", on);
      window.removeEventListener("storage", on);
    };
  }, [reload]);

  const logout = useCallback(async () => {
    const s = getSession();
    if (s) await api("/auth/logout", { body: { refreshToken: s.refresh }, anonymous: true }).catch(() => {});
    clearSession();
  }, []);

  const value = useMemo(
    () => ({ user, loading, isStaff: user?.role === "admin" || user?.role === "owner", logout, reload }),
    [user, loading, logout, reload],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export const useAuth = () => useContext(Ctx);
