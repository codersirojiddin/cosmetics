"use client";

import Link from "next/link";
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from "react";
import { Check, Loader2, X } from "lucide-react";
import { apiBlob } from "@/lib/api";
import { cx } from "@/lib/cx";
import { STATUS } from "@/lib/format";
import type { OrderStatus } from "@/lib/types";

export { cx };

// ---------- buttons ----------

const base =
  "inline-flex items-center justify-center gap-2 font-medium uppercase tracking-[0.08em] text-[12px] transition-colors disabled:opacity-50 select-none whitespace-nowrap";
const variants = {
  primary: "bg-ink text-paper border border-ink hover:bg-black",
  secondary: "bg-transparent text-ink border border-ink hover:bg-ink hover:text-paper",
  accent: "bg-accent text-white border border-accent hover:brightness-95",
  ghost: "text-ink border border-transparent hover:bg-ink/5",
  danger: "text-bad border border-bad hover:bg-bad hover:text-white",
};
const sizes = { sm: "h-9 px-3", md: "h-11 px-5", lg: "h-12 px-7" };

type BtnProps = { variant?: keyof typeof variants; size?: keyof typeof sizes; loading?: boolean };

export function Button({ variant = "primary", size = "md", loading, className, children, disabled, type = "button", ...rest }: BtnProps & ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button type={type} disabled={disabled || loading} className={cx(base, variants[variant], sizes[size], className)} {...rest}>
      {loading && <Loader2 className="size-4 animate-spin" aria-hidden />}
      {children}
    </button>
  );
}

export function LinkButton({ href, variant = "primary", size = "md", className, children }: BtnProps & { href: string; className?: string; children: ReactNode }) {
  return (
    <Link href={href} className={cx(base, variants[variant], sizes[size], className)}>
      {children}
    </Link>
  );
}

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cx("size-5 animate-spin text-muted", className)} aria-label="Yuklanmoqda" />;
}

// ---------- forms ----------

export const inputCls =
  "w-full bg-card border border-line px-3 h-11 text-[14px] placeholder:text-muted/70 outline-none transition-colors focus:border-ink disabled:bg-paper disabled:text-muted aria-[invalid=true]:border-bad";

export function Field({ label, error, hint, required, children, className }: { label: string; error?: string; hint?: string; required?: boolean; children: ReactNode; className?: string }) {
  return (
    <label className={cx("block", className)}>
      <span className="label-xs mb-1.5 block">
        {label}
        {required && <span className="text-accent"> *</span>}
      </span>
      {children}
      {hint && !error && <span className="mt-1 block text-[12px] text-muted">{hint}</span>}
      {error && <span className="mt-1 block text-[12px] text-bad">{error}</span>}
    </label>
  );
}

export function Input({ className, invalid, ...p }: InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }) {
  return <input aria-invalid={invalid || undefined} className={cx(inputCls, className)} {...p} />;
}

export function Textarea({ className, invalid, ...p }: TextareaHTMLAttributes<HTMLTextAreaElement> & { invalid?: boolean }) {
  return <textarea aria-invalid={invalid || undefined} className={cx(inputCls, "h-auto py-2.5 leading-relaxed", className)} {...p} />;
}

export function Select({ className, invalid, ...p }: SelectHTMLAttributes<HTMLSelectElement> & { invalid?: boolean }) {
  return <select aria-invalid={invalid || undefined} className={cx(inputCls, "pr-8", className)} {...p} />;
}

// ---------- display ----------

export function StatusBadge({ status }: { status: OrderStatus }) {
  const s = STATUS[status];
  return (
    <span className={cx("inline-flex items-center gap-1.5 text-[12px] font-medium uppercase tracking-[0.08em]", s.tone)}>
      <span className="size-2 rounded-full bg-current" aria-hidden />
      {s.label}
    </span>
  );
}

export function Tag({ children, tone = "ink" }: { children: ReactNode; tone?: "ink" | "accent" | "muted" }) {
  const t = { ink: "bg-ink text-paper", accent: "bg-accent text-white", muted: "border border-line text-muted" }[tone];
  return <span className={cx("inline-block px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.12em]", t)}>{children}</span>;
}

export function Alert({ kind = "error", children }: { kind?: "error" | "info" | "warn" | "ok"; children: ReactNode }) {
  const c = { error: "border-bad text-bad bg-bad/5", info: "border-info text-info bg-info/5", warn: "border-warn text-warn bg-warn/5", ok: "border-ok text-ok bg-ok/5" }[kind];
  return (
    <div role={kind === "error" ? "alert" : "status"} className={cx("border px-3 py-2.5 text-[13px] leading-snug", c)}>
      {children}
    </div>
  );
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="border border-dashed border-line px-6 py-14 text-center">
      <p className="text-[13px] font-medium uppercase tracking-[0.1em]">{title}</p>
      {children && <div className="mx-auto mt-2 max-w-sm text-[13px] text-muted">{children}</div>}
    </div>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cx("animate-pulse bg-line/60", className)} />;
}

export function PageLoader() {
  return (
    <div className="flex justify-center py-24">
      <Spinner className="size-6" />
    </div>
  );
}

export function Modal({ open, onClose, title, children, wide }: { open: boolean; onClose: () => void; title: string; children: ReactNode; wide?: boolean }) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink/50 p-0 sm:items-center sm:p-4" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div role="dialog" aria-modal="true" aria-label={title} className={cx("max-h-[92vh] w-full overflow-auto bg-card shadow-2xl", wide ? "sm:max-w-3xl" : "sm:max-w-md")}>
        <div className="sticky top-0 flex items-center justify-between border-b border-line bg-card px-5 py-3">
          <h2 className="text-[12px] font-semibold uppercase tracking-[0.12em]">{title}</h2>
          <button onClick={onClose} aria-label="Yopish" className="-mr-2 p-2 text-muted hover:text-ink">
            <X className="size-4" />
          </button>
        </div>
        <div className="p-5">{children}</div>
      </div>
    </div>
  );
}

export function Pagination({ page, limit, total, onPage }: { page: number; limit: number; total: number; onPage: (p: number) => void }) {
  const pages = Math.max(1, Math.ceil(total / limit));
  if (pages <= 1) return null;
  return (
    <div className="mt-6 flex items-center justify-between text-[12px]">
      <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        ← Oldingi
      </Button>
      <span className="text-muted">
        {page} / {pages}
      </span>
      <Button variant="secondary" size="sm" disabled={page >= pages} onClick={() => onPage(page + 1)}>
        Keyingi →
      </Button>
    </div>
  );
}

// ---------- toasts ----------

interface Toast {
  id: number;
  text: string;
  kind: "ok" | "error";
}
const ToastCtx = createContext<(text: string, kind?: Toast["kind"]) => void>(() => {});
export const useToast = () => useContext(ToastCtx);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([]);
  const seq = useRef(0);
  const push = useCallback((text: string, kind: Toast["kind"] = "ok") => {
    const id = ++seq.current;
    setItems((l) => [...l, { id, text, kind }]);
    setTimeout(() => setItems((l) => l.filter((t) => t.id !== id)), 4000);
  }, []);
  return (
    <ToastCtx.Provider value={push}>
      {children}
      <div className="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4" aria-live="polite">
        {items.map((t) => (
          <div key={t.id} className={cx("pointer-events-auto flex max-w-md items-center gap-2 px-4 py-2.5 text-[13px] text-white shadow-lg", t.kind === "ok" ? "bg-ink" : "bg-bad")}>
            {t.kind === "ok" && <Check className="size-4 shrink-0" />}
            {t.text}
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  );
}

// ---------- protected images (receipts need the Bearer header) ----------

export function AuthedImage({ path, alt, className }: { path: string; alt: string; className?: string }) {
  const [url, setUrl] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const [open, setOpen] = useState(false);
  useEffect(() => {
    let revoke: string | null = null;
    let live = true;
    apiBlob(path)
      .then((b) => {
        if (!live) return;
        revoke = URL.createObjectURL(b);
        setUrl(revoke);
      })
      .catch(() => live && setFailed(true));
    return () => {
      live = false;
      if (revoke) URL.revokeObjectURL(revoke);
    };
  }, [path]);
  if (failed) return <Alert>Rasmni yuklab bo&apos;lmadi</Alert>;
  if (!url) return <Skeleton className={cx("h-64", className)} />;
  return (
    <>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={url} alt={alt} onClick={() => setOpen(true)} className={cx("cursor-zoom-in border border-line bg-white object-contain", className)} />
      <Modal open={open} onClose={() => setOpen(false)} title={alt} wide>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={url} alt={alt} className="mx-auto max-h-[75vh] w-auto" />
      </Modal>
    </>
  );
}
