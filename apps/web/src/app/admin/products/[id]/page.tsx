"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { ExternalLink } from "lucide-react";
import { api } from "@/lib/api";
import { useAction, useApi } from "@/lib/hooks";
import type { AdminImage, AdminProduct, Category, ProductStatus } from "@/lib/types";
import { Alert, Button, Field, Input, PageLoader, Select, useToast } from "@/components/ui";
import { PageHeader, Panel } from "@/components/admin/bits";
import { ImagesEditor, MarkdownEditor, VariantsEditor, newVariant, type VForm } from "@/components/admin/editors";

interface Form {
  categoryId: string;
  name: string;
  brand: string;
  description: string;
  ingredients: string;
  howToUse: string;
  optionLabel: string;
  status: ProductStatus;
  isNew: boolean;
  isFeatured: boolean;
  images: AdminImage[];
  variants: VForm[];
}

const empty = (): Form => ({ categoryId: "", name: "", brand: "", description: "", ingredients: "", howToUse: "", optionLabel: "", status: "draft", isNew: false, isFeatured: false, images: [], variants: [newVariant()] });

const fromProduct = (p: AdminProduct): Form => ({
  categoryId: p.categoryId,
  name: p.name,
  brand: p.brand,
  description: p.description,
  ingredients: p.ingredients,
  howToUse: p.howToUse,
  optionLabel: p.optionLabel,
  status: p.status,
  isNew: p.isNew,
  isFeatured: p.isFeatured,
  images: p.images,
  variants: p.variants.map((v) => newVariant({ id: v.id, label: v.label, sku: v.sku, price: String(v.price), oldPrice: v.oldPrice ? String(v.oldPrice) : "", stock: String(v.stock), imageId: v.imageId, isActive: v.isActive })),
});

export default function ProductEditPage() {
  const { id } = useParams<{ id: string }>();
  const isNew = id === "new";
  const router = useRouter();
  const toast = useToast();
  const cats = useApi<{ items: Category[] }>("/admin/categories");
  const product = useApi<AdminProduct>(isNew ? null : `/admin/products/${id}`);
  const save = useAction();
  const [f, setF] = useState<Form>(empty);
  const [slug, setSlug] = useState("");
  const [localErr, setLocalErr] = useState<Record<string, string>>({});

  useEffect(() => {
    if (product.data) {
      setF(fromProduct(product.data));
      setSlug(product.data.slug);
    }
  }, [product.data]);

  const set = <K extends keyof Form>(k: K, v: Form[K]) => setF((s) => ({ ...s, [k]: v }));

  async function submit() {
    const errs: Record<string, string> = {};
    if (!f.categoryId) errs.categoryId = "Kategoriyani tanlang";
    if (f.name.trim().length < 2) errs.name = "Nomini kiriting";
    f.variants.forEach((v, i) => {
      if (!Number(v.price)) errs[`variants[${i}].price`] = "Narxni kiriting";
    });
    setLocalErr(errs);
    if (Object.keys(errs).length) {
      toast("Majburiy maydonlarni to'ldiring", "error");
      return;
    }
    const body = {
      categoryId: f.categoryId,
      name: f.name.trim(),
      brand: f.brand.trim(),
      description: f.description,
      ingredients: f.ingredients,
      howToUse: f.howToUse,
      optionLabel: f.optionLabel,
      status: f.status,
      isNew: f.isNew,
      isFeatured: f.isFeatured,
      imageIds: f.images.map((i) => i.id),
      variants: f.variants.map((v) => ({ id: v.id || undefined, label: v.label, sku: v.sku, price: Number(v.price) || 0, oldPrice: v.oldPrice ? Number(v.oldPrice) : null, stock: Number(v.stock) || 0, imageId: v.imageId, isActive: v.isActive })),
    };
    const p = await save.run(() => api<AdminProduct>(isNew ? "/admin/products" : `/admin/products/${id}`, { method: isNew ? "POST" : "PUT", body }));
    if (p) {
      toast(isNew ? "Mahsulot yaratildi" : "Saqlandi");
      setF(fromProduct(p));
      setSlug(p.slug);
      if (isNew) router.replace(`/admin/products/${p.id}`);
    }
  }

  if ((!isNew && product.loading && !product.data) || (cats.loading && !cats.data)) return <PageLoader />;
  if (!isNew && product.error && !product.data) return <Alert>{product.error}</Alert>;

  const errors = { ...save.fields, ...localErr };
  const err = (k: string) => errors[k];

  return (
    <>
      <Link href="/admin/products" className="text-[12px] uppercase tracking-[0.1em] text-muted hover:text-ink">
        ← Mahsulotlar
      </Link>
      <div className="mt-3">
        <PageHeader
          title={isNew ? "Yangi mahsulot" : f.name || "Mahsulot"}
          sub="Mahsulot"
          actions={
            !isNew && f.status === "active" && slug ? (
              <Link href={`/product/${slug}`} target="_blank" className="inline-flex items-center gap-1.5 text-[12px] uppercase tracking-[0.1em] underline underline-offset-4 hover:text-accent">
                Saytda ko&apos;rish <ExternalLink className="size-3.5" />
              </Link>
            ) : undefined
          }
        />
      </div>

      {save.error && (
        <div className="mb-6">
          <Alert>{save.error}</Alert>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="space-y-6">
          <Panel title="Asosiy">
            <div className="space-y-4">
              <Field label="Nomi" required error={err("name")}>
                <Input value={f.name} onChange={(e) => set("name", e.target.value)} invalid={!!err("name")} maxLength={200} />
              </Field>
              <MarkdownEditor label="Tavsif" value={f.description} onChange={(v) => set("description", v)} rows={8} error={err("description")} hint="Markdown: **qalin**, *qiya*, ro'yxat, sarlavha. HTML qabul qilinmaydi." />
              <MarkdownEditor label="Tarkibi" value={f.ingredients} onChange={(v) => set("ingredients", v)} rows={4} />
              <MarkdownEditor label="Ishlatish usuli" value={f.howToUse} onChange={(v) => set("howToUse", v)} rows={4} />
            </div>
          </Panel>

          <Panel title="Rasmlar">
            <ImagesEditor images={f.images} onChange={(v) => setF((s) => ({ ...s, images: v, variants: s.variants.map((x) => (x.imageId && !v.some((im) => im.id === x.imageId) ? { ...x, imageId: "" } : x)) }))} error={err("imageIds")} />
          </Panel>

          <Panel title="Narx, variantlar va qoldiq">
            <VariantsEditor variants={f.variants} onChange={(v) => set("variants", v)} images={f.images} optionLabel={f.optionLabel} onOptionLabel={(v) => set("optionLabel", v)} errors={errors} />
          </Panel>
        </div>

        <aside className="space-y-6 lg:sticky lg:top-6 lg:self-start">
          <Panel title="Nashr">
            <div className="space-y-4">
              <Field label="Holat">
                <Select value={f.status} onChange={(e) => set("status", e.target.value as ProductStatus)}>
                  <option value="draft">Qoralama (saytda ko&apos;rinmaydi)</option>
                  <option value="active">Faol (saytda ko&apos;rinadi)</option>
                  <option value="archived">Arxiv</option>
                </Select>
              </Field>
              <Field label="Kategoriya" required error={err("categoryId")}>
                <Select value={f.categoryId} onChange={(e) => set("categoryId", e.target.value)} invalid={!!err("categoryId")}>
                  <option value="">Tanlang…</option>
                  {(cats.data?.items ?? []).map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label="Brend">
                <Input value={f.brand} onChange={(e) => set("brand", e.target.value)} maxLength={100} />
              </Field>
              <div className="space-y-2">
                {([["isNew", "«Yangi» belgisi"], ["isFeatured", "Bosh sahifada «Mashhur»"]] as const).map(([k, label]) => (
                  <label key={k} className="flex items-center gap-2 text-[13px]">
                    <input type="checkbox" checked={f[k]} onChange={(e) => set(k, e.target.checked)} className="size-4 accent-[var(--color-ink)]" /> {label}
                  </label>
                ))}
              </div>
              <Button size="lg" className="w-full" loading={save.busy} onClick={() => void submit()}>
                {isNew ? "Yaratish" : "Saqlash"}
              </Button>
            </div>
          </Panel>
        </aside>
      </div>
    </>
  );
}
