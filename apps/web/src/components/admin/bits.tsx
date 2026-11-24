"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { dateTime, money } from "@/lib/format";
import type { OrderSummary } from "@/lib/types";
import { StatusBadge } from "../ui";

export function PageHeader({ title, sub, actions }: { title: string; sub?: string; actions?: ReactNode }) {
  return (
    <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div>
        {sub && <p className="label-xs">{sub}</p>}
        <h1 className="mt-1 text-[24px] font-semibold uppercase tracking-[0.04em]">{title}</h1>
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

export function Panel({ title, children, actions }: { title?: string; children: ReactNode; actions?: ReactNode }) {
  return (
    <section className="border border-line bg-card">
      {(title || actions) && (
        <div className="flex items-center justify-between border-b border-line px-4 py-3">
          <h2 className="label-xs">{title}</h2>
          {actions}
        </div>
      )}
      <div className="p-4">{children}</div>
    </section>
  );
}

export function KV({ k, children }: { k: string; children: ReactNode }) {
  return (
    <div>
      <dt className="label-xs">{k}</dt>
      <dd className="mt-0.5 break-words text-[13px]">{children}</dd>
    </div>
  );
}

const GRID = "md:grid-cols-[84px_minmax(0,1.6fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_120px]";

export function OrdersList({ items }: { items: OrderSummary[] }) {
  return (
    <div className="border border-line bg-card">
      <div className={`hidden border-b border-line px-4 py-2.5 md:grid ${GRID} gap-4`}>
        {["Raqam", "Mijoz", "Summa", "Holat", "Admin", "Sana"].map((h) => (
          <span key={h} className="label-xs">
            {h}
          </span>
        ))}
      </div>
      <ul className="divide-y divide-line">
        {items.map((o) => (
          <li key={o.id}>
            <Link href={`/admin/orders/${o.id}`} className={`grid gap-x-4 gap-y-1 px-4 py-3.5 hover:bg-ink/[0.03] md:items-center ${GRID}`}>
              <span className="flex items-center gap-1.5 text-[13px] font-semibold">
                #{o.number}
                {o.duplicateReceipt && <AlertTriangle className="size-3.5 text-warn" aria-label="Chek takrorlangan bo'lishi mumkin" />}
              </span>
              <span className="min-w-0">
                <span className="block truncate text-[13px]">{o.customerName}</span>
                <span className="block text-[12px] text-muted">{o.phone}</span>
              </span>
              <span className="text-[13px] font-medium">{money(o.totalAmount)}</span>
              <span>
                <StatusBadge status={o.status} />
              </span>
              <span className="truncate text-[12px] text-muted">{o.assignedAdmin ? o.assignedAdmin.name : <span className="text-accent">Hech kim olmagan</span>}</span>
              <span className="text-[12px] text-muted">{dateTime(o.createdAt)}</span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
