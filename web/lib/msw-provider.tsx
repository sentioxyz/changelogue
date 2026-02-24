// web/lib/msw-provider.tsx
"use client";

import { useEffect, useState, type ReactNode } from "react";

export function MSWProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (process.env.NODE_ENV !== "development") {
      setReady(true);
      return;
    }

    import("./mock/browser").then(({ worker }) => {
      worker.start({ onUnhandledRequest: "bypass" }).then(() => setReady(true));
    });
  }, []);

  if (!ready) return null;
  return <>{children}</>;
}
