import Link from "next/link";

export default function NotFound() {
  return (
    <div className="mx-auto max-w-md px-4 py-32 text-center">
      <p className="text-[56px] font-semibold leading-none">404</p>
      <p className="mt-4 text-[13px] font-semibold uppercase tracking-[0.14em]">Sahifa topilmadi</p>
      <p className="mt-2 text-[13px] text-muted">Manzil noto&apos;g&apos;ri yoki sahifa o&apos;chirilgan.</p>
      <Link href="/" className="mt-8 inline-block border border-ink px-5 py-3 text-[12px] font-medium uppercase tracking-[0.1em] hover:bg-ink hover:text-paper">
        Bosh sahifaga
      </Link>
    </div>
  );
}
