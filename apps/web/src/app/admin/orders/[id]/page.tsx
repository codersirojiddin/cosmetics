"use client";

import { useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { AlertTriangle, Copy, MapPin, Phone, Send } from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { dateTime, money, STATUS } from "@/lib/format";
import { useAction, useApi } from "@/lib/hooks";
import type { AdminOrder, OrderStatus } from "@/lib/types";
import { Alert, AuthedImage, Button, Modal, PageLoader, StatusBadge, Textarea, useToast } from "@/components/ui";
import { KV, Panel } from "@/components/admin/bits";
import { useAdmin } from "@/components/admin/shell";

type Variant = "primary" | "secondary" | "danger";
const ACTIONS: Record<OrderStatus, { to: OrderStatus; label: string; variant: Variant; ask: string }[]> = {
  waiting: [
    { to: "confirmed", label: "To'lovni tasdiqlash", variant: "primary", ask: "To'lov qabul qilindi deb belgilaysizmi? Mijozga Telegram'da xabar boradi." },
    { to: "shipping", label: "Jo'natildi", variant: "secondary", ask: "Buyurtma jo'natildi deb belgilaysizmi? Mijozga xabar boradi." },
    { to: "rejected", label: "Rad etish", variant: "danger", ask: "Buyurtmani rad etasizmi? Mahsulot omborga qaytariladi va mijozga sabab bilan xabar boradi." },
  ],
  confirmed: [
    { to: "shipping", label: "Jo'natildi", variant: "primary", ask: "Buyurtma jo'natildi deb belgilaysizmi? Mijozga xabar boradi." },
    { to: "rejected", label: "Rad etish", variant: "danger", ask: "Buyurtmani rad etasizmi? Mahsulot omborga qaytariladi va mijozga sabab bilan xabar boradi." },
  ],
  shipping: [{ to: "delivered", label: "Yetkazildi", variant: "primary", ask: "Mijoz buyurtmani oldi, buyurtma yakunlansinmi?" }],
  delivered: [],
  rejected: [],
};

export default function AdminOrderPage() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const { reloadStats } = useAdmin();
  const toast = useToast();
  const { data: o, error, loading, reload, setData } = useApi<AdminOrder>(`/admin/orders/${id}`);
  const act = useAction();
  const [modal, setModal] = useState<(typeof ACTIONS)[OrderStatus][number] | null>(null);
  const [comment, setComment] = useState("");
  const [note, setNote] = useState("");

  if (loading && !o) return <PageLoader />;
  if (!o) {
    return (
      <div className="py-16 text-center">
        <p className="text-[13px] uppercase tracking-[0.1em]">{error ?? "Buyurtma topilmadi"}</p>
        <Link href="/admin/orders" className="mt-4 inline-block underline underline-offset-4">
          Ro&apos;yxatga qaytish
        </Link>
      </div>
    );
  }

  const isOwner = user?.role === "owner";
  const mine = o.assignedAdmin?.id === user?.id;
  const lockedByOther = !!o.assignedAdmin && !mine && !isOwner;
  const final = o.status === "delivered" || o.status === "rejected";

  const apply = (next: AdminOrder | undefined, msg: string) => {
    if (next) {
      setData(next);
      reloadStats();
      toast(msg);
    }
  };

  const claim = async (force = false) =>
    apply(await act.run(() => api<AdminOrder>(`/admin/orders/${o.id}/claim`, { method: "POST", body: force ? { force: true } : undefined })), "Buyurtma sizga biriktirildi");

  const change = async () => {
    if (!modal) return;
    const next = await act.run(() =>
      api<AdminOrder>(`/admin/orders/${o.id}/status`, { body: { to: modal.to, expectedStatus: o.status, comment: modal.to === "rejected" ? "" : comment.trim(), rejectReason: modal.to === "rejected" ? comment.trim() : "" } }),
    );
    if (next) {
      setModal(null);
      setComment("");
      apply(next, `Holat: ${STATUS[next.status].label}`);
    }
  };

  const addNote = async () => {
    if (!note.trim()) return;
    const next = await act.run(() => api<AdminOrder>(`/admin/orders/${o.id}/notes`, { body: { body: note.trim() } }));
    if (next) {
      setNote("");
      apply(next, "Izoh qo'shildi");
    }
  };

  const tel = o.phone.replace(/[^+\d]/g, "");
  const maps = [
    ["OpenStreetMap", `https://www.openstreetmap.org/?mlat=${o.lat}&mlon=${o.lng}#map=17/${o.lat}/${o.lng}`],
    ["Google Maps", `https://www.google.com/maps?q=${o.lat},${o.lng}`],
    ["Yandex", `https://yandex.com/maps/?pt=${o.lng},${o.lat}&z=17&l=map`],
  ];

  return (
    <>
      <Link href="/admin/orders" className="text-[12px] uppercase tracking-[0.1em] text-muted hover:text-ink">
        ← Buyurtmalar
      </Link>
      <div className="mb-6 mt-3 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-[24px] font-semibold uppercase tracking-[0.04em]">Buyurtma #{o.number}</h1>
          <p className="mt-1 text-[12px] text-muted">{dateTime(o.createdAt)}</p>
        </div>
        <StatusBadge status={o.status} />
      </div>

      <div className="mb-6 space-y-3">
        {o.receiptDuplicate && (
          <Alert kind="warn">
            <span className="flex items-start gap-2">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <span>
                <b>Diqqat:</b> bu chek{" "}
                <Link href={`/admin/orders/${o.receiptDuplicate.orderId}`} className="underline underline-offset-4">
                  #{o.receiptDuplicate.number}
                </Link>{" "}
                buyurtmadagi chek bilan {o.receiptDuplicate.kind === "exact" ? "aynan bir xil" : "juda o'xshash"}. Pul haqiqatan tushganini tekshiring.
              </span>
            </span>
          </Alert>
        )}
        {o.status === "rejected" && o.rejectReason && <Alert>Rad etish sababi: {o.rejectReason}</Alert>}
        {act.error && <Alert>{act.error} <button onClick={() => void reload()} className="ml-1 underline underline-offset-4">Yangilash</button></Alert>}
      </div>

      <div className="grid gap-6 lg:grid-cols-[1fr_380px]">
        <div className="space-y-6">
          <Panel title="To'lov cheki">
            <AuthedImage path={`/files/receipts/${o.receiptFileId}`} alt={`Chek, buyurtma #${o.number}`} className="max-h-[560px] w-full" />
            <p className="mt-2 text-[12px] text-muted">Rasmni bosib kattalashtiring. Kutilayotgan summa: <b className="text-ink">{money(o.totalAmount)}</b></p>
          </Panel>

          <Panel title="Mahsulotlar">
            <ul className="divide-y divide-line">
              {o.items.map((it) => (
                <li key={it.variantId} className="flex items-center gap-4 py-3 first:pt-0">
                  {it.image ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={it.image.thumb} alt="" className="size-14 border border-line object-cover" />
                  ) : (
                    <div className="size-14 border border-line bg-paper" />
                  )}
                  <div className="min-w-0 flex-1">
                    <p className="text-[13px] font-medium uppercase tracking-[0.04em]">{it.productName}</p>
                    {it.variantLabel && <p className="text-[12px] text-muted">{it.variantLabel}</p>}
                    <p className="text-[12px] text-muted">
                      {money(it.unitPrice)} × {it.quantity}
                    </p>
                  </div>
                  <p className="text-[13px] font-semibold">{money(it.lineTotal)}</p>
                </li>
              ))}
            </ul>
            <div className="mt-2 flex items-baseline justify-between border-t border-line pt-3">
              <span className="label-xs">Jami</span>
              <span className="text-[18px] font-semibold">{money(o.totalAmount)}</span>
            </div>
          </Panel>

          <Panel title="Ichki izohlar (mijoz ko'rmaydi)">
            {o.notes.length > 0 && (
              <ul className="mb-4 space-y-3">
                {o.notes.map((n) => (
                  <li key={n.id} className="border-l-2 border-line pl-3 text-[13px]">
                    <p>{n.body}</p>
                    <p className="mt-0.5 text-[11px] text-muted">
                      {n.author} · {dateTime(n.createdAt)}
                    </p>
                  </li>
                ))}
              </ul>
            )}
            <Textarea rows={2} value={note} onChange={(e) => setNote(e.target.value)} placeholder="Masalan: qo'ng'iroq qildim, ertaga olib boradi" maxLength={1000} />
            <Button className="mt-2" size="sm" variant="secondary" onClick={() => void addNote()} disabled={!note.trim() || act.busy}>
              Izoh qo&apos;shish
            </Button>
          </Panel>
        </div>

        <div className="space-y-6">
          <Panel title="Holatni boshqarish">
            {o.assignedAdmin ? (
              <p className="mb-4 text-[13px]">
                Mas&apos;ul: <b>{mine ? "Siz" : o.assignedAdmin.name}</b>
              </p>
            ) : (
              !final && <p className="mb-4 text-[13px] text-muted">Buyurtmani hali hech kim olmagan.</p>
            )}

            {!final && !o.assignedAdmin && (
              <Button className="mb-3 w-full" variant="accent" onClick={() => void claim()} loading={act.busy}>
                ✋ Men olaman
              </Button>
            )}
            {!final && lockedByOther && <Alert kind="info">Bu buyurtmani {o.assignedAdmin!.name} olgan. Faqat u (yoki owner) holatni o&apos;zgartira oladi.</Alert>}
            {!final && isOwner && o.assignedAdmin && !mine && (
              <Button className="mb-3 w-full" variant="secondary" size="sm" onClick={() => void claim(true)} loading={act.busy}>
                Menga o&apos;tkazish
              </Button>
            )}

            {!lockedByOther && (
              <div className="flex flex-col gap-2">
                {ACTIONS[o.status].map((a) => (
                  <Button key={a.to} variant={a.variant} className="w-full" disabled={act.busy} onClick={() => { act.setError(null); setComment(""); setModal(a); }}>
                    {a.label}
                  </Button>
                ))}
              </div>
            )}
            {final && <p className="text-[13px] text-muted">Buyurtma yakunlangan, holatni o&apos;zgartirib bo&apos;lmaydi.</p>}
          </Panel>

          <Panel title="Mijoz">
            <dl className="space-y-3">
              <KV k="Ism">{o.customerName}</KV>
              <KV k="Telefon">
                <span className="flex flex-wrap items-center gap-2">
                  <a href={`tel:${tel}`} className="inline-flex items-center gap-1.5 font-medium underline underline-offset-4 hover:text-accent">
                    <Phone className="size-3.5" /> {o.phone}
                  </a>
                  <button onClick={() => navigator.clipboard?.writeText(o.phone).then(() => toast("Nusxalandi"))} aria-label="Raqamni nusxalash" className="p-1 text-muted hover:text-ink">
                    <Copy className="size-3.5" />
                  </button>
                </span>
              </KV>
              <KV k="Telegram">
                {o.customer.username ? (
                  <a href={`https://t.me/${o.customer.username}`} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 underline underline-offset-4 hover:text-accent">
                    <Send className="size-3.5" /> @{o.customer.username}
                  </a>
                ) : (
                  <span className="text-muted">username yo&apos;q (ID: {o.customer.telegramId})</span>
                )}
              </KV>
              <KV k="Manzil">{o.addressText}</KV>
              <KV k="Joylashuv">
                <span className="flex flex-wrap gap-x-3 gap-y-1">
                  {maps.map(([name, href]) => (
                    <a key={name} href={href} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 underline underline-offset-4 hover:text-accent">
                      <MapPin className="size-3.5" /> {name}
                    </a>
                  ))}
                </span>
              </KV>
              {o.comment && <KV k="Mijoz izohi">{o.comment}</KV>}
            </dl>
          </Panel>

          <Panel title="Tarix">
            <ol className="space-y-3">
              {o.history.map((h, i) => (
                <li key={i} className="text-[13px]">
                  <p className="font-medium">
                    {STATUS[h.to].label}
                    <span className="ml-2 text-[11px] font-normal text-muted">{h.changedBy || "tizim"}</span>
                  </p>
                  {h.comment && <p className="text-muted">{h.comment}</p>}
                  <p className="text-[11px] text-muted">{dateTime(h.at)}</p>
                </li>
              ))}
            </ol>
          </Panel>
        </div>
      </div>

      <Modal open={!!modal} onClose={() => setModal(null)} title={modal?.label ?? ""}>
        <p className="text-[13px]">{modal?.ask}</p>
        <div className="mt-4">
          <Textarea
            rows={3}
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder={modal?.to === "rejected" ? "Rad etish sababi (majburiy, mijozga ko'rsatiladi)" : "Izoh (ixtiyoriy)"}
            aria-label="Izoh"
            invalid={!!act.fields.rejectReason}
            maxLength={500}
          />
          {act.fields.rejectReason && <p className="mt-1 text-[12px] text-bad">{act.fields.rejectReason}</p>}
        </div>
        {act.error && (
          <div className="mt-4">
            <Alert>{act.error}</Alert>
          </div>
        )}
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setModal(null)}>
            Bekor qilish
          </Button>
          <Button variant={modal?.to === "rejected" ? "danger" : "primary"} loading={act.busy} onClick={() => void change()}>
            Tasdiqlash
          </Button>
        </div>
      </Modal>
    </>
  );
}
