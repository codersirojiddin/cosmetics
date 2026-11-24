"use client";

import { useRef, useState } from "react";
import { ImagePlus, Plus } from "lucide-react";
import { api, errorInfo } from "@/lib/api";
import { downscale } from "@/lib/image";
import { useAction, useApi } from "@/lib/hooks";
import { cx } from "@/lib/cx";
import type { Category, ImageURLs } from "@/lib/types";
import { Alert, Button, EmptyState, Field, Input, Modal, PageLoader, Tag, useToast } from "@/components/ui";
import { PageHeader } from "@/components/admin/bits";

interface Draft {
  id?: string;
  name: string;
  position: string;
  isActive: boolean;
  imageId: string;
  image: ImageURLs | null;
}

const blank: Draft = { name: "", position: "0", isActive: true, imageId: "", image: null };

export default function CategoriesPage() {
  const toast = useToast();
  const { data, error, loading, reload } = useApi<{ items: Category[] }>("/admin/categories");
  const [draft, setDraft] = useState<Draft | null>(null);
  const [del, setDel] = useState<Category | null>(null);
  const [uploading, setUploading] = useState(false);
  const act = useAction();
  const file = useRef<HTMLInputElement>(null);

  async function upload(f?: File) {
    if (!f) return;
    setUploading(true);
    try {
      const blob = await downscale(f, 1600, 0.9);
      const form = new FormData();
      form.append("file", blob, "category.jpg");
      const r = await api<{ id: string; urls: ImageURLs }>("/admin/uploads/image?kind=category", { form });
      setDraft((d) => d && { ...d, imageId: r.id, image: r.urls });
    } catch (e) {
      const info = errorInfo(e);
      toast(info.fields.file ?? info.message, "error");
    } finally {
      setUploading(false);
      if (file.current) file.current.value = "";
    }
  }

  async function save() {
    if (!draft) return;
    const body = { name: draft.name.trim(), imageId: draft.imageId, position: Number(draft.position) || 0, isActive: draft.isActive };
    const r = await act.run(() => api<Category>(draft.id ? `/admin/categories/${draft.id}` : "/admin/categories", { method: draft.id ? "PUT" : "POST", body }));
    if (r) {
      toast(draft.id ? "Saqlandi" : "Kategoriya yaratildi");
      setDraft(null);
      void reload();
    }
  }

  async function remove() {
    if (!del) return;
    const ok = await act.run(() => api(`/admin/categories/${del.id}`, { method: "DELETE" }).then(() => true));
    if (ok) {
      toast("Kategoriya o'chirildi");
      setDel(null);
      void reload();
    }
  }

  return (
    <>
      <PageHeader
        title="Kategoriyalar"
        sub="Katalog"
        actions={
          <Button onClick={() => { act.setError(null); setDraft({ ...blank, position: String((data?.items.length ?? 0) + 1) }); }}>
            <Plus className="size-4" /> Yangi kategoriya
          </Button>
        }
      />
      {error && <Alert>{error}</Alert>}
      {loading && !data ? (
        <PageLoader />
      ) : data && data.items.length > 0 ? (
        <div className="border border-line bg-card">
          <ul className="divide-y divide-line">
            {data.items.map((c) => (
              <li key={c.id} className="flex items-center gap-4 px-4 py-3">
                {c.image ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={c.image.thumb} alt="" className="size-12 border border-line object-cover" />
                ) : (
                  <div className="size-12 border border-dashed border-line" />
                )}
                <button className="min-w-0 flex-1 text-left" onClick={() => { act.setError(null); setDraft({ id: c.id, name: c.name, position: String(c.position), isActive: c.isActive, imageId: c.imageId, image: c.image }); }}>
                  <p className="truncate text-[13px] font-medium uppercase tracking-[0.04em]">{c.name}</p>
                  <p className="text-[12px] text-muted">
                    /{c.slug} · {c.productCount} ta faol mahsulot
                  </p>
                </button>
                {!c.isActive && <Tag tone="muted">Yashirin</Tag>}
                <Button variant="ghost" size="sm" onClick={() => { act.setError(null); setDel(c); }}>
                  O&apos;chirish
                </Button>
              </li>
            ))}
          </ul>
        </div>
      ) : (
        !error && <EmptyState title="Kategoriya yo'q">Mahsulot qo&apos;shishdan oldin kamida bitta kategoriya yarating.</EmptyState>
      )}

      <Modal open={!!draft} onClose={() => setDraft(null)} title={draft?.id ? "Kategoriyani tahrirlash" : "Yangi kategoriya"}>
        {draft && (
          <div className="space-y-4">
            <Field label="Nomi" required error={act.fields.name}>
              <Input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} maxLength={100} autoFocus invalid={!!act.fields.name} />
            </Field>
            <div className="grid grid-cols-2 gap-4">
              <Field label="Tartib raqami" hint="Kichigi birinchi chiqadi">
                <Input value={draft.position} onChange={(e) => setDraft({ ...draft, position: e.target.value.replace(/\D/g, "") })} inputMode="numeric" />
              </Field>
              <label className="mt-6 flex items-center gap-2 text-[13px]">
                <input type="checkbox" checked={draft.isActive} onChange={(e) => setDraft({ ...draft, isActive: e.target.checked })} className="size-4 accent-[var(--color-ink)]" /> Saytda ko&apos;rsatish
              </label>
            </div>
            <div>
              <span className="label-xs mb-1.5 block">Rasm (ixtiyoriy)</span>
              <input ref={file} type="file" accept="image/*" className="sr-only" onChange={(e) => void upload(e.target.files?.[0])} />
              <div className="flex items-center gap-3">
                {draft.image && (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={draft.image.thumb} alt="" className="size-16 border border-line object-cover" />
                )}
                <Button variant="secondary" size="sm" loading={uploading} onClick={() => file.current?.click()}>
                  <ImagePlus className="size-4" /> {draft.image ? "Almashtirish" : "Yuklash"}
                </Button>
                {draft.image && (
                  <button className={cx("text-[12px] uppercase tracking-[0.1em] text-bad underline underline-offset-4")} onClick={() => setDraft({ ...draft, imageId: "", image: null })}>
                    Olib tashlash
                  </button>
                )}
              </div>
              {act.fields.imageId && <p className="mt-1 text-[12px] text-bad">{act.fields.imageId}</p>}
            </div>
            {act.error && <Alert>{act.error}</Alert>}
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="ghost" onClick={() => setDraft(null)}>
                Bekor qilish
              </Button>
              <Button loading={act.busy} onClick={() => void save()}>
                Saqlash
              </Button>
            </div>
          </div>
        )}
      </Modal>

      <Modal open={!!del} onClose={() => setDel(null)} title="Kategoriyani o'chirish">
        <p className="text-[13px]">
          <b>{del?.name}</b> kategoriyasini o&apos;chirasizmi? Agar unda mahsulotlar bo&apos;lsa, avval ularni boshqa kategoriyaga ko&apos;chirishingiz kerak.
        </p>
        {act.error && (
          <div className="mt-4">
            <Alert>{act.error}</Alert>
          </div>
        )}
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setDel(null)}>
            Bekor qilish
          </Button>
          <Button variant="danger" loading={act.busy} onClick={() => void remove()}>
            O&apos;chirish
          </Button>
        </div>
      </Modal>
    </>
  );
}
