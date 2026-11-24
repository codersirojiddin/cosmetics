import Link from "next/link";
import { publicGet } from "@/lib/api";
import { SHOP_NAME } from "@/lib/format";

export async function Footer() {
  const s = await publicGet<{ shopName: string; shopPhone: string }>("/settings/public", 120).catch(() => null);
  return (
    <footer className="mt-24 border-t border-line">
      <div className="mx-auto grid max-w-6xl gap-10 px-4 py-12 sm:grid-cols-3">
        <div>
          <p className="text-[15px] font-semibold uppercase tracking-[0.22em]">{s?.shopName || SHOP_NAME}</p>
          <p className="mt-3 max-w-xs text-[13px] text-muted">Teri, soch va makiyaj uchun toza tanlov. Buyurtma Telegram orqali, to&apos;lov chek bilan tasdiqlanadi.</p>
        </div>
        <div className="text-[12px] uppercase tracking-[0.12em]">
          <p className="label-xs mb-3">Sahifalar</p>
          <ul className="space-y-2">
            <li>
              <Link href="/catalog" className="hover:text-accent">
                Katalog
              </Link>
            </li>
            <li>
              <Link href="/orders" className="hover:text-accent">
                Buyurtmalarim
              </Link>
            </li>
          </ul>
        </div>
        <div className="text-[13px]">
          <p className="label-xs mb-3">Aloqa</p>
          {s?.shopPhone ? (
            <a href={`tel:${s.shopPhone.replace(/[^+\d]/g, "")}`} className="text-[15px] font-medium hover:text-accent">
              {s.shopPhone}
            </a>
          ) : (
            <p className="text-muted">Savollar bo&apos;lsa, buyurtma berganingizdan keyin admin siz bilan bog&apos;lanadi.</p>
          )}
        </div>
      </div>
      <div className="border-t border-line py-4 text-center text-[11px] uppercase tracking-[0.14em] text-muted">
        © {new Date().getFullYear()} {s?.shopName || SHOP_NAME}
      </div>
    </footer>
  );
}
