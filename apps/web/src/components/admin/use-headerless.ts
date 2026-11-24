"use client";

import { useEffect } from "react";

/** The admin area has its own chrome: make sure search engines and previews ignore it. */
export function useHeaderless() {
  useEffect(() => {
    const m = document.createElement("meta");
    m.name = "robots";
    m.content = "noindex, nofollow";
    document.head.appendChild(m);
    return () => m.remove();
  }, []);
}
