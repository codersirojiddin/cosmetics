import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { publicGet } from "@/lib/api";
import type { Category, Page, ProductCard as Card } from "@/lib/types";
import { ProductCard } from "@/components/product-card";
import { LinkButton } from "@/components/ui";

export const dynamic = "force-dynamic";

const steps = [
  ["01", "Tanlang", "Mahsulot va variantini (tus, hajm) tanlab, buyurtma berishni bosing."],
  ["02", "To'lang", "Ko'rsatilgan kartaga summani o'tkazing va chek rasmini yuklang."],
  ["03", "Qabul qiling", "Admin chekni tekshiradi, buyurtmangiz jo'natiladi. Har bosqich haqida Telegram'da xabar olasiz."],
];

export default async function Home() {
  const [cats, featured, fresh] = await Promise.all([
    publicGet<{ items: Category[] }>("/categories").catch(() => ({ items: [] as Category[] })),
    publicGet<Page<Card>>("/products?featured=1&limit=4").catch(() => null),
    publicGet<Page<Card>>("/products?new=1&limit=4").catch(() => null),
  ]);
  const mosaic = (featured?.items ?? []).filter((p) => p.image).slice(0, 3);

  return (
    <>
      <section className="grid items-center gap-10 py-12 sm:py-16 lg:grid-cols-[1.1fr_1fr]">
        <div>
          <p className="label-xs">Kosmetika · Parvarish · Makiyaj</p>
          <h1 className="mt-4 text-[34px] font-semibold uppercase leading-[1.08] tracking-[0.02em] sm:text-[48px]">
            Toza tanlov.
            <br />
            Sodda parvarish<span className="text-accent">.</span>
          </h1>
          <p className="mt-5 max-w-md text-[14px] text-muted">
            Terisi, sochi va uslubi uchun to&apos;g&apos;ri mahsulotni toping. Buyurtma Telegram orqali, to&apos;lov chek bilan: murakkab ro&apos;yxatdan o&apos;tish yo&apos;q.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <LinkButton href="/catalog" size="lg">
              Katalogni ko&apos;rish <ArrowRight className="size-4" />
            </LinkButton>
            <LinkButton href="#qanday" variant="secondary" size="lg">
              Qanday buyurtma beriladi
            </LinkButton>
          </div>
        </div>
        {mosaic.length > 0 && (
          <div className="grid grid-cols-3 gap-3">
            {mosaic.map((p, i) => (
              <Link key={p.id} href={`/product/${p.slug}`} className={`group block overflow-hidden border border-line bg-card ${i === 1 ? "mt-8" : ""}`}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={p.image!.card} alt={p.name} className="aspect-[3/4] size-full object-cover transition-transform duration-500 group-hover:scale-[1.04]" />
              </Link>
            ))}
          </div>
        )}
      </section>

      {cats.items.length > 0 && (
        <section className="border-t border-line py-12">
          <SectionTitle title="Kategoriyalar" href="/catalog" />
          <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
            {cats.items.map((c) => (
              <Link key={c.id} href={`/catalog?category=${c.slug}`} className="group flex items-end justify-between border border-line bg-card p-4 transition-colors hover:border-ink sm:h-28">
                <span className="text-[13px] font-medium uppercase leading-snug tracking-[0.06em]">{c.name}</span>
                <span className="text-[11px] text-muted">{c.productCount}</span>
              </Link>
            ))}
          </div>
        </section>
      )}

      {featured && featured.items.length > 0 && (
        <section className="border-t border-line py-12">
          <SectionTitle title="Mashhur" href="/catalog?sort=new" />
          <Grid items={featured.items} />
        </section>
      )}

      {fresh && fresh.items.length > 0 && (
        <section className="border-t border-line py-12">
          <SectionTitle title="Yangi" href="/catalog?sort=new" />
          <Grid items={fresh.items} />
        </section>
      )}

      <section id="qanday" className="scroll-mt-28 border-t border-line py-12">
        <SectionTitle title="Qanday buyurtma beriladi" />
        <ol className="mt-6 grid gap-px border border-line bg-line sm:grid-cols-3">
          {steps.map(([n, t, d]) => (
            <li key={n} className="bg-card p-6">
              <p className="text-[28px] font-semibold text-accent">{n}</p>
              <p className="mt-3 text-[13px] font-semibold uppercase tracking-[0.1em]">{t}</p>
              <p className="mt-2 text-[13px] text-muted">{d}</p>
            </li>
          ))}
        </ol>
      </section>
    </>
  );
}

function SectionTitle({ title, href }: { title: string; href?: string }) {
  return (
    <div className="flex items-end justify-between">
      <h2 className="text-[13px] font-semibold uppercase tracking-[0.16em]">{title}</h2>
      {href && (
        <Link href={href} className="text-[12px] uppercase tracking-[0.1em] text-muted hover:text-ink">
          Hammasi →
        </Link>
      )}
    </div>
  );
}

function Grid({ items }: { items: Card[] }) {
  return (
    <div className="mt-6 grid grid-cols-2 gap-x-3 gap-y-8 sm:gap-x-5 lg:grid-cols-4">
      {items.map((p) => (
        <ProductCard key={p.id} p={p} />
      ))}
    </div>
  );
}
