import Link from "next/link";
import { num } from "@/lib/format";
import type { ProductCard as Card } from "@/lib/types";
import { Tag } from "./ui";

export function ProductCard({ p }: { p: Card }) {
  const discount = p.oldPriceFrom && p.oldPriceFrom > p.priceFrom;
  return (
    <Link href={`/product/${p.slug}`} className="group block">
      <div className="relative aspect-[4/5] overflow-hidden border border-line bg-card">
        {p.image ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={p.image.card} alt={p.name} loading="lazy" className="size-full object-cover transition-transform duration-500 group-hover:scale-[1.03]" />
        ) : (
          <div className="flex size-full items-center justify-center text-[11px] uppercase tracking-[0.14em] text-muted">Rasm yo&apos;q</div>
        )}
        <div className="absolute left-2 top-2 flex flex-col items-start gap-1">
          {!p.inStock && <Tag tone="muted">Tugagan</Tag>}
          {p.inStock && p.isNew && <Tag>Yangi</Tag>}
          {p.inStock && discount && <Tag tone="accent">Chegirma</Tag>}
        </div>
      </div>
      <div className="mt-3">
        {p.brand && <p className="label-xs">{p.brand}</p>}
        <h3 className="mt-0.5 text-[13px] font-medium uppercase leading-snug tracking-[0.04em] group-hover:text-accent">{p.name}</h3>
        <p className="mt-1.5 text-[13px]">
          {p.hasVariants && <span className="text-muted">dan </span>}
          <span className="font-semibold">{num(p.priceFrom)} so&apos;m</span>
          {discount && <span className="ml-2 text-muted line-through">{num(p.oldPriceFrom!)}</span>}
        </p>
      </div>
    </Link>
  );
}
