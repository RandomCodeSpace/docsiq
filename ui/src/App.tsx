import { useEffect } from "react";
import { Providers } from "@/components/layout/Providers";
import { initAuth } from "@/lib/api-client";

export default function App() {
  useEffect(() => {
    initAuth();
  }, []);

  return (
    <Providers>
      <div className="grid min-h-screen place-items-center">
        <div className="text-center">
          <h1 className="font-sans text-2xl font-semibold">docsiq</h1>
          <p className="text-[var(--color-text-muted)] font-mono text-sm mt-2">
            wave-1 scaffold
          </p>
        </div>
      </div>
    </Providers>
  );
}
