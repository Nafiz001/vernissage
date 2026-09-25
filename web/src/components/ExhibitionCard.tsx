"use client";

import Link from "next/link";
import { inkOn } from "@/lib/color";
import { count, openingLine } from "@/lib/format";
import { frameCss } from "@/lib/frames";
import type { Exhibition } from "@/lib/types";
import { blurhashURL } from "./Framed";

/**
 * An exhibition as a little wall in its own paint, its cover hung in the
 * middle under a light, the title lettered underneath.
 */
export function ExhibitionCard({ e, href }: { e: Exhibition; href?: string }) {
  const cover = e.preview[0];
  const { soft } = inkOn(e.paintHex);
  const blur = blurhashURL(cover?.blurhash);
  const aspect = cover?.aspect ?? 0.8;
  // Fit the cover in 56% x 62% of the wall, keeping its proportions.
  const w = aspect >= 0.9 ? 56 : 56 * (aspect / 0.9);
  const opening = openingLine(e);
  const gilt = frameCss("gilt", 7);

  return (
    <Link href={href ?? `/e/${e.slug}`} className="group block">
      <div
        className="plaster relative aspect-[4/3] overflow-hidden rounded-[3px] shadow-[0_20px_40px_-24px_rgba(0,0,0,.7)]"
        style={{ backgroundColor: e.paintHex }}
      >
        <div className="wash" style={{ width: "90%", height: "95%", left: "5%", top: "-8%" }} />
        {cover ? (
          <div
            className="absolute left-1/2 top-[44%] -translate-x-1/2 -translate-y-1/2 transition-transform duration-500 ease-[var(--ease-museum)] group-hover:-translate-y-[52%]"
            style={{ width: `${w}%`, aspectRatio: aspect }}
          >
            <div className="h-full w-full" style={gilt.outer}>
              <div
                className="relative h-full w-full overflow-hidden bg-cover"
                style={{ backgroundColor: cover.dominant, backgroundImage: blur ? `url(${blur})` : undefined, ...gilt.sight }}
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={`/img/${cover.id}/800.jpg`} alt="" loading="lazy" className="absolute inset-0 h-full w-full object-cover" />
              </div>
            </div>
          </div>
        ) : (
          <p className="absolute inset-0 grid place-items-center text-[14px]" style={{ color: soft }}>
            Nothing hung yet
          </p>
        )}
        {e.inside > 0 && (
          <span
            className="absolute left-3 top-3 flex items-center gap-2 rounded-full bg-black/35 px-2.5 py-1 text-[12.5px] text-white backdrop-blur"
          >
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-300 opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-300" />
            </span>
            {e.inside} inside now
          </span>
        )}
        {e.status === "draft" && (
          <span className="absolute right-3 top-3 rounded-full bg-black/35 px-2.5 py-1 text-[12.5px] text-white backdrop-blur">Draft</span>
        )}
        <p className="absolute inset-x-0 bottom-0 px-4 pb-3 text-[13px]" style={{ color: soft }}>
          {count(e.workCount, "work", "works")}
        </p>
      </div>
      <h3 className="lettering-sm mt-4 text-[26px] group-hover:underline group-hover:decoration-1 group-hover:underline-offset-4">{e.title}</h3>
      <p className="mt-1 text-[15px] text-[var(--soft)]">Curated by {e.owner.name}</p>
      {opening && <p className="mt-0.5 text-[14px] text-[var(--soft)]">{opening}</p>}
    </Link>
  );
}
