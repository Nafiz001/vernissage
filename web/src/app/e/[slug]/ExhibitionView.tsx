"use client";

import { useMutation } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Framed } from "@/components/Framed";
import { useWidth } from "@/components/Salon";
import { SiteFooter } from "@/components/SiteFooter";
import { SiteHeader } from "@/components/SiteHeader";
import { WallLabel } from "@/components/WallLabel";
import { api } from "@/lib/api";
import { inkOn, isDark } from "@/lib/color";
import { count, openingLine, relativeDays, WALL_NAMES } from "@/lib/format";
import { useNow } from "@/lib/hooks";
import type { Artwork, ExhibitionDetail } from "@/lib/types";

export function ExhibitionView({ detail }: { detail: ExhibitionDetail }) {
  const e = detail.exhibition;
  const tone = inkOn(e.paintHex);
  const works = new Map(detail.works.map((w) => [w.id, w]));
  const cover = (e.coverId && works.get(e.coverId)) || detail.works[0];
  const [applause, setApplause] = useState({ n: e.applause, on: detail.applauded });
  const [copied, setCopied] = useState(false);
  const now = useNow();

  useEffect(() => {
    if (e.status === "published") api(`/api/exhibitions/${e.slug}/visit`, { method: "POST" }).catch(() => {});
  }, [e.slug, e.status]);

  const applaud = useMutation({
    mutationFn: () => api<{ applause: number; applauded: boolean }>(`/api/exhibitions/${e.slug}/applause`, { method: "POST" }),
    onMutate: () => setApplause((a) => ({ n: a.n + (a.on ? -1 : 1), on: !a.on })),
    onSuccess: (r) => setApplause({ n: r.applause, on: r.applauded }),
  });

  // The walking order: round the room clockwise from the far wall.
  const order = [0, 1, 2, 3].flatMap((wall) =>
    detail.placements
      .filter((p) => p.wall === wall)
      .sort((a, b) => a.x - b.x)
      .map((p) => ({ p, a: works.get(p.artworkId) }))
      .filter((x): x is { p: typeof x.p; a: Artwork } => !!x.a),
  );
  const opening = openingLine(e);
  const upcoming = now !== null && e.openingAt && new Date(e.openingAt).getTime() > now;

  return (
    <main
      className={`room plaster min-h-svh ${isDark(e.paintHex) ? "room-dark" : "room-light"}`}
      style={{ ["--wall" as string]: e.paintHex, ["--ink" as string]: tone.ink, ["--soft" as string]: tone.soft, ["--line" as string]: tone.line }}
    >
      <SiteHeader />
      <section className="mx-auto grid max-w-[1440px] items-center gap-14 px-5 pb-24 pt-10 sm:px-8 lg:min-h-[78svh] lg:grid-cols-[minmax(0,6fr)_minmax(0,5fr)]">
        <div>
          {e.status === "draft" && (
            <p className="mb-6 inline-block rounded-full border border-[var(--line)] px-3 py-1 text-[14px]">
              A draft only you can see
            </p>
          )}
          <h1 className="lettering text-[clamp(52px,7vw,112px)]">{e.title}</h1>
          <p className="mt-6 text-[18px]">
            Curated by{" "}
            <Link href={`/u/${e.owner.handle}`} className="underline underline-offset-4">
              {e.owner.name}
            </Link>
          </p>
          {opening && (
            <p className="mt-1 text-[17px] text-[var(--soft)]">
              {opening}
              {upcoming && e.openingAt && `, ${relativeDays(e.openingAt, now!)}`}
            </p>
          )}
          {e.statement && <p className="prose-serif mt-8 max-w-[58ch] whitespace-pre-line text-[var(--ink)]">{e.statement}</p>}

          <div className="mt-10 flex flex-wrap items-center gap-3">
            <Link href={`/e/${e.slug}/walk`} className="btn btn-solid h-14 px-8 text-[17px]">
              Walk in
            </Link>
            {e.status === "published" && (
              <button
                className="btn btn-line h-14 px-6"
                aria-pressed={applause.on}
                onClick={() => applaud.mutate()}
              >
                <span aria-hidden>
                  👏
                </span>
                {applause.on ? "Applauded" : "Applaud"}
                <span className="text-[var(--soft)]">{applause.n}</span>
              </button>
            )}
            <button
              className="btn btn-quiet h-14"
              onClick={() => {
                navigator.clipboard?.writeText(window.location.href.replace(/\/walk$/, ""));
                setCopied(true);
                setTimeout(() => setCopied(false), 2000);
              }}
            >
              {copied ? "Link copied" : "Copy link"}
            </button>
            {e.mine && (
              <Link href={`/studio/${e.id}`} className="btn btn-quiet h-14">
                Edit in the studio
              </Link>
            )}
          </div>
          <p className="mt-8 flex flex-wrap gap-x-6 gap-y-1 text-[15px] text-[var(--soft)]">
            <span>{count(e.workCount, "work", "works")}</span>
            <span>{count(e.visits, "visit", "visits")}</span>
            {e.inside > 0 && <span className="text-[var(--ink)]">{count(e.inside, "person", "people")} inside now</span>}
          </p>
        </div>
        {cover && <CoverWall cover={cover} />}
      </section>

      {order.length > 0 && (
        <section className="border-t border-[var(--line)]" aria-labelledby="checklist">
          <div className="mx-auto max-w-[1440px] px-5 py-20 sm:px-8">
            <h2 id="checklist" className="lettering text-[48px]">
              Checklist
            </h2>
            <p className="mt-2 text-[var(--soft)]">In the order you meet them, clockwise from the far wall.</p>
            <ol className="mt-12 grid gap-x-12 gap-y-14 md:grid-cols-2 xl:grid-cols-3">
              {order.map(({ p, a }, i) => (
                <li key={a.id} className="grid grid-cols-[auto_1fr] gap-5">
                  <span className="lettering-sm w-7 pt-1 text-[22px] text-[var(--soft)]">{i + 1}</span>
                  <div>
                    <Link href={`/artwork/${a.id}`} className="inline-block transition-transform hover:-translate-y-0.5">
                      <Framed artwork={a} width={Math.min(260, 170 * Math.sqrt(a.hang.w / a.hang.h))} maxSrc={800} />
                    </Link>
                    <p className="mt-2 text-[13px] text-[var(--soft)]">{WALL_NAMES[p.wall]}</p>
                    <WallLabel artwork={a} note={p.label || undefined} credit={false} className="mt-3 max-w-[340px]" />
                  </div>
                </li>
              ))}
            </ol>
          </div>
        </section>
      )}
      <SiteFooter />
    </main>
  );
}

function CoverWall({ cover }: { cover: Artwork }) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const boxH = 520;
  const s = Math.min((width || 400) / cover.hang.w, boxH / cover.hang.h) * 0.9;
  return (
    <div ref={ref} className="relative flex h-[520px] items-center justify-center">
      <div className="wash" style={{ width: cover.hang.w * s * 2, height: cover.hang.h * s * 2, left: "50%", top: "50%", transform: "translate(-50%,-58%)" }} />
      <Link href={`/artwork/${cover.id}`} className="relative z-10">
        <Framed artwork={cover} width={cover.hang.w * s} maxSrc={1600} priority />
      </Link>
    </div>
  );
}
