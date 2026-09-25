// OKLab, the same perceptual colour space the server analyses in.

type Lab = { L: number; a: number; b: number };

const toLinear = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
const toSrgb = (c: number) => (c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055);

export function hexToLab(hex: string): Lab {
  const n = parseInt(hex.replace("#", ""), 16);
  const r = toLinear(((n >> 16) & 255) / 255);
  const g = toLinear(((n >> 8) & 255) / 255);
  const b = toLinear((n & 255) / 255);
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return {
    L: 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    a: 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    b: 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  };
}

export function labToHex({ L, a, b }: Lab): string {
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  const ch = [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ].map((c) => Math.round(Math.min(1, Math.max(0, toSrgb(c))) * 255));
  return "#" + ch.map((c) => c.toString(16).padStart(2, "0")).join("");
}

export function isDark(hex: string) {
  return hexToLab(hex).L < 0.62;
}

/**
 * A wall to hang a work on, the way a curator picks paint: a deep, quiet
 * shade of the work's own most telling colour, so the picture is the
 * brightest, most saturated thing in view.
 */
export function wallFor(palette: { hex: string; w: number }[], fallback = "#3b1c1d") {
  if (!palette.length) return fallback;
  let best = palette[0]!;
  let bestScore = -1;
  for (const s of palette) {
    const c = hexToLab(s.hex);
    const score = Math.hypot(c.a, c.b) * Math.sqrt(s.w);
    if (score > bestScore) {
      best = s;
      bestScore = score;
    }
  }
  const c = hexToLab(best.hex);
  const chroma = Math.hypot(c.a, c.b);
  const keep = chroma > 0 ? Math.min(0.055, chroma * 0.55) / chroma : 0;
  return labToHex({ L: 0.27, a: c.a * keep, b: c.b * keep });
}

/** Text colours that read on a wall. */
export function inkOn(wall: string) {
  return isDark(wall)
    ? { ink: "#f2ede3", soft: "rgba(242, 237, 227, 0.68)", line: "rgba(242, 237, 227, 0.18)" }
    : { ink: "#1d1a16", soft: "rgba(29, 26, 22, 0.64)", line: "rgba(29, 26, 22, 0.16)" };
}
