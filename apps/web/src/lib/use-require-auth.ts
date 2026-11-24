"use client";

import { useEffect } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "./auth";

/** Redirect to /login (and back afterwards) when nobody is signed in. */
export function useRequireAuth() {
  const auth = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams().toString();
  useEffect(() => {
    if (!auth.loading && !auth.user) {
      const back = pathname + (search ? `?${search}` : "");
      router.replace(`/login?next=${encodeURIComponent(back)}`);
    }
  }, [auth.loading, auth.user, pathname, search, router]);
  return auth;
}
