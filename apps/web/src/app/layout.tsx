import type { Metadata, Viewport } from "next";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "@fontsource/ibm-plex-mono/600.css";
import "@fontsource-variable/inter";
import "./globals.css";
import { AuthProvider } from "@/lib/auth";
import { ToastProvider } from "@/components/ui";
import { SHOP_NAME } from "@/lib/format";

export const metadata: Metadata = {
  title: { default: `${SHOP_NAME}: kosmetika do'koni`, template: `%s | ${SHOP_NAME}` },
  description: "Teri, soch va makiyaj uchun sifatli kosmetika. Telegram orqali tez va oson buyurtma bering.",
};

export const viewport: Viewport = { themeColor: "#f6f3ee", width: "device-width", initialScale: 1 };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="uz">
      <body className="min-h-dvh">
        <AuthProvider>
          <ToastProvider>{children}</ToastProvider>
        </AuthProvider>
      </body>
    </html>
  );
}
