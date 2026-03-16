"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { usePathname, useRouter } from "next/navigation";

interface User {
  id: string;
  github_id: number;
  github_login: string;
  name?: string;
  avatar_url?: string;
}

interface AuthContextValue {
  user: User | null;
  loading: boolean;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  loading: true,
  logout: async () => {},
});

export function useAuth() {
  return useContext(AuthContext);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    fetch("/auth/me")
      .then((res) => {
        if (res.ok) return res.json();
        throw new Error("unauthorized");
      })
      .then((data) => {
        setUser(data);
        setLoading(false);
      })
      .catch(() => {
        setUser(null);
        setLoading(false);
        if (pathname !== "/login") {
          router.replace("/login");
        }
      });
  }, [pathname, router]);

  const logout = async () => {
    await fetch("/auth/logout", { method: "POST" });
    setUser(null);
    router.replace("/login");
  };

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-[#f8f8f6]">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-[#e8601a] border-t-transparent" />
      </div>
    );
  }

  // On login page, render without redirect guard
  if (pathname === "/login") {
    return (
      <AuthContext.Provider value={{ user, loading, logout }}>
        {children}
      </AuthContext.Provider>
    );
  }

  // Not logged in and not on login page — handled by useEffect redirect above
  if (!user) {
    return null;
  }

  return (
    <AuthContext.Provider value={{ user, loading, logout }}>
      {children}
    </AuthContext.Provider>
  );
}
