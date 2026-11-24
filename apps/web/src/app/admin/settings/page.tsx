"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useAction, useApi } from "@/lib/hooks";
import type { Settings } from "@/lib/types";
import { Alert, Button, Field, Input, PageLoader, Textarea, useToast } from "@/components/ui";
import { PageHeader, Panel } from "@/components/admin/bits";

export default function SettingsPage() {
  const { user } = useAuth();
  const toast = useToast();
  const { data, error, loading, setData } = useApi<Settings>("/admin/settings");
  const [f, setF] = useState<Settings | null>(null);
  const save = useAction();
  const owner = user?.role === "owner";

  useEffect(() => {
    if (data) setF(data);
  }, [data]);

  if (loading && !data) return <PageLoader />;
  if (error || !f) return <Alert>{error ?? "Sozlamalarni yuklab bo'lmadi"}</Alert>;

  const set = (k: keyof Settings, v: string) => setF({ ...f, [k]: v });
  const submit = async () => {
    const r = await save.run(() => api<Settings>("/admin/settings", { method: "PUT", body: f }));
    if (r) {
      setData(r);
      toast("Sozlamalar saqlandi");
    }
  };

  return (
    <>
      <PageHeader title="Sozlamalar" sub="Do'kon" />
      {!owner && (
        <div className="mb-6">
          <Alert kind="info">Sozlamalarni faqat do&apos;kon egasi (owner) o&apos;zgartira oladi. Bu yerda ularni ko&apos;rishingiz mumkin.</Alert>
        </div>
      )}
      {save.error && (
        <div className="mb-6">
          <Alert>{save.error}</Alert>
        </div>
      )}
      <div className="max-w-2xl space-y-6">
        <Panel title="To'lov ma'lumotlari (mijoz checkout'da ko'radi)">
          <div className="space-y-4">
            <Field label="Karta raqami" error={save.fields.cardNumber} hint="Mijozlar pulni shu kartaga o'tkazadi. Diqqat bilan tekshiring!">
              <Input value={f.cardNumber} onChange={(e) => set("cardNumber", e.target.value)} disabled={!owner} inputMode="numeric" placeholder="8600 0000 0000 0000" maxLength={32} invalid={!!save.fields.cardNumber} />
            </Field>
            <Field label="Karta egasi">
              <Input value={f.cardHolder} onChange={(e) => set("cardHolder", e.target.value)} disabled={!owner} placeholder="F. FAMILIYA" maxLength={100} />
            </Field>
            <Field label="To'lov bo'yicha izoh" hint="Masalan: izohga hech narsa yozish shart emas">
              <Textarea rows={2} value={f.paymentNote} onChange={(e) => set("paymentNote", e.target.value)} disabled={!owner} maxLength={500} />
            </Field>
          </div>
        </Panel>
        <Panel title="Do'kon">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Do'kon nomi">
              <Input value={f.shopName} onChange={(e) => set("shopName", e.target.value)} disabled={!owner} maxLength={100} />
            </Field>
            <Field label="Telefon" hint="Saytning pastki qismida ko'rinadi">
              <Input value={f.shopPhone} onChange={(e) => set("shopPhone", e.target.value)} disabled={!owner} inputMode="tel" maxLength={30} />
            </Field>
          </div>
        </Panel>
        {owner && (
          <Button size="lg" loading={save.busy} onClick={() => void submit()}>
            Saqlash
          </Button>
        )}
      </div>
    </>
  );
}
