"use client";

import { Suspense, useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Send } from "lucide-react";
import { api, errorInfo, setSession } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { safeNext } from "@/lib/format";
import type { Tokens, User } from "@/lib/types";
import { Alert, Button, PageLoader, Spinner } from "@/components/ui";

interface Start {
  code: string;
  secret: string;
  deepLink: string;
  expiresAt: string;
}
interface Poll {
  status: "pending" | "expired" | "confirmed";
  tokens?: Tokens;
  user?: User;
}

function LoginInner() {
  const router = useRouter();
  const next = safeNext(useSearchParams().get("next"), "/");
  const { user, loading } = useAuth();
  const [phase, setPhase] = useState<"idle" | "starting" | "waiting" | "expired">("idle");
  const [link, setLink] = useState("");
  const [error, setError] = useState<string | null>(null);
  const req = useRef<Start | null>(null);

  useEffect(() => {
    if (!loading && user) router.replace(next);
  }, [loading, user, next, router]);

  const poll = useCallback(async () => {
    const r = req.current;
    if (!r) return;
    try {
      const res = await api<Poll>("/auth/telegram/poll", { body: { code: r.code, secret: r.secret }, anonymous: true });
      if (res.status === "confirmed" && res.tokens) {
        req.current = null;
        setSession(res.tokens);
        router.replace(next);
      } else if (res.status === "expired") {
        req.current = null;
        setPhase("expired");
      }
    } catch {
      /* temporary network problem: keep polling */
    }
  }, [next, router]);

  useEffect(() => {
    if (phase !== "waiting") return;
    const t = setInterval(() => void poll(), 2000);
    const vis = () => document.visibilityState === "visible" && void poll();
    document.addEventListener("visibilitychange", vis);
    return () => {
      clearInterval(t);
      document.removeEventListener("visibilitychange", vis);
    };
  }, [phase, poll]);

  async function start() {
    setError(null);
    setPhase("starting");
    try {
      const r = await api<Start>("/auth/telegram/start", { method: "POST", anonymous: true });
      req.current = r;
      setLink(r.deepLink);
      setPhase("waiting");
      window.open(r.deepLink, "_blank", "noopener"); // may be blocked: the visible button below is the fallback
    } catch (e) {
      setError(errorInfo(e).message);
      setPhase("idle");
    }
  }

  if (loading || user) return <PageLoader />;

  return (
    <div className="mx-auto max-w-md py-14 sm:py-20">
      <p className="label-xs">Kirish</p>
      <h1 className="mt-2 text-[26px] font-semibold uppercase tracking-[0.04em]">Telegram orqali kiring</h1>
      <p className="mt-3 text-[13px] text-muted">Parol va ro&apos;yxatdan o&apos;tish shart emas. Buyurtma holati haqida xabarlar ham shu Telegram akkauntingizga keladi.</p>

      <ol className="mt-8 space-y-3 text-[13px]">
        {["Pastdagi tugmani bosing", "Telegram'da «Start» (Boshlash) tugmasini bosing", "Shu sahifaga qaytasiz: avtomatik tizimga kirasiz"].map((t, i) => (
          <li key={t} className="flex gap-3">
            <span className="grid size-6 shrink-0 place-items-center border border-ink text-[11px] font-semibold">{i + 1}</span>
            <span className="pt-0.5">{t}</span>
          </li>
        ))}
      </ol>

      <div className="mt-8 space-y-4">
        {error && <Alert>{error}</Alert>}
        {phase === "expired" && <Alert kind="warn">Havola muddati tugadi. Qaytadan urinib ko&apos;ring.</Alert>}

        {phase === "waiting" ? (
          <>
            <a href={link} target="_blank" rel="noopener noreferrer" className="inline-flex h-12 w-full items-center justify-center gap-2 bg-ink px-7 text-[12px] font-medium uppercase tracking-[0.08em] text-paper hover:bg-black">
              <Send className="size-4" /> Telegram&apos;ni ochish
            </a>
            <p className="flex items-center justify-center gap-2 text-[12px] text-muted">
              <Spinner className="size-4" /> «Start» bosilishi kutilmoqda…
            </p>
            <button onClick={() => void start()} className="mx-auto block text-[12px] uppercase tracking-[0.1em] text-muted underline underline-offset-4 hover:text-ink">
              Yangi havola olish
            </button>
          </>
        ) : (
          <Button size="lg" className="w-full" loading={phase === "starting"} onClick={() => void start()}>
            <Send className="size-4" /> Telegram orqali kirish
          </Button>
        )}
      </div>
      <p className="mt-6 text-[11px] leading-relaxed text-muted">Havola 5 daqiqa amal qiladi. Biz faqat Telegram ismingiz va username&apos;ingizni olamiz.</p>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<PageLoader />}>
      <LoginInner />
    </Suspense>
  );
}
