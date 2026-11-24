import type { OrderStatus, ProductStatus } from "./types";

export const SHOP_NAME = process.env.NEXT_PUBLIC_SHOP_NAME ?? "Cosmetics";

/** 85000 -> "85 000" (deterministic, so server and client render the same text) */
export function num(n: number): string {
  return String(Math.round(n)).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export function money(n: number): string {
  return `${num(n)} so'm`;
}

const pad = (n: number) => String(n).padStart(2, "0");

/** Client-side only (uses the browser's time zone). */
export function dateTime(iso: string): string {
  const d = new Date(iso);
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export const STATUS: Record<OrderStatus, { label: string; tone: string; hint: string }> = {
  waiting: { label: "Kutilmoqda", tone: "text-warn", hint: "Admin chekni tekshirmoqda" },
  confirmed: { label: "Tasdiqlandi", tone: "text-info", hint: "To'lov qabul qilindi" },
  shipping: { label: "Yo'lda", tone: "text-violet", hint: "Buyurtma jo'natildi" },
  delivered: { label: "Yetkazildi", tone: "text-ok", hint: "Buyurtma yakunlandi" },
  rejected: { label: "Rad etildi", tone: "text-bad", hint: "Buyurtma rad etildi" },
};

export const PRODUCT_STATUS: Record<ProductStatus, { label: string; tone: string }> = {
  draft: { label: "Qoralama", tone: "text-warn" },
  active: { label: "Faol", tone: "text-ok" },
  archived: { label: "Arxiv", tone: "text-muted" },
};

/** Only allow same-site relative redirects. */
export function safeNext(next: string | null | undefined, fallback = "/"): string {
  if (next && next.startsWith("/") && !next.startsWith("//") && !next.includes("\\")) return next;
  return fallback;
}

export function fullName(u: { firstName: string; lastName: string; username: string }): string {
  const n = `${u.firstName} ${u.lastName}`.trim();
  return n || (u.username ? `@${u.username}` : "Foydalanuvchi");
}
