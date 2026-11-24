"use client";

import Link from "next/link";
import { STATUS } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import type { OrderStatus, OrderSummary, Page } from "@/lib/types";
import { Alert, EmptyState, PageLoader } from "@/components/ui";
import { OrdersList, PageHeader } from "@/components/admin/bits";
import { useAdmin } from "@/components/admin/shell";

const ORDER: OrderStatus[] = ["waiting", "confirmed", "shipping", "delivered", "rejected"];

export default function Dashboard() {
  const { stats } = useAdmin();
  const recent = useApi<Page<OrderSummary>>("/admin/orders?status=waiting&limit=8");

  return (
    <>
      <PageHeader title="Boshqaruv paneli" sub="Umumiy ko'rinish" />

      {stats && stats.unassigned > 0 && (
        <div className="mb-6">
          <Alert kind="warn">
            <b>{stats.unassigned} ta</b> faol buyurtmani hali hech kim olmagan.{" "}
            <Link href="/admin/orders?assigned=none" className="underline underline-offset-4">
              Ko&apos;rish
            </Link>
          </Alert>
        </div>
      )}

      <div className="grid grid-cols-2 gap-px border border-line bg-line sm:grid-cols-3 lg:grid-cols-5">
        {ORDER.map((s) => (
          <Link key={s} href={`/admin/orders?status=${s}`} className="bg-card p-5 transition-colors hover:bg-paper">
            <p className="label-xs">{STATUS[s].label}</p>
            <p className={`mt-2 text-[32px] font-semibold leading-none ${STATUS[s].tone}`}>{stats ? stats.counts[s] : "–"}</p>
          </Link>
        ))}
      </div>

      <div className="mt-10">
        <div className="mb-4 flex items-end justify-between">
          <h2 className="text-[13px] font-semibold uppercase tracking-[0.14em]">Kutilayotgan buyurtmalar</h2>
          <Link href="/admin/orders" className="text-[12px] uppercase tracking-[0.1em] text-muted hover:text-ink">
            Hammasi →
          </Link>
        </div>
        {recent.loading && !recent.data ? (
          <PageLoader />
        ) : recent.error ? (
          <Alert>{recent.error}</Alert>
        ) : recent.data && recent.data.items.length > 0 ? (
          <OrdersList items={recent.data.items} />
        ) : (
          <EmptyState title="Yangi buyurtmalar yo'q">Yangi buyurtma kelganda Telegram&apos;da va shu yerda xabar olasiz.</EmptyState>
        )}
      </div>
    </>
  );
}
