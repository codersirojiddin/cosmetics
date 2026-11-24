import { Suspense } from "react";
import type { Metadata } from "next";
import { AdminShell } from "@/components/admin/shell";
import { PageLoader } from "@/components/ui";

export const metadata: Metadata = { title: "Admin", robots: { index: false, follow: false } };

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <Suspense fallback={<PageLoader />}>
      <AdminShell>{children}</AdminShell>
    </Suspense>
  );
}
