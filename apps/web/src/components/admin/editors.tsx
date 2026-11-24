"use client";

import { useRef, useState } from "react";
import { ArrowLeft, ArrowRight, Bold, Heading2, ImagePlus, Italic, Link2, List, Plus, Trash2, X } from "lucide-react";
import { api, errorInfo } from "@/lib/api";
import { downscale } from "@/lib/image";
import { cx } from "@/lib/cx";
import type { AdminImage } from "@/lib/types";
import { Markdown } from "../markdown";
import { Button, Field, Input, Select, inputCls, useToast } from "../ui";

// ---------- Markdown editor ----------

export function MarkdownEditor({ label, value, onChange, rows = 7, hint, error }: { label: string; value: string; onChange: (v: string) => void; rows?: number; hint?: string; error?: string }) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const [preview, setPreview] = useState(false);

  const edit = (fn: (v: string, s: number, e: number) => { text: string; from: number; to: number }) => {
    const el = ref.current;
    if (!el) return;
    const r = fn(value, el.selectionStart, el.selectionEnd);
    onChange(r.text);
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(r.from, r.to);
    });
  };
  const wrap = (mark: string, ph = "matn") =>
    edit((v, s, e) => {
      const sel = v.slice(s, e) || ph;
      return { text: v.slice(0, s) + mark + sel + mark + v.slice(e), from: s + mark.length, to: s + mark.length + sel.length };
    });
  const prefixLines = (prefix: string) =>
    edit((v, s, e) => {
      const start = v.lastIndexOf("\n", s - 1) + 1;
      const nl = v.indexOf("\n", e);
      const end = nl === -1 ? v.length : nl;
      const block = v.slice(start, end).split("\n").map((l) => prefix + l).join("\n");
      return { text: v.slice(0, start) + block + v.slice(end), from: start, to: start + block.length };
    });
  const link = () =>
    edit((v, s, e) => {
      const sel = v.slice(s, e) || "matn";
      const ins = `[${sel}](https://)`;
      return { text: v.slice(0, s) + ins + v.slice(e), from: s + sel.length + 3, to: s + ins.length - 1 };
    });

  const tools = [
    { icon: Bold, title: "Qalin", run: () => wrap("**") },
    { icon: Italic, title: "Qiya", run: () => wrap("*") },
    { icon: Heading2, title: "Sarlavha", run: () => prefixLines("## ") },
    { icon: List, title: "Ro'yxat", run: () => prefixLines("- ") },
    { icon: Link2, title: "Havola", run: link },
  ];

  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <span className="label-xs">{label}</span>
        <div className="flex">
          {(["Yozish", "Ko'rinish"] as const).map((t, i) => (
            <button key={t} type="button" onClick={() => setPreview(i === 1)} className={cx("border border-line px-2.5 py-1 text-[11px] uppercase tracking-[0.08em]", i === 1 && "border-l-0", preview === (i === 1) ? "bg-ink text-paper" : "bg-card text-muted hover:text-ink")}>
              {t}
            </button>
          ))}
        </div>
      </div>
      {!preview && (
        <div className="flex gap-0.5 border border-b-0 border-line bg-paper p-1">
          {tools.map((t) => (
            <button key={t.title} type="button" title={t.title} aria-label={t.title} onClick={t.run} className="grid size-8 place-items-center text-muted hover:bg-ink/5 hover:text-ink">
              <t.icon className="size-4" />
            </button>
          ))}
        </div>
      )}
      {preview ? (
        <div className="min-h-32 border border-line bg-card p-4">{value.trim() ? <Markdown>{value}</Markdown> : <p className="text-[13px] text-muted">Hali hech narsa yozilmagan</p>}</div>
      ) : (
        <textarea ref={ref} rows={rows} value={value} onChange={(e) => onChange(e.target.value)} className={cx(inputCls, "h-auto resize-y py-3 leading-relaxed")} aria-invalid={!!error || undefined} maxLength={10000} />
      )}
      {hint && !error && <p className="mt-1 text-[12px] text-muted">{hint}</p>}
      {error && <p className="mt-1 text-[12px] text-bad">{error}</p>}
    </div>
  );
}

// ---------- images ----------

export function ImagesEditor({ images, onChange, error }: { images: AdminImage[]; onChange: (v: AdminImage[]) => void; error?: string }) {
  const toast = useToast();
  const [busy, setBusy] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const MAX = 8;

  async function upload(files: FileList | null) {
    if (!files) return;
    let list = [...images];
    for (const f of Array.from(files).slice(0, MAX - images.length)) {
      setBusy((b) => b + 1);
      try {
        const blob = await downscale(f, 2400, 0.9);
        const form = new FormData();
        form.append("file", blob, "image.jpg");
        const r = await api<AdminImage>("/admin/uploads/image?kind=product", { form });
        list = [...list, { id: r.id, urls: r.urls }];
        onChange(list);
      } catch (e) {
        const info = errorInfo(e);
        toast(`${f.name}: ${info.fields.file ?? info.message}`, "error");
      } finally {
        setBusy((b) => b - 1);
        if (input.current) input.current.value = "";
      }
    }
  }
  const move = (i: number, d: number) => {
    const j = i + d;
    if (j < 0 || j >= images.length) return;
    const next = [...images];
    [next[i], next[j]] = [next[j], next[i]];
    onChange(next);
  };

  return (
    <div>
      <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
        {images.map((im, i) => (
          <div key={im.id} className="group relative">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={im.urls.thumb} alt={`Rasm ${i + 1}`} className="aspect-square w-full border border-line object-cover" />
            {i === 0 && <span className="absolute left-1 top-1 bg-ink px-1.5 py-0.5 text-[9px] uppercase tracking-[0.1em] text-paper">Asosiy</span>}
            <button type="button" onClick={() => onChange(images.filter((x) => x.id !== im.id))} aria-label="Rasmni olib tashlash" className="absolute right-1 top-1 grid size-6 place-items-center bg-card/90 text-bad hover:bg-bad hover:text-white">
              <X className="size-3.5" />
            </button>
            <div className="mt-1 flex justify-center gap-1">
              <button type="button" onClick={() => move(i, -1)} disabled={i === 0} aria-label="Oldinga" className="p-1 text-muted hover:text-ink disabled:opacity-25">
                <ArrowLeft className="size-3.5" />
              </button>
              <button type="button" onClick={() => move(i, 1)} disabled={i === images.length - 1} aria-label="Orqaga" className="p-1 text-muted hover:text-ink disabled:opacity-25">
                <ArrowRight className="size-3.5" />
              </button>
            </div>
          </div>
        ))}
        {Array.from({ length: busy }).map((_, i) => (
          <div key={`b${i}`} className="aspect-square animate-pulse border border-line bg-line/50" />
        ))}
        {images.length + busy < MAX && (
          <button type="button" onClick={() => input.current?.click()} className="flex aspect-square flex-col items-center justify-center gap-1.5 border border-dashed border-ink text-[11px] uppercase tracking-[0.1em] hover:bg-ink/[0.03]">
            <ImagePlus className="size-5" /> Qo&apos;shish
          </button>
        )}
      </div>
      <input ref={input} type="file" accept="image/*" multiple className="sr-only" onChange={(e) => void upload(e.target.files)} />
      <p className="mt-2 text-[12px] text-muted">Kamida 300×300 px. Birinchi rasm asosiy. {MAX} tagacha.</p>
      {error && <p className="mt-1 text-[12px] text-bad">{error}</p>}
    </div>
  );
}

// ---------- variants ----------

export interface VForm {
  key: string;
  id?: string;
  label: string;
  sku: string;
  price: string;
  oldPrice: string;
  stock: string;
  imageId: string;
  isActive: boolean;
}

export const newVariant = (over: Partial<VForm> = {}): VForm => ({ key: Math.random().toString(36).slice(2), label: "", sku: "", price: "", oldPrice: "", stock: "0", imageId: "", isActive: true, ...over });

export function VariantsEditor({ variants, onChange, images, optionLabel, onOptionLabel, errors }: { variants: VForm[]; onChange: (v: VForm[]) => void; images: AdminImage[]; optionLabel: string; onOptionLabel: (v: string) => void; errors: Record<string, string> }) {
  const multi = variants.length > 1;
  const set = (i: number, patch: Partial<VForm>) => onChange(variants.map((v, j) => (j === i ? { ...v, ...patch } : v)));
  const e = (i: number, k: string) => errors[`variants[${i}].${k}`];

  return (
    <div className="space-y-4">
      {multi && (
        <Field label="Variant turi" hint="Masalan: Tus, Hajm, Rang. Mijoz shu nom bilan tanlaydi.">
          <Input value={optionLabel} onChange={(ev) => onOptionLabel(ev.target.value)} placeholder="Tus" maxLength={40} />
        </Field>
      )}
      {variants.map((v, i) => (
        <div key={v.key} className={cx("border border-line bg-card p-4", !v.isActive && "opacity-60")}>
          <div className="mb-3 flex items-center justify-between">
            <span className="label-xs">{multi ? `Variant ${i + 1}` : "Narx va qoldiq"}</span>
            <div className="flex items-center gap-3">
              <label className="flex items-center gap-1.5 text-[12px] uppercase tracking-[0.08em]">
                <input type="checkbox" checked={v.isActive} onChange={(ev) => set(i, { isActive: ev.target.checked })} className="size-4 accent-[var(--color-ink)]" /> Faol
              </label>
              {multi && (
                <button type="button" onClick={() => onChange(variants.filter((_, j) => j !== i))} aria-label="Variantni o'chirish" className="p-1 text-bad hover:bg-bad/10">
                  <Trash2 className="size-4" />
                </button>
              )}
            </div>
          </div>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {multi && (
              <Field label="Nomi" required error={e(i, "label")} className="lg:col-span-2">
                <Input value={v.label} onChange={(ev) => set(i, { label: ev.target.value })} placeholder="Rose / 50 ml" invalid={!!e(i, "label")} maxLength={80} />
              </Field>
            )}
            <Field label="Narx (so'm)" required error={e(i, "price")}>
              <Input value={v.price} onChange={(ev) => set(i, { price: ev.target.value.replace(/\D/g, "") })} inputMode="numeric" placeholder="85000" invalid={!!e(i, "price")} />
            </Field>
            <Field label="Eski narx" error={e(i, "oldPrice")} hint="Chegirma uchun">
              <Input value={v.oldPrice} onChange={(ev) => set(i, { oldPrice: ev.target.value.replace(/\D/g, "") })} inputMode="numeric" invalid={!!e(i, "oldPrice")} />
            </Field>
            <Field label="Qoldiq (dona)" error={e(i, "stock")}>
              <Input value={v.stock} onChange={(ev) => set(i, { stock: ev.target.value.replace(/\D/g, "") })} inputMode="numeric" invalid={!!e(i, "stock")} />
            </Field>
            <Field label="SKU" error={e(i, "sku")}>
              <Input value={v.sku} onChange={(ev) => set(i, { sku: ev.target.value })} placeholder="ixtiyoriy" invalid={!!e(i, "sku")} maxLength={64} />
            </Field>
            {multi && images.length > 0 && (
              <Field label="Variant rasmi" error={e(i, "imageId")}>
                <Select value={v.imageId} onChange={(ev) => set(i, { imageId: ev.target.value })}>
                  <option value="">Asosiy rasm</option>
                  {images.map((im, k) => (
                    <option key={im.id} value={im.id}>
                      Rasm {k + 1}
                    </option>
                  ))}
                </Select>
              </Field>
            )}
          </div>
        </div>
      ))}
      {errors.variants && <p className="text-[12px] text-bad">{errors.variants}</p>}
      <Button variant="secondary" size="sm" onClick={() => onChange([...variants, newVariant()])}>
        <Plus className="size-4" /> {multi ? "Yana variant qo'shish" : "Variantlar qo'shish (tus, hajm)"}
      </Button>
    </div>
  );
}
