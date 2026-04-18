import { useEffect, useState } from "react";

const KEY = "docsiq-last-visit";

export function useLastVisit() {
  const [last, setLast] = useState<number>(() => {
    const v = localStorage.getItem(KEY);
    return v ? Number(v) : 0;
  });

  function touch() {
    const now = Date.now();
    localStorage.setItem(KEY, String(now));
    setLast(now);
  }

  return { lastVisit: last, touch };
}

export function useTouchOnUnmount() {
  useEffect(() => () => { localStorage.setItem("docsiq-last-visit", String(Date.now())); }, []);
}
