import type { Artwork, Exhibition } from "./types";

export const WALL_NAMES = ["North wall", "East wall", "South wall", "West wall"];

export function cm(v: number) {
  return v >= 100 ? `${(v / 100).toFixed(2)} m` : `${Math.round(v * 10) / 10} cm`;
}

/** "73 × 92 cm (28 ¾ × 36 ¼ in.)", height first as museums write it. */
export function dimensions(a: Artwork) {
  if (!a.size) return null;
  const inches = (c: number) => {
    const v = c / 2.54;
    const whole = Math.floor(v);
    const q = Math.round((v - whole) * 4);
    const frac = ["", "¼", "½", "¾", ""][q];
    return `${q === 4 ? whole + 1 : whole}${frac ? " " + frac : ""}`;
  };
  return `${a.size.hCm} × ${a.size.wCm} cm (${inches(a.size.hCm)} × ${inches(a.size.wCm)} in.)`;
}

export function kindName(k: string, plural = false) {
  const names: Record<string, [string, string]> = {
    painting: ["Painting", "Paintings"],
    print: ["Print", "Prints"],
    drawing: ["Drawing", "Drawings"],
  };
  return names[k]?.[plural ? 1 : 0] ?? k;
}

export function count(n: number, one: string, many: string) {
  return `${n.toLocaleString("en")} ${n === 1 ? one : many}`;
}

export function relativeDays(iso: string, now = Date.now()) {
  const d = new Date(iso).getTime() - now;
  const days = Math.round(d / 86_400_000);
  const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
  if (Math.abs(d) < 3_600_000) return rtf.format(Math.round(d / 60_000), "minute");
  if (Math.abs(d) < 86_400_000) return rtf.format(Math.round(d / 3_600_000), "hour");
  return rtf.format(days, "day");
}

export function openingLine(e: Exhibition, now = Date.now()) {
  if (!e.openingAt) return null;
  const at = new Date(e.openingAt);
  const when = at.toLocaleString("en", { weekday: "long", day: "numeric", month: "long", hour: "numeric", minute: "2-digit" });
  return at.getTime() > now ? `Opening night ${when}` : `Opened ${when}`;
}

/** Initials for an avatar. */
export function initials(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]!.toUpperCase())
    .join("");
}
