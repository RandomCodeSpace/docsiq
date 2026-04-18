import { en } from "./en";

type PathsOf<T, P extends string = ""> = T extends string
  ? P
  : T extends object
  ? {
      [K in keyof T & string]: PathsOf<T[K], P extends "" ? K : `${P}.${K}`>;
    }[keyof T & string]
  : never;

export type MessageKey = PathsOf<typeof en>;

export function t(key: MessageKey): string {
  const parts = (key as string).split(".");
  let cur: unknown = en;
  for (const p of parts) {
    if (typeof cur !== "object" || cur === null || !(p in cur)) return key as string;
    cur = (cur as Record<string, unknown>)[p];
  }
  return typeof cur === "string" ? cur : (key as string);
}
