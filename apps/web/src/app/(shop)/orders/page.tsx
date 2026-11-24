"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { dateTime, money } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import { useRequireAuth } from "@/lib/use-require-auth";
import type { Order, Page } from "@/lib/types";
import { Alert, EmptyState, LinkButton, PageLoader, Pagination, StatusBadge } from "@/components/ui";

function Inner() {
  const { user, loading } = useRequireAuth();
  const [page, setPage] = useState(1);
  const { data, error, loading: busy } = useApi<Page<Order>>(user ? `/orders?page=${page}&limit=10` : null);

  if (loading || !user || (busy && !data)) return <PageLoader />;

  return (
    <div className="py-10">
      <p className="label-xs">Profil</p>
      <h1 className="mt-2 text-[26px] font-semibold uppercase tracking-[0.04em]">Buyurtmalarim</h1>
      <div className="mt-8">
        {error && <Alert>{error}</Alert>}
        {data && data.items.length === 0 && (
          <EmptyState title="Hali buyurtma yo'q">
            Birinchi buyurtmangizni bering.
            <div className="mt-5">
              <LinkButton href="/catalog">Katalogga o&apos;tish</LinkButton>
            </div>
          </EmptyState>
        )}
        {data && data.items.length > 0 && (
          <ul className="divide-y divide-line border border-line bg-card">
            {data.items.map((o) => (
              <li key={o.id}>
                <Link href={`/orders/${o.id}`} className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 p-4 hover:bg-ink/[0.03] sm:p-5">
                  <div className="min-w-0">
                    <p className="text-[14px] font-semibold">#{o.number}</p>
                    <p className="mt-1 truncate text-[12px] text-muted">
                      {o.items.map((i) => (i.variantLabel ? `${i.productName} (${i.variantLabel})` : i.productName) + (i.quantity > 1 ? ` ×${i.quantity}` : "")).join(", ")}
                    </p>
                    <p className="mt-1 text-[11px] text-muted">{dateTime(o.createdAt)}</p>
                  </div>
                  <div className="flex items-center gap-6">
                    <StatusBadge status={o.status} />
                    <span className="min-w-28 text-right text-[14px] font-semibold">{money(o.totalAmount)}</span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}
        {data && <Pagination page={data.page} limit={data.limit} total={data.total} onPage={setPage} />}
      </div>
    </div>
  );
}

export default function OrdersPage() {
  return (
    <Suspense fallback={<PageLoader />}>
      <Inner />
    </Suspense>
  );
}
