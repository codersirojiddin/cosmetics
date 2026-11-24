import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ApiError, publicGet } from "@/lib/api";
import type { ProductDetail } from "@/lib/types";
import { Markdown } from "@/components/markdown";
import { ProductBuy } from "./product-buy";

export const dynamic = "force-dynamic";

async function load(slug: string): Promise<ProductDetail> {
  try {
    return await publicGet<ProductDetail>(`/products/${encodeURIComponent(slug)}`, 15);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound();
    throw e;
  }
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const p = await load(slug).catch(() => null);
  if (!p) return { title: "Mahsulot" };
  return {
    title: p.name,
    description: p.description.replace(/[*_#>`\[\]]/g, "").slice(0, 160),
    openGraph: p.images[0] ? { images: [p.images[0].card] } : undefined,
  };
}

export default async function ProductPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const p = await load(slug);
  const sections = [
    ["Tavsif", p.description, true],
    ["Tarkibi", p.ingredients, false],
    ["Ishlatish usuli", p.howToUse, false],
  ] as const;

  return (
    <div className="py-8 sm:py-10">
      <nav className="mb-6 text-[12px] uppercase tracking-[0.1em] text-muted" aria-label="Yo'l">
        <Link href="/catalog" className="hover:text-ink">
          Katalog
        </Link>
        <span className="mx-2">/</span>
        <Link href={`/catalog?category=${p.category.slug}`} className="hover:text-ink">
          {p.category.name}
        </Link>
      </nav>

      <ProductBuy product={p}>
        <div className="mt-10 border-t border-line">
          {sections
            .filter(([, text]) => text.trim())
            .map(([title, text, open]) => (
              <details key={title} open={open} className="group border-b border-line py-4">
                <summary className="flex list-none items-center justify-between text-[12px] font-semibold uppercase tracking-[0.14em] [&::-webkit-details-marker]:hidden">
                  {title}
                  <span className="text-lg leading-none text-muted group-open:hidden">+</span>
                  <span className="hidden text-lg leading-none text-muted group-open:inline">−</span>
                </summary>
                <div className="pt-4">
                  <Markdown>{text}</Markdown>
                </div>
              </details>
            ))}
        </div>
      </ProductBuy>
    </div>
  );
}
