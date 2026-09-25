"use client";

import Link from "next/link";
import { useEffect, useRef, useState, type ReactNode } from "react";
import type { Artwork } from "@/lib/types";
import { Framed } from "./Framed";

export interface SalonRow {
  height: number;
  items: { work: Artwork; width: number }[];
}

/**
 * Lays works out in justified rows, the way a nineteenth-century Salon hung
 * them frame to frame: every row fills the wall's width, and rows alternate
 * between taller and shorter so the wall doesn't read as a grid.
 */
export function salonRows(works: Artwork[], wallWidth: number, targets: number[], gap: number): SalonRow[] {
  const rows: SalonRow[] = [];
  let i = 0;
  let r = 0;
  while (i < works.length) {
    const target = targets[r % targets.length]!;
    const row: Artwork[] = [];
    let ratio = 0;
    while (i < works.length) {
      const w = works[i]!;
      row.push(w);
      ratio += w.hang.w / w.hang.h;
      i++;
      if (ratio * target + gap * (row.length - 1) >= wallWidth) break;
    }
    const full = ratio * target + gap * (row.length - 1) >= wallWidth;
    const height = full ? (wallWidth - gap * (row.length - 1)) / ratio : target;
    rows.push({ height, items: row.map((w) => ({ work: w, width: (w.hang.w / w.hang.h) * height })) });
    r++;
  }
  return rows;
}

export function useWidth<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(0);
  useEffect(() => {
    if (!ref.current) return;
    const ro = new ResizeObserver(([e]) => setWidth(e!.contentRect.width));
    ro.observe(ref.current);
    return () => ro.disconnect();
  }, []);
  return [ref, width] as const;
}

/** A small salon hang for the front page. */
export function SalonCluster({ works }: { works: Artwork[] }) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const mobile = width < 640;
  const gap = mobile ? 14 : 22;
  const rows = width ? salonRows(works, width, mobile ? [110, 90] : [190, 150, 170], gap) : [];
  return (
    <div ref={ref} className="flex flex-col" style={{ gap: gap * 1.3 }}>
      {rows.map((row, r) => (
        <div key={r} className="flex items-end justify-center" style={{ gap }}>
          {row.items.map(({ work, width }) => (
            <Link key={work.id} href={`/artwork/${work.id}`} className="block transition-transform duration-300 hover:-translate-y-1" title={`${work.title}${work.artist ? `, ${work.artist}` : ""}`}>
              <Framed artwork={work} width={width} maxSrc={800} />
            </Link>
          ))}
        </div>
      ))}
    </div>
  );
}

export function Skirting({ children }: { children?: ReactNode }) {
  return (
    <div
      aria-hidden
      className="absolute inset-x-0 bottom-0 h-3 bg-[color-mix(in_oklab,var(--wall)_78%,black)] shadow-[inset_0_1px_0_rgba(255,255,255,.12)]"
    >
      {children}
    </div>
  );
}
