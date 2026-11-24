"use client";

import { Suspense, useEffect } from "react";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { Check } from "lucide-react";
import { dateTime, money } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import { useRequireAuth } from "@/lib/use-require-auth";
import type { Order, OrderStatus } from "@/lib/types";
import { Alert, AuthedImage, PageLoader, StatusBadge } from "@/components/ui";
import { cx } from "@/lib/cx";

const STEPS: { status: OrderStatus; label: string; hint: string }[] = [
  { status: "waiting", label: "Qabul qilindi", hint: "Admin chekni tekshirmoqda" },
  { status: "confirmed", label: "To'lov tasdiqlandi", hint: "To'lov qabul qilindi" },
  { status: "shipping", label: "Jo'natildi", hint: "Buyurtma yo'lda" },
  { status: "delivered", label: "Yetkazildi", hint: "Buyurtma yakunlandi" },
];

function Inner() {
  const { id } = useParams<{ id: string }>();
  const fresh = useSearchParams().get("new") === "1";
  const { user, loading } = useRequireAuth();
  const { data: o, error, loading: busy, reload } = useApi<Order>(user ? `/orders/${id}` : null);

  const status = o?.status;
  useEffect(() => {
    if (!status || status === "delivered" || status === "rejected") return;
    const t = setInterval(() => void reload(), 20000);
    return () => clearInterval(t);
  }, [status, reload]);

  if (loading || !user || (busy && !o)) return <PageLoader />;
  if (error || !o) {
    return (
      <div className="py-20 text-center">
        <p className="text-[13px] uppercase tracking-[0.1em]">{error ?? "Buyurtma topilmadi"}</p>
        <Link href="/orders" className="mt-4 inline-block underline underline-offset-4">
          Buyurtmalarimga qaytish
        </Link>
      </div>
    );
  }

  const reached = new Map(o.timeline.map((t) => [t.status, t.at]));
  const rejected = o.status === "rejected";
  const idx = STEPS.findIndex((s) => s.status === o.status);

  return (
    <div className="py-8 sm:py-10">
      <Link href="/orders" className="text-[12px] uppercase tracking-[0.1em] text-muted hover:text-ink">
        ← Buyurtmalarim
      </Link>
      <div className="mt-3 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-[26px] font-semibold uppercase tracking-[0.04em]">Buyurtma #{o.number}</h1>
          <p className="mt-1 text-[12px] text-muted">{dateTime(o.createdAt)}</p>
        </div>
        <StatusBadge status={o.status} />
      </div>

      <div className="mt-6 space-y-3">
        {fresh && o.status === "waiting" && (
          <Alert kind="ok">
            <b>Buyurtmangiz qabul qilindi!</b> Admin chekni tekshiradi va siz bilan bog&apos;lanadi. Har bir bosqich haqida Telegram&apos;da xabar olasiz.
          </Alert>
        )}
        {rejected && (
          <Alert>
            <b>Buyurtma rad etildi.</b> {o.rejectReason && <>Sabab: {o.rejectReason}</>}
          </Alert>
        )}
      </div>

      <div className="mt-8 grid gap-8 lg:grid-cols-[1fr_360px] lg:gap-12">
        <div className="space-y-8">
          <section>
            <h2 className="label-xs mb-4">Holat</h2>
            <ol className="border border-line bg-card">
              {STEPS.map((s, i) => {
                const at = reached.get(s.status);
                const done = !!at || (!rejected && i < idx);
                const current = !rejected && i === idx;
                return (
                  <li key={s.status} className={cx("flex items-center gap-4 border-b border-line px-4 py-3.5 last:border-b-0", !done && !current && "text-muted")}>
                    <span className={cx("grid size-6 shrink-0 place-items-center border text-[11px]", done || current ? "border-ink bg-ink text-paper" : "border-line")}>{done ? <Check className="size-3.5" /> : i + 1}</span>
                    <div className="min-w-0 flex-1">
                      <p className="text-[13px] font-medium uppercase tracking-[0.06em]">{s.label}</p>
                      {current && <p className="text-[12px] text-muted">{s.hint}</p>}
                    </div>
                    {at && <span className="text-[11px] text-muted">{dateTime(at)}</span>}
                  </li>
                );
              })}
              {rejected && (
                <li className="flex items-center gap-4 px-4 py-3.5 text-bad">
                  <span className="grid size-6 shrink-0 place-items-center border border-bad text-[11px]">×</span>
                  <p className="flex-1 text-[13px] font-medium uppercase tracking-[0.06em]">Rad etildi</p>
                  {reached.get("rejected") && <span className="text-[11px]">{dateTime(reached.get("rejected")!)}</span>}
                </li>
              )}
            </ol>
          </section>

          <section>
            <h2 className="label-xs mb-4">Mahsulotlar</h2>
            <ul className="divide-y divide-line border border-line bg-card">
              {o.items.map((it) => (
                <li key={it.variantId} className="flex items-center gap-4 p-4">
                  {it.image ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={it.image.thumb} alt="" className="size-16 border border-line object-cover" />
                  ) : (
                    <div className="size-16 border border-line bg-paper" />
                  )}
                  <div className="min-w-0 flex-1">
                    <p className="text-[13px] font-medium uppercase tracking-[0.04em]">{it.productName}</p>
                    {it.variantLabel && <p className="text-[12px] text-muted">{it.variantLabel}</p>}
                    <p className="mt-1 text-[12px] text-muted">
                      {money(it.unitPrice)} × {it.quantity}
                    </p>
                  </div>
                  <p className="text-[13px] font-semibold">{money(it.lineTotal)}</p>
                </li>
              ))}
              <li className="flex items-baseline justify-between bg-paper px-4 py-3.5">
                <span className="label-xs">Jami</span>
                <span className="text-[18px] font-semibold">{money(o.totalAmount)}</span>
              </li>
            </ul>
          </section>
        </div>

        <aside className="space-y-8">
          <section>
            <h2 className="label-xs mb-4">Yetkazib berish</h2>
            <dl className="space-y-3 border border-line bg-card p-4 text-[13px]">
              <Row k="Qabul qiluvchi" v={o.customerName} />
              <Row k="Telefon" v={o.phone} />
              <Row k="Manzil" v={o.addressText} />
              {o.comment && <Row k="Izoh" v={o.comment} />}
            </dl>
          </section>
          <section>
            <h2 className="label-xs mb-4">To&apos;lov cheki</h2>
            <AuthedImage path={`/files/receipts/${o.receiptFileId}`} alt={`Chek, buyurtma #${o.number}`} className="max-h-80 w-full" />
          </section>
        </aside>
      </div>
    </div>
  );
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div>
      <dt className="label-xs">{k}</dt>
      <dd className="mt-0.5 break-words">{v}</dd>
    </div>
  );
}

export default function OrderPage() {
  return (
    <Suspense fallback={<PageLoader />}>
      <Inner />
    </Suspense>
  );
}
