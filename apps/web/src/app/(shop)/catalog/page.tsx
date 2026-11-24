import type { Metadata } from "next";
import Link from "next/link";
import { publicGet } from "@/lib/api";
import type { Category, Page, ProductCard as Card } from "@/lib/types";
import { ProductCard } from "@/components/product-card";
import { EmptyState } from "@/components/ui";
import { cx } from "@/lib/cx";

export const dynamic = "force-dynamic";
export const metadata: Metadata = { title: "Katalog" };

const SORTS = [
  ["new", "Yangilari"],
  ["price_asc", "Arzonlari"],
  ["price_desc", "Qimmatlari"],
  ["name", "Nomi (A-Z)"],
] as const;

type SP = Promise<Record<string, string | undefined>>;

export default async function CatalogPage({ searchParams }: { searchParams: SP }) {
  const sp = await searchParams;
  const page = Math.max(1, Number(sp.page) || 1);
  const sort = SORTS.some(([k]) => k === sp.sort) ? sp.sort! : "new";
  const q = (sp.q ?? "").trim();

  const qs = new URLSearchParams({ page: String(page), limit: "12", sort });
  if (sp.category) qs.set("category", sp.category);
  if (q) qs.set("q", q);

  const [cats, products] = await Promise.all([
    publicGet<{ items: Category[] }>("/categories").catch(() => ({ items: [] as Category[] })),
    publicGet<Page<Card>>(`/products?${qs}`, 15).catch(() => null),
  ]);

  const href = (over: Record<string, string | undefined>) => {
    const p = new URLSearchParams();
    const merged = { category: sp.category, q: q || undefined, sort: sort === "new" ? undefined : sort, ...over };
    for (const [k, v] of Object.entries(merged)) if (v) p.set(k, v);
    const s = p.toString();
    return s ? `/catalog?${s}` : "/catalog";
  };
  const pages = products ? Math.max(1, Math.ceil(products.total / products.limit)) : 1;
  const activeCat = cats.items.find((c) => c.slug === sp.category);

  return (
    <div className="py-10">
      <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="label-xs">Katalog</p>
          <h1 className="mt-2 text-[26px] font-semibold uppercase tracking-[0.04em]">{activeCat ? activeCat.name : "Barcha mahsulotlar"}</h1>
        </div>
        <form action="/catalog" method="get" className="flex w-full sm:w-80">
          {sp.category && <input type="hidden" name="category" value={sp.category} />}
          {sort !== "new" && <input type="hidden" name="sort" value={sort} />}
          <input
            name="q"
            defaultValue={q}
            autoFocus={sp.focus === "search"}
            placeholder="Qidirish…"
            aria-label="Qidirish"
            className="h-11 min-w-0 flex-1 border border-line bg-card px-3 text-[14px] outline-none focus:border-ink"
          />
          <button className="h-11 border border-l-0 border-ink bg-ink px-4 text-[12px] font-medium uppercase tracking-[0.1em] text-paper hover:bg-black">Top</button>
        </form>
      </div>

      <div className="mt-8 flex flex-wrap items-center justify-between gap-4 border-y border-line py-3">
        <div className="-mx-1 flex flex-wrap gap-1">
          <Chip href={href({ category: undefined, page: undefined })} active={!sp.category}>
            Hammasi
          </Chip>
          {cats.items.map((c) => (
            <Chip key={c.id} href={href({ category: c.slug, page: undefined })} active={sp.category === c.slug}>
              {c.name} <span className="text-muted">{c.productCount}</span>
            </Chip>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-x-1 text-[12px]">
          <span className="label-xs mr-1">Saralash</span>
          {SORTS.map(([k, label]) => (
            <Link key={k} href={href({ sort: k === "new" ? undefined : k, page: undefined })} className={cx("px-2 py-1 uppercase tracking-[0.08em] hover:text-accent", sort === k ? "font-semibold underline underline-offset-4" : "text-muted")}>
              {label}
            </Link>
          ))}
        </div>
      </div>

      {!products ? (
        <div className="mt-10">
          <EmptyState title="Katalogni yuklab bo'lmadi">Server bilan aloqa yo&apos;q. Birozdan keyin sahifani yangilang.</EmptyState>
        </div>
      ) : products.items.length === 0 ? (
        <div className="mt-10">
          <EmptyState title="Hech narsa topilmadi">{q ? `«${q}» bo'yicha mahsulot yo'q. Boshqa so'z bilan urinib ko'ring.` : "Bu kategoriyada hozircha mahsulot yo'q."}</EmptyState>
        </div>
      ) : (
        <>
          <p className="mt-6 text-[12px] text-muted">{products.total} ta mahsulot</p>
          <div className="mt-4 grid grid-cols-2 gap-x-3 gap-y-8 sm:gap-x-5 lg:grid-cols-4">
            {products.items.map((p) => (
              <ProductCard key={p.id} p={p} />
            ))}
          </div>
          {pages > 1 && (
            <nav className="mt-12 flex items-center justify-between text-[12px] uppercase tracking-[0.1em]" aria-label="Sahifalar">
              {page > 1 ? <Link href={href({ page: String(page - 1) })} className="border border-ink px-4 py-2.5 hover:bg-ink hover:text-paper">← Oldingi</Link> : <span />}
              <span className="text-muted">
                {page} / {pages}
              </span>
              {page < pages ? <Link href={href({ page: String(page + 1) })} className="border border-ink px-4 py-2.5 hover:bg-ink hover:text-paper">Keyingi →</Link> : <span />}
            </nav>
          )}
        </>
      )}
    </div>
  );
}

function Chip({ href, active, children }: { href: string; active: boolean; children: React.ReactNode }) {
  return (
    <Link href={href} className={cx("border px-3 py-1.5 text-[12px] uppercase tracking-[0.08em] transition-colors", active ? "border-ink bg-ink text-paper" : "border-transparent hover:border-line")}>
      {children}
    </Link>
  );
}
