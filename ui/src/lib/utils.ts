import { clsx, type ClassValue } from 'clsx'
export function cn(...inputs: ClassValue[]) { return clsx(inputs) }

export function colorJSON(data: unknown): string {
  return esc(JSON.stringify(data, null, 2))
    .replace(/"([^"]+)":/g, '<span style="color:var(--syntax-key)">"$1"</span>:')
    .replace(/: "([^"]*)"/g, ': <span style="color:var(--syntax-str)">"$1"</span>')
    .replace(/: (true|false)/g, ': <span style="color:var(--syntax-bool)">$1</span>')
    .replace(/: (null)/g, ': <span style="color:var(--syntax-null)">$1</span>')
    .replace(/: (-?\d+\.?\d*)/g, ': <span style="color:var(--syntax-num)">$1</span>')
}

export function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

export function fmt(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}
