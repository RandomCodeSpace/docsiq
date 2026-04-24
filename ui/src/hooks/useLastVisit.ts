import { useCallback, useEffect, useState } from "react";

const KEY = "docsiq-last-visit";

export function useLastVisit() {
  const [last, setLast] = useState<number>(() => {
    const v = localStorage.getItem(KEY);
    return v ? Number(v) : 0;
  });

  // Stable reference so consumers can safely place `touch` in
  // useEffect dep arrays without triggering an update loop. Without
  // useCallback, `touch` is a fresh function every render which, when
  // used as a cleanup that itself calls setState, causes an infinite
  // render loop (Maximum update depth exceeded).
  const touch = useCallback(() => {
    const now = Date.now();
    localStorage.setItem(KEY, String(now));
    setLast(now);
  }, []);

  return { lastVisit: last, touch };
}

export function useTouchOnUnmount() {
  useEffect(() => () => { localStorage.setItem("docsiq-last-visit", String(Date.now())); }, []);
}
