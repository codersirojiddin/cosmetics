"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useContext, useEffect, useMemo, useRef, type ReactNode } from "react";
import { ClipboardList, ExternalLink, LogOut, Package, ScrollText, Settings as Cog, Tags, Users } from "lucide-react";
import { useHeaderless } from "./use-headerless";
import { useApi } from "@/lib/hooks";
import { useRequireAuth } from "@/lib/use-require-auth";
import { SHOP_NAME, fullName } from "@/lib/format";
import { cx } from "@/lib/cx";
import type { Stats } from "@/lib/types";
import { PageLoader, useToast } from "../ui";

interface AdminCtx {
  stats: Stats | null;
  reloadStats: () => void;
}
const Ctx = createContext<AdminCtx>({ stats: null, reloadStats: () => {} });
export const useAdmin = () => useContext(Ctx);

function beep() {
  try {
    const ac = new AudioContext();
    const o = ac.createOscillator();
    const g = ac.createGain();
    o.connect(g);
    g.connect(ac.destination);
    o.frequency.value = 880;
    g.gain.value = 0.06;
    o.start();
    o.stop(ac.currentTime + 0.2);
  } catch {
    /* audio may be blocked until the first click: ignore */
  }
}

const NAV = [
  { href: "/admin/orders", label: "Buyurtmalar", icon: ClipboardList, badge: true },
  { href: "/admin/products", label: "Mahsulotlar", icon: Package },
  { href: "/admin/categories", label: "Kategoriyalar", icon: Tags },
  { href: "/admin/settings", label: "Sozlamalar", icon: Cog },
  { href: "/admin/admins", label: "Adminlar", icon: Users, owner: true },
  { href: "/admin/audit", label: "Jurnal", icon: ScrollText, owner: true },
];

export function AdminShell({ children }: { children: ReactNode }) {
  useHeaderless();
  const { user, loading, isStaff, logout } = useRequireAuth();
  const pathname = usePathname();
  const router = useRouter();
  const toast = useToast();
  const stats = useApi<Stats>(isStaff ? "/admin/stats" : null);
  const { reload } = stats;
  const prev = useRef<number | null>(null);

  useEffect(() => {
    if (!isStaff) return;
    const t = setInterval(() => void reload(), 15000);
    return () => clearInterval(t);
  }, [isStaff, reload]);

  useEffect(() => {
    const w = stats.data?.counts.waiting;
    if (w === undefined) return;
    if (prev.current !== null && w > prev.current) {
      toast("Yangi buyurtma keldi!");
      beep();
    }
    prev.current = w;
    document.title = `${w > 0 ? `(${w}) ` : ""}Admin | ${SHOP_NAME}`;
  }, [stats.data, toast]);

  const ctx = useMemo(() => ({ stats: stats.data, reloadStats: () => void reload() }), [stats.data, reload]);

  if (loading || !user) return <PageLoader />;
  if (!isStaff) {
    return (
      <div className="mx-auto max-w-md px-4 py-24 text-center">
        <p className="text-[13px] font-semibold uppercase tracking-[0.12em]">Ruxsat yo&apos;q</p>
        <p className="mt-3 text-[13px] text-muted">Bu bo&apos;lim faqat adminlar uchun. Admin bo&apos;lish uchun do&apos;kon egasidan taklif havolasini so&apos;rang.</p>
        <Link href="/" className="mt-6 inline-block underline underline-offset-4">
          Bosh sahifa
        </Link>
      </div>
    );
  }

  const items = NAV.filter((n) => !n.owner || user.role === "owner");
  const waiting = stats.data?.counts.waiting ?? 0;
  const active = (href: string) => pathname === href || pathname.startsWith(href + "/");

  const Nav = ({ row }: { row?: boolean }) => (
    <nav className={cx(row ? "flex gap-1 overflow-x-auto px-3 pb-2" : "flex flex-col gap-0.5 px-3")}>
      {items.map((n) => (
        <Link
          key={n.href}
          href={n.href}
          className={cx(
            "flex shrink-0 items-center gap-2.5 px-3 py-2.5 text-[12px] font-medium uppercase tracking-[0.1em] transition-colors",
            active(n.href) ? "bg-ink text-paper" : "text-ink hover:bg-ink/5",
          )}
        >
          <n.icon className="size-4" />
          {n.label}
          {n.badge && waiting > 0 && <span className="ml-auto bg-accent px-1.5 py-px text-[10px] text-white">{waiting}</span>}
        </Link>
      ))}
    </nav>
  );

  return (
    <Ctx.Provider value={ctx}>
      <div className="min-h-dvh lg:grid lg:grid-cols-[250px_1fr]">
        <aside className="sticky top-0 hidden h-dvh flex-col border-r border-line bg-card lg:flex">
          <div className="px-6 py-6">
            <p className="text-[15px] font-semibold uppercase tracking-[0.22em]">{SHOP_NAME}</p>
            <p className="label-xs mt-1">Admin panel</p>
          </div>
          <Nav />
          <div className="mt-auto space-y-1 border-t border-line p-3">
            <p className="px-3 pt-2 text-[12px] font-medium">{fullName(user)}</p>
            <p className="label-xs px-3 pb-2">{user.role === "owner" ? "Owner" : "Admin"}</p>
            <Link href="/" className="flex items-center gap-2.5 px-3 py-2 text-[12px] uppercase tracking-[0.1em] text-muted hover:text-ink">
              <ExternalLink className="size-4" /> Saytga qaytish
            </Link>
            <button
              onClick={async () => {
                await logout();
                router.push("/");
              }}
              className="flex w-full items-center gap-2.5 px-3 py-2 text-[12px] uppercase tracking-[0.1em] text-muted hover:text-ink"
            >
              <LogOut className="size-4" /> Chiqish
            </button>
          </div>
        </aside>

        <div className="min-w-0">
          <header className="sticky top-0 z-30 border-b border-line bg-paper/95 backdrop-blur lg:hidden">
            <div className="flex items-center justify-between px-4 py-3">
              <p className="text-[14px] font-semibold uppercase tracking-[0.2em]">{SHOP_NAME} · admin</p>
              <div className="flex items-center gap-3 text-muted">
                <Link href="/" aria-label="Saytga qaytish">
                  <ExternalLink className="size-4" />
                </Link>
                <button
                  aria-label="Chiqish"
                  onClick={async () => {
                    await logout();
                    router.push("/");
                  }}
                >
                  <LogOut className="size-4" />
                </button>
              </div>
            </div>
            <Nav row />
          </header>
          <main className="mx-auto max-w-6xl px-4 py-6 lg:px-10 lg:py-9">{children}</main>
        </div>
      </div>
    </Ctx.Provider>
  );
}
