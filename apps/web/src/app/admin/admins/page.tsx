"use client";

import { useState } from "react";
import { Copy, UserPlus } from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { dateTime, fullName } from "@/lib/format";
import { useAction, useApi } from "@/lib/hooks";
import type { StaffUser } from "@/lib/types";
import { Alert, Button, Modal, PageLoader, Tag, useToast } from "@/components/ui";
import { PageHeader } from "@/components/admin/bits";

interface Invite {
  code: string;
  deepLink: string;
  expiresAt: string;
}

export default function AdminsPage() {
  const { user } = useAuth();
  const toast = useToast();
  const { data, error, loading, reload } = useApi<{ items: StaffUser[] }>(user?.role === "owner" ? "/admin/admins" : null);
  const [invite, setInvite] = useState<Invite | null>(null);
  const [remove, setRemove] = useState<StaffUser | null>(null);
  const act = useAction();

  if (user && user.role !== "owner") return <Alert kind="info">Bu bo&apos;lim faqat do&apos;kon egasi (owner) uchun.</Alert>;
  if (loading && !data) return <PageLoader />;

  const createInvite = async () => {
    const r = await act.run(() => api<Invite>("/admin/admins/invites", { method: "POST" }));
    if (r) setInvite(r);
  };
  const toggle = async (a: StaffUser) => {
    const r = await act.run(() => api<StaffUser>(`/admin/admins/${a.id}`, { method: "PATCH", body: { isActive: !a.isActive } }));
    if (r) {
      toast(r.isActive ? "Admin qayta yoqildi" : "Admin bloklandi");
      void reload();
    }
  };
  const doRemove = async () => {
    if (!remove) return;
    const ok = await act.run(() => api(`/admin/admins/${remove.id}`, { method: "DELETE" }).then(() => true));
    if (ok) {
      toast("Admin olib tashlandi");
      setRemove(null);
      void reload();
    }
  };

  return (
    <>
      <PageHeader
        title="Adminlar"
        sub="Jamoa"
        actions={
          <Button onClick={() => void createInvite()} loading={act.busy && !remove}>
            <UserPlus className="size-4" /> Admin qo&apos;shish
          </Button>
        }
      />
      {(error || act.error) && !remove && <Alert>{error ?? act.error}</Alert>}

      <div className="mt-4 border border-line bg-card">
        <ul className="divide-y divide-line">
          {(data?.items ?? []).map((a) => (
            <li key={a.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3.5">
              <div className="min-w-0 flex-1">
                <p className="text-[13px] font-medium">
                  {fullName(a)} {a.id === user?.id && <span className="text-muted">(siz)</span>}
                </p>
                <p className="text-[12px] text-muted">
                  {a.username ? `@${a.username} · ` : ""}
                  {a.lastLoginAt ? `oxirgi kirish ${dateTime(a.lastLoginAt)}` : "hali kirmagan"}
                </p>
              </div>
              <Tag tone={a.role === "owner" ? "accent" : "ink"}>{a.role === "owner" ? "Owner" : "Admin"}</Tag>
              {!a.isActive && <Tag tone="muted">Bloklangan</Tag>}
              {a.role === "admin" && (
                <div className="flex gap-1">
                  <Button variant="secondary" size="sm" onClick={() => void toggle(a)} disabled={act.busy}>
                    {a.isActive ? "Bloklash" : "Yoqish"}
                  </Button>
                  <Button variant="danger" size="sm" onClick={() => { act.setError(null); setRemove(a); }}>
                    Olib tashlash
                  </Button>
                </div>
              )}
            </li>
          ))}
        </ul>
      </div>
      <p className="mt-4 max-w-xl text-[12px] text-muted">Yangi admin: «Admin qo&apos;shish» ni bosing, chiqqan havolani xodimga yuboring. U havolani Telegram&apos;da ochib «Start» bosishi bilan admin bo&apos;ladi. Havola bir marta va 24 soat ishlaydi.</p>

      <Modal open={!!invite} onClose={() => setInvite(null)} title="Taklif havolasi">
        {invite && (
          <div className="space-y-4">
            <p className="text-[13px]">Bu havolani yangi adminga yuboring. U Telegram&apos;da ochib «Start» bosadi.</p>
            <div className="break-all border border-ink bg-paper p-3 text-[13px]">{invite.deepLink}</div>
            <p className="text-[12px] text-muted">Amal qilish muddati: {dateTime(invite.expiresAt)} gacha. Faqat bir marta ishlatiladi.</p>
            <Button className="w-full" onClick={() => navigator.clipboard?.writeText(invite.deepLink).then(() => toast("Havola nusxalandi"))}>
              <Copy className="size-4" /> Havolani nusxalash
            </Button>
          </div>
        )}
      </Modal>

      <Modal open={!!remove} onClose={() => setRemove(null)} title="Adminni olib tashlash">
        <p className="text-[13px]">
          <b>{remove ? fullName(remove) : ""}</b> oddiy mijozga aylantiriladi va admin panelga kira olmaydi.
        </p>
        {act.error && (
          <div className="mt-4">
            <Alert>{act.error}</Alert>
          </div>
        )}
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setRemove(null)}>
            Bekor qilish
          </Button>
          <Button variant="danger" loading={act.busy} onClick={() => void doRemove()}>
            Olib tashlash
          </Button>
        </div>
      </Modal>
    </>
  );
}
