import * as THREE from "three";

// Everything in the room is drawn at load time on canvases: no texture
// files to download, and every floor is as large as it needs to be.

function canvas(size: number) {
  const c = document.createElement("canvas");
  c.width = c.height = size;
  return [c, c.getContext("2d")!] as const;
}

/** A small deterministic random generator, so the parquet is the same every visit. */
function rng(seed: number) {
  let s = seed >>> 0;
  return () => {
    s = (s + 0x6d2b79f5) >>> 0;
    let t = s;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function finish(c: HTMLCanvasElement, repeat: number, srgb = true) {
  const t = new THREE.CanvasTexture(c);
  t.wrapS = t.wrapT = THREE.RepeatWrapping;
  t.repeat.set(repeat, repeat);
  t.anisotropy = 8;
  if (srgb) t.colorSpace = THREE.SRGBColorSpace;
  return t;
}

function grain(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, base: [number, number, number], r: () => number, along: "x" | "y") {
  const shade = 0.82 + r() * 0.3;
  ctx.fillStyle = `rgb(${base[0] * shade}, ${base[1] * shade}, ${base[2] * shade})`;
  ctx.fillRect(x, y, w, h);
  const lines = 14 + Math.floor(r() * 10);
  for (let i = 0; i < lines; i++) {
    const a = 0.04 + r() * 0.08;
    ctx.strokeStyle = r() < 0.5 ? `rgba(40,22,10,${a})` : `rgba(255,235,200,${a * 0.6})`;
    ctx.lineWidth = 0.6 + r() * 1.4;
    ctx.beginPath();
    if (along === "x") {
      const yy = y + r() * h;
      ctx.moveTo(x, yy);
      ctx.bezierCurveTo(x + w * 0.3, yy + (r() - 0.5) * 4, x + w * 0.7, yy + (r() - 0.5) * 4, x + w, yy + (r() - 0.5) * 3);
    } else {
      const xx = x + r() * w;
      ctx.moveTo(xx, y);
      ctx.bezierCurveTo(xx + (r() - 0.5) * 4, y + h * 0.3, xx + (r() - 0.5) * 4, y + h * 0.7, xx + (r() - 0.5) * 3, y + h);
    }
    ctx.stroke();
  }
  ctx.strokeStyle = "rgba(20,10,4,0.55)";
  ctx.lineWidth = 1.2;
  ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1);
}

/** Oak parquet: half-metre squares of four blocks, each square turned against its neighbours. */
function parquet(): HTMLCanvasElement {
  const size = 1024;
  const [c, ctx] = canvas(size);
  const r = rng(7);
  const sq = 256;
  const plank = sq / 4;
  for (let i = 0; i < size / sq; i++) {
    for (let j = 0; j < size / sq; j++) {
      const across = (i + j) % 2 === 0;
      for (let k = 0; k < 4; k++) {
        if (across) grain(ctx, i * sq, j * sq + k * plank, sq, plank, [170, 126, 84], r, "x");
        else grain(ctx, i * sq + k * plank, j * sq, plank, sq, [164, 120, 80], r, "y");
      }
    }
  }
  return c;
}

function boards(): HTMLCanvasElement {
  const size = 1024;
  const [c, ctx] = canvas(size);
  const r = rng(11);
  const w = 128;
  for (let i = 0; i < size / w; i++) {
    let y = -r() * 600;
    while (y < size) {
      const len = 500 + r() * 700;
      grain(ctx, i * w, y, w, len, [98, 64, 40], r, "y");
      y += len;
    }
  }
  return c;
}

function noiseCanvas(size: number, base: string, specks: number, alpha: number, seed: number) {
  const [c, ctx] = canvas(size);
  const r = rng(seed);
  ctx.fillStyle = base;
  ctx.fillRect(0, 0, size, size);
  for (let i = 0; i < specks; i++) {
    const v = r() < 0.5 ? 0 : 255;
    ctx.fillStyle = `rgba(${v},${v},${v},${r() * alpha})`;
    const s = 1 + r() * 3;
    ctx.fillRect(r() * size, r() * size, s, s);
  }
  return [c, ctx, r] as const;
}

function concrete(): HTMLCanvasElement {
  const [c, ctx, r] = noiseCanvas(1024, "#6f6b66", 60000, 0.09, 3);
  for (let i = 0; i < 40; i++) {
    const g = ctx.createRadialGradient(r() * 1024, r() * 1024, 0, r() * 1024, r() * 1024, 80 + r() * 220);
    g.addColorStop(0, `rgba(${r() < 0.5 ? "0,0,0" : "255,255,255"},${0.03 + r() * 0.04})`);
    g.addColorStop(1, "rgba(0,0,0,0)");
    ctx.fillStyle = g;
    ctx.fillRect(0, 0, 1024, 1024);
  }
  // Saw-cut joints every 2 m.
  ctx.strokeStyle = "rgba(40,38,35,.5)";
  ctx.lineWidth = 2;
  ctx.strokeRect(0, 0, 1024, 1024);
  return c;
}

function marble(): HTMLCanvasElement {
  const [c, ctx, r] = noiseCanvas(1024, "#e4e0d8", 20000, 0.05, 5);
  for (let i = 0; i < 26; i++) {
    ctx.strokeStyle = `rgba(90,86,80,${0.05 + r() * 0.12})`;
    ctx.lineWidth = 0.6 + r() * 2.2;
    ctx.beginPath();
    let x = r() * 1024;
    let y = 0;
    ctx.moveTo(x, y);
    while (y < 1024) {
      x += (r() - 0.5) * 60;
      y += 20 + r() * 40;
      ctx.lineTo(x, y);
    }
    ctx.stroke();
  }
  // Tile joints, 1 m slabs.
  ctx.strokeStyle = "rgba(120,114,106,.45)";
  ctx.lineWidth = 2;
  ctx.strokeRect(0, 0, 512, 512);
  ctx.strokeRect(512, 0, 512, 512);
  ctx.strokeRect(0, 512, 512, 512);
  ctx.strokeRect(512, 512, 512, 512);
  return c;
}

const cache = new Map<string, THREE.Texture>();

/** A floor texture and how many metres one tile covers. */
export function floorTexture(kind: string): { map: THREE.Texture; metres: number } {
  const metres = { oak: 2, walnut: 3.2, concrete: 2, marble: 2 }[kind] ?? 2;
  let t = cache.get("floor:" + kind);
  if (!t) {
    const c = kind === "walnut" ? boards() : kind === "concrete" ? concrete() : kind === "marble" ? marble() : parquet();
    t = finish(c, 1);
    cache.set("floor:" + kind, t);
  }
  return { map: t, metres };
}

/** Plaster: the faint unevenness of a painted wall, as a bump map. */
export function plasterBump(): THREE.Texture {
  let t = cache.get("plaster");
  if (!t) {
    const [c] = noiseCanvas(512, "#808080", 90000, 0.18, 9);
    t = finish(c, 1, false);
    cache.set("plaster", t);
  }
  return t;
}

/** The pool of light a spotlight throws on the wall: bright top-centre, soft edges. */
export function washTexture(): THREE.Texture {
  let t = cache.get("wash");
  if (!t) {
    const [c, ctx] = canvas(256);
    const g = ctx.createRadialGradient(128, 110, 0, 128, 128, 128);
    g.addColorStop(0, "rgba(255,236,208,1)");
    g.addColorStop(0.35, "rgba(255,230,196,.55)");
    g.addColorStop(0.7, "rgba(255,226,190,.12)");
    g.addColorStop(1, "rgba(255,226,190,0)");
    ctx.fillStyle = g;
    ctx.fillRect(0, 0, 256, 256);
    t = new THREE.CanvasTexture(c);
    t.colorSpace = THREE.SRGBColorSpace;
    cache.set("wash", t);
  }
  return t;
}

/** A soft round shadow for under people and benches. */
export function blobTexture(): THREE.Texture {
  let t = cache.get("blob");
  if (!t) {
    const [c, ctx] = canvas(128);
    const g = ctx.createRadialGradient(64, 64, 0, 64, 64, 64);
    g.addColorStop(0, "rgba(0,0,0,.5)");
    g.addColorStop(1, "rgba(0,0,0,0)");
    ctx.fillStyle = g;
    ctx.fillRect(0, 0, 128, 128);
    t = new THREE.CanvasTexture(c);
    cache.set("blob", t);
  }
  return t;
}
