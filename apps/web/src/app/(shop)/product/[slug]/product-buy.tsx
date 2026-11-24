"use client";

import { useMemo, useState, type ReactNode } from "react";
import { Minus, Plus } from "lucide-react";
import { money, num } from "@/lib/format";
import type { ProductDetail } from "@/lib/types";
import { LinkButton, Tag, cx } from "@/components/ui";

const MAX_QTY = 10;

export function ProductBuy({ product: p, children }: { product: ProductDetail; children: ReactNode }) {
  const firstAvailable = p.variants.find((v) => v.inStock) ?? p.variants[0];
  const [variantId, setVariantId] = useState(firstAvailable.id);
  const [qty, setQty] = useState(1);
  const [imgIdx, setImgIdx] = useState(0);

  const variant = p.variants.find((v) => v.id === variantId) ?? firstAvailable;
  const hasChoice = p.variants.length > 1;
  const discount = variant.oldPrice && variant.oldPrice > variant.price;

  const pick = (id: string) => {
    setVariantId(id);
    const v = p.variants.find((x) => x.id === id);
    const i = v?.image ? p.images.findIndex((im) => im.full === v.image!.full) : -1;
    if (i >= 0) setImgIdx(i);
  };

  const total = useMemo(() => variant.price * qty, [variant.price, qty]);
  const img = p.images[imgIdx] ?? p.images[0];
  const checkout = `/checkout?p=${encodeURIComponent(p.slug)}&v=${variant.id}&q=${qty}`;

  return (
    <div className="grid gap-8 lg:grid-cols-2 lg:gap-14">
      <div className="lg:sticky lg:top-32 lg:self-start">
        <div className="aspect-[4/5] overflow-hidden border border-line bg-card">
          {img ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={img.full} alt={p.name} className="size-full object-cover" />
          ) : (
            <div className="flex size-full items-center justify-center text-[12px] uppercase tracking-[0.14em] text-muted">Rasm yo&apos;q</div>
          )}
        </div>
        {p.images.length > 1 && (
          <div className="mt-3 grid grid-cols-5 gap-2">
            {p.images.map((im, i) => (
              <button key={im.full} onClick={() => setImgIdx(i)} aria-label={`Rasm ${i + 1}`} aria-current={i === imgIdx} className={cx("aspect-square overflow-hidden border bg-card", i === imgIdx ? "border-ink" : "border-line opacity-70 hover:opacity-100")}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={im.thumb} alt="" className="size-full object-cover" />
              </button>
            ))}
          </div>
        )}
      </div>

      <div>
        <div className="flex items-center gap-2">
          {p.brand && <p className="label-xs">{p.brand}</p>}
          {p.isNew && <Tag>Yangi</Tag>}
        </div>
        <h1 className="mt-2 text-[24px] font-semibold uppercase leading-tight tracking-[0.03em] sm:text-[30px]">{p.name}</h1>

        <p className="mt-5 flex items-baseline gap-3">
          <span className="text-[26px] font-semibold">{money(variant.price)}</span>
          {discount && <span className="text-[15px] text-muted line-through">{num(variant.oldPrice!)}</span>}
        </p>

        {hasChoice && (
          <fieldset className="mt-7">
            <legend className="label-xs mb-2">
              {p.optionLabel || "Variant"}: <span className="text-ink">{variant.label}</span>
            </legend>
            <div className="flex flex-wrap gap-2">
              {p.variants.map((v) => (
                <button
                  key={v.id}
                  onClick={() => pick(v.id)}
                  aria-pressed={v.id === variantId}
                  className={cx(
                    "min-w-16 border px-4 py-2.5 text-[12px] font-medium uppercase tracking-[0.08em] transition-colors",
                    v.id === variantId ? "border-ink bg-ink text-paper" : "border-line bg-card hover:border-ink",
                    !v.inStock && "text-muted line-through opacity-60",
                  )}
                >
                  {v.label}
                </button>
              ))}
            </div>
          </fieldset>
        )}

        <div className="mt-7 flex items-center gap-4">
          <div className="flex items-center border border-line bg-card" role="group" aria-label="Soni">
            <button onClick={() => setQty((q) => Math.max(1, q - 1))} disabled={qty <= 1} aria-label="Kamaytirish" className="grid size-11 place-items-center hover:bg-ink/5 disabled:opacity-30">
              <Minus className="size-4" />
            </button>
            <span className="w-10 text-center text-[14px] font-medium" aria-live="polite">
              {qty}
            </span>
            <button onClick={() => setQty((q) => Math.min(MAX_QTY, q + 1))} disabled={qty >= MAX_QTY} aria-label="Ko'paytirish" className="grid size-11 place-items-center hover:bg-ink/5 disabled:opacity-30">
              <Plus className="size-4" />
            </button>
          </div>
          <p className="text-[12px] text-muted">
            Jami: <span className="text-[15px] font-semibold text-ink">{money(total)}</span>
          </p>
        </div>

        <div className="mt-6">
          {variant.inStock ? (
            <LinkButton href={checkout} size="lg" className="w-full">
              Buyurtma berish
            </LinkButton>
          ) : (
            <button disabled className="inline-flex h-12 w-full items-center justify-center border border-line bg-paper text-[12px] font-medium uppercase tracking-[0.1em] text-muted">
              Hozircha tugagan
            </button>
          )}
          <p className="mt-3 text-[12px] leading-relaxed text-muted">To&apos;lov kartaga o&apos;tkazma orqali. Keyingi sahifada karta ma&apos;lumotlari ko&apos;rsatiladi va chek rasmini yuklaysiz.</p>
        </div>

        {children}
      </div>
    </div>
  );
}
