"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Search } from "lucide-react";
import { STATUS } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import { cx } from "@/lib/cx";
import type { OrderStatus, OrderSummary, Page } from "@/lib/types";
import { Alert, Button, EmptyState, Input, PageLoader, Pagination, Select } from "@/components/ui";
import { OrdersList, PageHeader } from "@/components/admin/bits";
import { useAdmin } from "@/components/admin/shell";

const TABS: (OrderStatus | "")[] = ["", "waiting", "confirmed", "shipping", "delivered", "rejected"];

function Inner() {
  const router = useRouter();
  const sp = useSearchParams();
  const { stats } = useAdmin();
  const status = sp.get("status") ?? "";
  const assigned = sp.get("assigned") ?? "";
  const q = sp.get("q") ?? "";
  const page = Math.max(1, Number(sp.get("page")) || 1);
  const [search, setSearch] = useState(q);

  const go = (over: Record<string, string>) => {
    const p = new URLSearchParams({ status, assigned, q, page: String(page), ...over });
    for (const [k, v] of [...p]) if (!v || (k === "page" && v === "1")) p.delete(k);
    router.replace(`/admin/orders${p.size ? `?${p}` : ""}`);
  };

  const qs = new URLSearchParams({ page: String(page), limit: "20" });
  if (status) qs.set("status", status);
  if (assigned) qs.set("assigned", assigned);
  if (q) qs.set("q", q);
  const { data, error, loading } = useApi<Page<OrderSummary>>(`/admin/orders?${qs}`);

  const count = (s: OrderStatus | "") => (!stats ? null : s === "" ? Object.values(stats.counts).reduce((a, b) => a + b, 0) : stats.counts[s]);

  return (
    <>
      <PageHeader title="Buyurtmalar" sub="Admin" />

      <div className="-mx-4 mb-4 flex gap-1 overflow-x-auto border-b border-line px-4 lg:mx-0 lg:px-0">
        {TABS.map((t) => (
          <button key={t || "all"} onClick={() => go({ status: t, page: "1" })} className={cx("shrink-0 border-b-2 px-3 py-2.5 text-[12px] font-medium uppercase tracking-[0.08em]", status === t ? "border-ink text-ink" : "border-transparent text-muted hover:text-ink")}>
            {t ? STATUS[t].label : "Hammasi"} <span className="text-muted">{count(t)}</span>
          </button>
        ))}
      </div>

      <div className="mb-6 flex flex-col gap-3 sm:flex-row">
        <form
          className="flex flex-1"
          onSubmit={(e) => {
            e.preventDefault();
            go({ q: search.trim(), page: "1" });
          }}
        >
          <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Raqam, telefon yoki ism…" aria-label="Qidirish" className="border-r-0" />
          <Button type="submit" aria-label="Qidirish" className="px-4">
            <Search className="size-4" />
          </Button>
        </form>
        <Select value={assigned} onChange={(e) => go({ assigned: e.target.value, page: "1" })} aria-label="Admin bo'yicha" className="sm:w-56">
          <option value="">Barcha adminlar</option>
          <option value="me">Mening buyurtmalarim</option>
          <option value="none">Hech kim olmagan</option>
        </Select>
      </div>

      {error && <Alert>{error}</Alert>}
      {loading && !data ? (
        <PageLoader />
      ) : data && data.items.length > 0 ? (
        <>
          <OrdersList items={data.items} />
          <Pagination page={data.page} limit={data.limit} total={data.total} onPage={(p) => go({ page: String(p) })} />
        </>
      ) : (
        !error && <EmptyState title="Buyurtma topilmadi">Filtrlarni o&apos;zgartirib ko&apos;ring.</EmptyState>
      )}
    </>
  );
}

export default function AdminOrdersPage() {
  return (
    <Suspense fallback={<PageLoader />}>
      <Inner />
    </Suspense>
  );
}
