"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Copy, ImagePlus, Minus, Plus } from "lucide-react";
import { api, errorInfo } from "@/lib/api";
import { money } from "@/lib/format";
import { useAction, useApi } from "@/lib/hooks";
import { downscale } from "@/lib/image";
import { useRequireAuth } from "@/lib/use-require-auth";
import type { Order, PaymentInfo, ProductDetail } from "@/lib/types";
import { Alert, Button, Field, Input, PageLoader, Skeleton, Textarea, useToast } from "@/components/ui";
import type { LatLng } from "@/components/location-picker";

const LocationPicker = dynamic(() => import("@/components/location-picker"), { ssr: false, loading: () => <Skeleton className="h-64" /> });

const clamp = (n: number, a: number, b: number) => Math.min(b, Math.max(a, n));

interface Receipt {
  id: string;
  preview: string;
}

function CheckoutInner() {
  const sp = useSearchParams();
  const slug = sp.get("p");
  const variantId = sp.get("v");
  const router = useRouter();
  const toast = useToast();
  const { user, loading } = useRequireAuth();
  const product = useApi<ProductDetail>(slug ? `/products/${encodeURIComponent(slug)}` : null);
  const pay = useApi<PaymentInfo>(user ? "/payment-info" : null);
  const submit = useAction();

  const [qty, setQty] = useState(clamp(Number(sp.get("q")) || 1, 1, 10));
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [address, setAddress] = useState("");
  const [comment, setComment] = useState("");
  const [loc, setLoc] = useState<LatLng | null>(null);
  const [receipt, setReceipt] = useState<Receipt | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [errs, setErrs] = useState<Record<string, string>>({});
  const file = useRef<HTMLInputElement>(null);
  const prefilled = useRef(false);
  // a field's error disappears as soon as the user edits it
  const clear = (k: string) => setErrs((e) => (e[k] ? { ...e, [k]: "" } : e));

  useEffect(() => {
    if (user && !prefilled.current) {
      prefilled.current = true;
      setName(`${user.firstName} ${user.lastName}`.trim());
      if (user.phone) setPhone(user.phone);
    }
  }, [user]);

  const p = product.data;
  const variant = p?.variants.find((v) => v.id === variantId);
  const total = variant ? variant.price * qty : 0;
  const card = pay.data;
  const noCard = !!card && !card.cardNumber;

  async function onFile(f: File | undefined) {
    if (!f) return;
    setUploadError(null);
    setUploading(true);
    try {
      const blob = await downscale(f);
      const form = new FormData();
      form.append("file", blob, "receipt.jpg");
      const r = await api<{ id: string }>("/uploads/receipt", { form });
      setReceipt((old) => {
        if (old) URL.revokeObjectURL(old.preview);
        return { id: r.id, preview: URL.createObjectURL(blob) };
      });
      setErrs((e) => ({ ...e, receiptFileId: "" }));
    } catch (e) {
      const info = errorInfo(e);
      setUploadError(info.fields.file ?? info.message);
    } finally {
      setUploading(false);
      if (file.current) file.current.value = "";
    }
  }

  function validate(): Record<string, string> {
    const e: Record<string, string> = {};
    if (name.trim().length < 2) e.customerName = "Ismingizni kiriting";
    const digits = phone.replace(/\D/g, "");
    if (!(digits.length === 9 || (digits.length === 12 && digits.startsWith("998")) || (digits.length === 10 && digits.startsWith("0")))) e.phone = "Telefon raqamini to'liq kiriting: +998 90 123 45 67";
    if (address.trim().length < 5) e.addressText = "Manzilni to'liqroq yozing (shahar, ko'cha, uy)";
    if (!loc) e.location = "Xaritadan yetkazib berish nuqtasini belgilang";
    if (!receipt) e.receiptFileId = "To'lov chekining rasmini yuklang";
    return e;
  }

  async function onSubmit() {
    const e = validate();
    setErrs(e);
    if (Object.keys(e).length) {
      const first = ["receiptFileId", "customerName", "phone", "addressText", "location"].find((k) => e[k]);
      document.getElementById(`f-${first}`)?.scrollIntoView({ behavior: "smooth", block: "center" });
      return;
    }
    const order = await submit.run(() =>
      api<Order>("/orders", {
        body: { items: [{ variantId, quantity: qty }], customerName: name.trim(), phone, addressText: address.trim(), lat: loc!.lat, lng: loc!.lng, receiptFileId: receipt!.id, comment: comment.trim() },
      }),
    );
    if (order) {
      toast("Buyurtma qabul qilindi");
      router.replace(`/orders/${order.id}?new=1`);
    }
  }

  if (!slug || !variantId) {
    return (
      <div className="py-20 text-center">
        <p className="text-[13px] uppercase tracking-[0.1em]">Mahsulot tanlanmagan</p>
        <Link href="/catalog" className="mt-4 inline-block underline underline-offset-4">
          Katalogga o&apos;tish
        </Link>
      </div>
    );
  }
  if (loading || !user || product.loading) return <PageLoader />;
  if (!p || !variant) {
    return (
      <div className="py-20 text-center">
        <p className="text-[13px] uppercase tracking-[0.1em]">Mahsulot yoki variant topilmadi</p>
        <Link href="/catalog" className="mt-4 inline-block underline underline-offset-4">
          Katalogga o&apos;tish
        </Link>
      </div>
    );
  }

  const fe = (k: string) => errs[k] || submit.fields[k] || undefined;
  const img = variant.image ?? p.images[0];

  return (
    <div className="py-8 sm:py-10">
      <Link href={`/product/${p.slug}`} className="text-[12px] uppercase tracking-[0.1em] text-muted hover:text-ink">
        ← Mahsulotga qaytish
      </Link>
      <h1 className="mt-3 text-[26px] font-semibold uppercase tracking-[0.04em]">Buyurtmani rasmiylashtirish</h1>

      <div className="mt-8 grid gap-8 lg:grid-cols-[1fr_380px] lg:gap-12">
        <div className="space-y-8">
          {/* 1. payment */}
          <Section n="1" title="To'lov va chek">
            {noCard ? (
              <Alert kind="warn">Do&apos;kon to&apos;lov ma&apos;lumotlarini hali kiritmagan. Iltimos, birozdan keyin qayta urinib ko&apos;ring yoki do&apos;kon bilan bog&apos;laning.</Alert>
            ) : card ? (
              <div className="border border-line bg-paper p-4">
                <p className="text-[13px]">
                  Quyidagi kartaga <b className="text-[15px]">{money(total)}</b> o&apos;tkazing:
                </p>
                <div className="mt-3 flex flex-wrap items-center justify-between gap-3 border border-ink bg-card px-4 py-3">
                  <div>
                    <p className="text-[18px] font-semibold tracking-[0.12em]">{card.cardNumber}</p>
                    {card.cardHolder && <p className="mt-0.5 text-[12px] uppercase tracking-[0.1em] text-muted">{card.cardHolder}</p>}
                  </div>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => navigator.clipboard?.writeText(card.cardNumber.replace(/\s/g, "")).then(() => toast("Karta raqami nusxalandi"))}
                  >
                    <Copy className="size-3.5" /> Nusxalash
                  </Button>
                </div>
                {card.paymentNote && <p className="mt-3 text-[12px] text-muted">{card.paymentNote}</p>}
              </div>
            ) : (
              <Skeleton className="h-28" />
            )}

            <div id="f-receiptFileId" className="mt-5 scroll-mt-32">
              <span className="label-xs mb-1.5 block">
                Chek rasmi <span className="text-accent">*</span>
              </span>
              <input ref={file} type="file" accept="image/*" className="sr-only" id="receipt-input" onChange={(e) => void onFile(e.target.files?.[0])} />
              {receipt ? (
                <div className="flex items-center gap-4 border border-line bg-card p-3">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={receipt.preview} alt="Chek" className="size-20 border border-line object-cover" />
                  <div className="min-w-0 flex-1">
                    <p className="text-[13px] font-medium text-ok">Chek yuklandi</p>
                    <label htmlFor="receipt-input" className="mt-1 inline-block cursor-pointer text-[12px] uppercase tracking-[0.1em] underline underline-offset-4 hover:text-accent">
                      Boshqa rasm tanlash
                    </label>
                  </div>
                </div>
              ) : (
                <label
                  htmlFor="receipt-input"
                  className="flex cursor-pointer flex-col items-center justify-center gap-2 border border-dashed border-ink bg-card px-4 py-9 text-center transition-colors hover:bg-ink/[0.03]"
                >
                  <ImagePlus className="size-6" />
                  <span className="text-[12px] font-medium uppercase tracking-[0.12em]">{uploading ? "Yuklanmoqda…" : "Chek rasmini tanlash"}</span>
                  <span className="text-[12px] text-muted">To&apos;lov ilovasidagi skrinshot yoki chek surati (JPG, PNG)</span>
                </label>
              )}
              {(uploadError || fe("receiptFileId")) && <p className="mt-1.5 text-[12px] text-bad">{uploadError ?? fe("receiptFileId")}</p>}
            </div>
          </Section>

          {/* 2. contact */}
          <Section n="2" title="Qabul qiluvchi">
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Ism" required error={fe("customerName")}>
                <Input id="f-customerName" value={name} onChange={(e) => { setName(e.target.value); clear("customerName"); }} autoComplete="name" invalid={!!fe("customerName")} maxLength={100} />
              </Field>
              <Field label="Telefon" required error={fe("phone")}>
                <Input id="f-phone" value={phone} onChange={(e) => { setPhone(e.target.value); clear("phone"); }} inputMode="tel" autoComplete="tel" placeholder="+998 90 123 45 67" invalid={!!fe("phone")} maxLength={30} />
              </Field>
            </div>
          </Section>

          {/* 3. address */}
          <Section n="3" title="Yetkazib berish manzili">
            <div className="space-y-4">
              <Field label="Manzil" required error={fe("addressText")} hint="Shahar, tuman, ko'cha, uy va kvartira">
                <Textarea id="f-addressText" rows={2} value={address} onChange={(e) => { setAddress(e.target.value); clear("addressText"); }} autoComplete="street-address" invalid={!!fe("addressText")} maxLength={300} />
              </Field>
              <div id="f-location" className="scroll-mt-32">
                <span className="label-xs mb-1.5 block">
                  Xaritadagi nuqta <span className="text-accent">*</span>
                </span>
                <LocationPicker value={loc} onChange={(v) => { setLoc(v); setErrs((e) => ({ ...e, location: "" })); }} />
                {(fe("location") || fe("lat") || fe("lng")) && <p className="mt-1.5 text-[12px] text-bad">{fe("location") ?? "Joylashuvni belgilang"}</p>}
              </div>
              <Field label="Izoh (ixtiyoriy)">
                <Textarea rows={2} value={comment} onChange={(e) => setComment(e.target.value)} placeholder="Masalan: qo'ng'iroq qilib keling" maxLength={500} />
              </Field>
            </div>
          </Section>
        </div>

        {/* summary */}
        <aside className="lg:sticky lg:top-32 lg:self-start">
          <div className="border border-ink bg-card p-5">
            <p className="label-xs">Buyurtma</p>
            <div className="mt-4 flex gap-4">
              {img && (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={img.thumb} alt="" className="size-20 border border-line object-cover" />
              )}
              <div className="min-w-0">
                <p className="text-[13px] font-medium uppercase leading-snug tracking-[0.04em]">{p.name}</p>
                {variant.label && <p className="mt-1 text-[12px] text-muted">{p.optionLabel ? `${p.optionLabel}: ` : ""}{variant.label}</p>}
                <p className="mt-1 text-[13px]">{money(variant.price)}</p>
              </div>
            </div>
            <div className="mt-4 flex items-center justify-between border-t border-line pt-4">
              <span className="label-xs">Soni</span>
              <div className="flex items-center border border-line">
                <button onClick={() => setQty((q) => Math.max(1, q - 1))} disabled={qty <= 1} aria-label="Kamaytirish" className="grid size-9 place-items-center hover:bg-ink/5 disabled:opacity-30">
                  <Minus className="size-3.5" />
                </button>
                <span className="w-9 text-center text-[13px] font-medium">{qty}</span>
                <button onClick={() => setQty((q) => Math.min(10, q + 1))} disabled={qty >= 10} aria-label="Ko'paytirish" className="grid size-9 place-items-center hover:bg-ink/5 disabled:opacity-30">
                  <Plus className="size-3.5" />
                </button>
              </div>
            </div>
            <div className="mt-4 flex items-baseline justify-between border-t border-line pt-4">
              <span className="label-xs">O&apos;tkazish summasi</span>
              <span className="text-[20px] font-semibold">{money(total)}</span>
            </div>

            {submit.error && (
              <div className="mt-4">
                <Alert>{submit.error}</Alert>
              </div>
            )}
            {Object.keys(errs).some((k) => errs[k]) && (
              <div className="mt-4">
                <Alert>Iltimos, qizil bilan belgilangan maydonlarni to&apos;ldiring.</Alert>
              </div>
            )}
            <Button size="lg" className="mt-5 w-full" loading={submit.busy} disabled={uploading || noCard} onClick={() => void onSubmit()}>
              Buyurtmani yuborish
            </Button>
            <p className="mt-3 text-[11px] leading-relaxed text-muted">Yuborgandan keyin admin chekni tekshiradi va siz bilan Telegram yoki telefon orqali bog&apos;lanadi.</p>
          </div>
        </aside>
      </div>
    </div>
  );
}

function Section({ n, title, children }: { n: string; title: string; children: React.ReactNode }) {
  return (
    <section>
      <h2 className="mb-4 flex items-center gap-3 text-[13px] font-semibold uppercase tracking-[0.14em]">
        <span className="grid size-6 place-items-center bg-ink text-[11px] text-paper">{n}</span>
        {title}
      </h2>
      {children}
    </section>
  );
}

export default function CheckoutPage() {
  return (
    <Suspense fallback={<PageLoader />}>
      <CheckoutInner />
    </Suspense>
  );
}
