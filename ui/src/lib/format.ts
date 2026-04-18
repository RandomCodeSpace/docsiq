const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

export function formatRelativeTime(fromMs: number, now: number = Date.now()): string {
  const diffMs = fromMs - now;
  const abs = Math.abs(diffMs);
  const min = 60_000;
  const hr = 60 * min;
  const day = 24 * hr;
  if (abs < min) return rtf.format(Math.round(diffMs / 1000), "second");
  if (abs < hr) return rtf.format(Math.round(diffMs / min), "minute");
  if (abs < day) return rtf.format(Math.round(diffMs / hr), "hour");
  return rtf.format(Math.round(diffMs / day), "day");
}

export function formatCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return (n / 1000).toFixed(n < 10_000 ? 1 : 0) + "k";
  return (n / 1_000_000).toFixed(1) + "m";
}
