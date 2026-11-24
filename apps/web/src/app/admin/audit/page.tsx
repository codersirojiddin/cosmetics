"use client";

import { useState } from "react";
import { useAuth } from "@/lib/auth";
import { dateTime } from "@/lib/format";
import { useApi } from "@/lib/hooks";
import type { AuditRow, Page } from "@/lib/types";
import { Alert, EmptyState, PageLoader, Pagination } from "@/components/ui";
import { PageHeader } from "@/components/admin/bits";

const LABELS: Record<string, string> = {
  "order.status": "Buyurtma holati o'zgardi",
  "order.claimed": "Buyurtma biriktirildi",
  "product.created": "Mahsulot yaratildi",
  "product.updated": "Mahsulot o'zgartirildi",
  "product.archived": "Mahsulot arxivlandi",
  "category.created": "Kategoriya yaratildi",
  "category.updated": "Kategoriya o'zgartirildi",
  "category.deleted": "Kategoriya o'chirildi",
  "settings.updated": "Sozlamalar o'zgartirildi",
  "admin.invite_created": "Admin taklifi yaratildi",
  "admin.joined": "Yangi admin qo'shildi",
  "admin.set_active": "Admin bloklandi/yoqildi",
  "admin.removed": "Admin olib tashlandi",
};

export default function AuditPage() {
  const { user } = useAuth();
  const [page, setPage] = useState(1);
  const { data, error, loading } = useApi<Page<AuditRow>>(user?.role === "owner" ? `/admin/audit?page=${page}&limit=50` : null);

  if (user && user.role !== "owner") return <Alert kind="info">Bu bo&apos;lim faqat do&apos;kon egasi (owner) uchun.</Alert>;
  if (loading && !data) return <PageLoader />;

  return (
    <>
      <PageHeader title="Jurnal" sub="Kim nima qilgani" />
      {error && <Alert>{error}</Alert>}
      {data && data.items.length === 0 && <EmptyState title="Hozircha yozuv yo'q" />}
      {data && data.items.length > 0 && (
        <div className="border border-line bg-card">
          <ul className="divide-y divide-line">
            {data.items.map((r) => (
              <li key={r.id} className="grid gap-x-4 gap-y-0.5 px-4 py-3 text-[13px] sm:grid-cols-[150px_150px_1fr]">
                <span className="text-[12px] text-muted">{dateTime(r.createdAt)}</span>
                <span className="font-medium">{r.actor || "tizim"}</span>
                <span>
                  {LABELS[r.action] ?? r.action}
                  {Object.keys(r.meta ?? {}).length > 0 && <span className="ml-2 text-[12px] text-muted">{Object.entries(r.meta).map(([k, v]) => `${k}: ${String(v)}`).join(", ")}</span>}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
      {data && <Pagination page={data.page} limit={data.limit} total={data.total} onPage={setPage} />}
    </>
  );
}
