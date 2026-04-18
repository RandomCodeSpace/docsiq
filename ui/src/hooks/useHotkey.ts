import { useEffect, useRef } from "react";

interface Options {
  enabled?: boolean;
  preventDefault?: boolean;
}

// combo: "mod+k" or "mod+\\" or "g,h" (chord — g then h within 1s)
export function useHotkey(
  combo: string,
  handler: (e: KeyboardEvent) => void,
  opts: Options = {},
) {
  const { enabled = true, preventDefault = true } = opts;
  const handlerRef = useRef(handler);
  handlerRef.current = handler;

  useEffect(() => {
    if (!enabled) return;
    const chord = combo.includes(",");
    let lastKey: string | null = null;
    let lastTime = 0;

    const onKeyDown = (e: KeyboardEvent) => {
      const key = e.key.toLowerCase();
      const mod = e.metaKey || e.ctrlKey;

      if (chord) {
        const [first, second] = combo.split(",");
        if (!lastKey) {
          if (key === first) { lastKey = key; lastTime = Date.now(); return; }
        } else {
          if (key === second && Date.now() - lastTime < 1000) {
            if (preventDefault) e.preventDefault();
            handlerRef.current(e);
          }
          lastKey = null;
        }
        return;
      }

      const parts = combo.split("+");
      const needsMod = parts.includes("mod");
      const target = parts[parts.length - 1];
      if (needsMod === mod && key === target) {
        if (preventDefault) e.preventDefault();
        handlerRef.current(e);
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [combo, enabled, preventDefault]);
}
