"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { ChevronDown, Search, User as UserIcon } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { SHOP_NAME, fullName } from "@/lib/format";
import { cx } from "@/lib/cx";

export function Header() {
  const { user, loading, isStaff, logout } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => {
    const on = (e: MouseEvent) => ref.current && !ref.current.contains(e.target as Node) && setOpen(false);
    document.addEventListener("mousedown", on);
    return () => document.removeEventListener("mousedown", on);
  }, []);

  return (
    <header className="sticky top-0 z-40 border-b border-line bg-paper/95 backdrop-blur">
      <div className="bg-ink px-4 py-1.5 text-center text-[11px] uppercase tracking-[0.14em] text-paper">
        Telegram orqali kiring · Chek yuklab to&apos;lang · Yetkazib beramiz
      </div>
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-4">
        <Link href="/" className="text-[17px] font-semibold uppercase tracking-[0.22em]">
          {SHOP_NAME}
        </Link>
        <nav className="flex items-center gap-1 sm:gap-3">
          <Link href="/catalog" className={cx("px-2 py-2 text-[12px] font-medium uppercase tracking-[0.12em] hover:text-accent", pathname.startsWith("/catalog") && "text-accent")}>
            Katalog
          </Link>
          <Link href="/catalog?focus=search" aria-label="Qidirish" className="p-2 hover:text-accent">
            <Search className="size-[18px]" />
          </Link>
          <div ref={ref} className="relative">
            {loading ? (
              <div className="h-9 w-20" />
            ) : user ? (
              <>
                <button onClick={() => setOpen((o) => !o)} aria-expanded={open} className="flex h-9 items-center gap-1.5 border border-line bg-card px-3 text-[12px] font-medium uppercase tracking-[0.08em] hover:border-ink">
                  <UserIcon className="size-4" />
                  <span className="max-w-24 truncate">{fullName(user)}</span>
                  <ChevronDown className="size-3.5" />
                </button>
                {open && (
                  <div className="absolute right-0 mt-2 w-52 border border-line bg-card py-1 shadow-lg">
                    <MenuLink href="/orders">Buyurtmalarim</MenuLink>
                    {isStaff && <MenuLink href="/admin">Admin panel</MenuLink>}
                    <button
                      onClick={async () => {
                        await logout();
                        setOpen(false);
                        router.push("/");
                      }}
                      className="block w-full px-4 py-2.5 text-left text-[12px] uppercase tracking-[0.1em] text-muted hover:bg-ink/5 hover:text-ink"
                    >
                      Chiqish
                    </button>
                  </div>
                )}
              </>
            ) : (
              <Link href={`/login?next=${encodeURIComponent(pathname)}`} className="flex h-9 items-center border border-ink px-4 text-[12px] font-medium uppercase tracking-[0.1em] hover:bg-ink hover:text-paper">
                Kirish
              </Link>
            )}
          </div>
        </nav>
      </div>
    </header>
  );
}

function MenuLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link href={href} className="block px-4 py-2.5 text-[12px] uppercase tracking-[0.1em] hover:bg-ink/5">
      {children}
    </Link>
  );
}
