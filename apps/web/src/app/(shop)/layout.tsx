import { Header } from "@/components/header";
import { Footer } from "@/components/footer";

export default function ShopLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Header />
      <main className="mx-auto max-w-6xl px-4">{children}</main>
      <Footer />
    </>
  );
}
