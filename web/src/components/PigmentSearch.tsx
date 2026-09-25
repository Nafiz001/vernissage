"use client";

import { useQuery } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { isDark } from "@/lib/color";
import type { ArtworkPage } from "@/lib/types";
import { Framed } from "./Framed";

/** Pigments a painter would have had, as they look once on the canvas. */
export const PIGMENTS = [
  { name: "Vermilion", hex: "#c8452c" },
  { name: "Madder rose", hex: "#b24a64" },
  { name: "Burnt sienna", hex: "#8d4a2a" },
  { name: "Yellow ochre", hex: "#c39434" },
  { name: "Gamboge", hex: "#dcab2e" },
  { name: "Terre verte", hex: "#6f8061" },
  { name: "Viridian", hex: "#2f6f5a" },
  { name: "Prussian blue", hex: "#1f3c5a" },
  { name: "Ultramarine", hex: "#2f4d7a" },
  { name: "Cobalt", hex: "#4a6a9a" },
  { name: "Lead white", hex: "#e9e2d0" },
  { name: "Bone black", hex: "#26221f" },
];

/** A fan of paint chips; choosing one asks the colour index what's full of it. */
export function PigmentSearch() {
  const [pick, setPick] = useState(PIGMENTS[0]!);
  const { data, isFetching } = useQuery({
    queryKey: ["pigment", pick.hex],
    queryFn: () => api<ArtworkPage>(`/api/artworks?color=${encodeURIComponent(pick.hex)}&limit=7`),
    placeholderData: (prev) => prev,
  });

  return (
    <div>
      <div role="radiogroup" aria-label="Pigment" className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-3 pt-4 sm:gap-3">
        {PIGMENTS.map((p) => {
          const on = p.hex === pick.hex;
          return (
            <button
              key={p.hex}
              role="radio"
              aria-checked={on}
              onClick={() => setPick(p)}
              className="w-[92px] shrink-0 overflow-hidden rounded-[4px] bg-white text-left shadow-[0_1px_2px_rgba(0,0,0,.2),0_8px_16px_-10px_rgba(0,0,0,.4)] transition-transform duration-300 ease-[var(--ease-museum)] aria-checked:-translate-y-3 aria-checked:shadow-[0_2px_4px_rgba(0,0,0,.2),0_18px_28px_-12px_rgba(0,0,0,.5)]"
            >
              <span className="block h-[86px]" style={{ background: p.hex }} />
              <span className="text-ink block px-2 py-2 text-[12.5px] font-medium leading-tight">{p.name}</span>
            </button>
          );
        })}
      </div>

      <div className="mt-10 flex flex-wrap items-baseline gap-x-4 gap-y-2">
        <p className="lettering-sm text-[30px]" aria-live="polite">
          {data ? (
            <>
              {data.total.toLocaleString("en")} works are full of {pick.name.toLowerCase()}.
            </>
          ) : (
            <>Looking…</>
          )}
        </p>
        <Link href={`/collection?color=${encodeURIComponent(pick.hex)}`} className="text-[15px] underline underline-offset-4">
          See them all
        </Link>
      </div>

      <div className={`mt-8 flex min-h-[220px] items-end gap-5 overflow-x-auto pb-4 transition-opacity ${isFetching ? "opacity-60" : ""}`}>
        <AnimatePresence mode="popLayout" initial={false}>
          {data?.items.map((a, i) => (
            <motion.div
              key={`${pick.hex}-${a.id}`}
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.45, delay: i * 0.05, ease: [0.65, 0, 0.35, 1] }}
              className="shrink-0"
            >
              <Link href={`/artwork/${a.id}`} className="block">
                <Framed artwork={a} width={Math.max(110, Math.min(260, 190 * (a.hang.w / a.hang.h)))} maxSrc={800} />
              </Link>
              <div className="mt-3 flex h-2 w-full overflow-hidden rounded-full" aria-hidden>
                {a.palette.map((s) => (
                  <span key={s.hex} style={{ background: s.hex, flexGrow: s.w, outline: isDark(s.hex) ? undefined : "1px solid rgba(0,0,0,.06)" }} />
                ))}
              </div>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </div>
  );
}
