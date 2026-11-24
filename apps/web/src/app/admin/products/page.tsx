"use client";

import { Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Plus } from "lucide-react";
import { PRODUCT_STATUS, num } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import type { AdminProductRow, Category, Page } from "@/lib/types";
import { Alert, EmptyState, Input, LinkButton, PageLoader, Pagination, Select } from "@/components/ui";
import { PageHeader } from "@/components/admin/bits";
import { cx } from "@/lib/cx";

function Inner() {
  const router = useRouter();
  const sp = useSearchParams();
  const status = sp.get("status") ?? "";
  const category = sp.get("categoryId") ?? "";
  const q = sp.get("q") ?? "";
  const page = Math.max(1, Number(sp.get("page")) || 1);
  const cats = useApi<{ items: Category[] }>("/admin/categories");

  const go = (over: Record<string, string>) => {
    const p = new URLSearchParams({ status, categoryId: category, q, page: String(page), ...over });
    for (const [k, v] of [...p]) if (!v || (k === "page" && v === "1")) p.delete(k);
    router.replace(`/admin/products${p.size ? `?${p}` : ""}`);
  };

  const qs = new URLSearchParams({ page: String(page), limit: "20" });
  if (status) qs.set("status", status);
  if (category) qs.set("categoryId", category);
  if (q) qs.set("q", q);
  const { data, error, loading } = useApi<Page<AdminProductRow>>(`/admin/products?${qs}`);

  return (
    <>
      <PageHeader
        title="Mahsulotlar"
        sub="Katalog"
        actions={
          <LinkButton href="/admin/products/new">
            <Plus className="size-4" /> Yangi mahsulot
          </LinkButton>
        }
      />
      <div className="mb-6 grid gap-3 sm:grid-cols-[1fr_200px_220px]">
        <Input key={q} defaultValue={q} placeholder="Nomi yoki brend…" aria-label="Qidirish" onKeyDown={(e) => e.key === "Enter" && go({ q: e.currentTarget.value.trim(), page: "1" })} />
        <Select value={status} onChange={(e) => go({ status: e.target.value, page: "1" })} aria-label="Holat">
          <option value="">Barcha holatlar</option>
          <option value="active">Faol</option>
          <option value="draft">Qoralama</option>
          <option value="archived">Arxiv</option>
        </Select>
        <Select value={category} onChange={(e) => go({ categoryId: e.target.value, page: "1" })} aria-label="Kategoriya">
          <option value="">Barcha kategoriyalar</option>
          {(cats.data?.items ?? []).map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </Select>
      </div>

      {error && <Alert>{error}</Alert>}
      {loading && !data ? (
        <PageLoader />
      ) : data && data.items.length > 0 ? (
        <>
          <div className="border border-line bg-card">
            <ul className="divide-y divide-line">
              {data.items.map((p) => (
                <li key={p.id}>
                  <Link href={`/admin/products/${p.id}`} className="flex items-center gap-4 px-4 py-3 hover:bg-ink/[0.03]">
                    {p.image ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={p.image.thumb} alt="" className="size-14 shrink-0 border border-line object-cover" />
                    ) : (
                      <div className="size-14 shrink-0 border border-dashed border-line" />
                    )}
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-[13px] font-medium uppercase tracking-[0.04em]">{p.name}</p>
                      <p className="truncate text-[12px] text-muted">
                        {[p.brand, p.category.name, p.variantCount > 1 ? `${p.variantCount} variant` : ""].filter(Boolean).join(" · ")}
                      </p>
                    </div>
                    <div className="hidden w-28 text-right sm:block">
                      <p className="text-[13px] font-medium">{num(p.priceFrom)}</p>
                      <p className={cx("text-[12px]", p.stockTotal === 0 ? "text-bad" : "text-muted")}>{p.stockTotal === 0 ? "Qoldiq yo'q" : `${p.stockTotal} dona`}</p>
                    </div>
                    <span className={cx("w-24 text-right text-[11px] font-medium uppercase tracking-[0.1em]", PRODUCT_STATUS[p.status].tone)}>{PRODUCT_STATUS[p.status].label}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </div>
          <Pagination page={data.page} limit={data.limit} total={data.total} onPage={(n) => go({ page: String(n) })} />
        </>
      ) : (
        !error && (
          <EmptyState title="Mahsulot topilmadi">
            {q || status || category ? "Filtrlarni o'zgartirib ko'ring." : "Birinchi mahsulotingizni qo'shing."}
          </EmptyState>
        )
      )}
    </>
  );
}

export default function AdminProductsPage() {
  return (
    <Suspense fallback={<PageLoader />}>
      <Inner />
    </Suspense>
  );
}
