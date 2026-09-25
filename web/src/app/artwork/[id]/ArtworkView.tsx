"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { motion } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { DeepZoom } from "@/components/DeepZoom";
import { ExhibitionCard } from "@/components/ExhibitionCard";
import { Framed } from "@/components/Framed";
import { useWidth } from "@/components/Salon";
import { SiteHeader } from "@/components/SiteHeader";
import { WallLabel } from "@/components/WallLabel";
import { api } from "@/lib/api";
import { inkOn, isDark, wallFor } from "@/lib/color";
import { kindName } from "@/lib/format";
import { useViewportHeight } from "@/lib/hooks";
import { useMe, useSavedSet, useToggleSaved, useUI } from "@/lib/session";
import type { Artwork, ArtworkDetail, Exhibition, ExhibitionDetail } from "@/lib/types";

export function ArtworkView({ detail }: { detail: ArtworkDetail }) {
  const a = detail.artwork;
  const wall = wallFor(a.palette);
  const tone = inkOn(wall);
  const [zoom, setZoom] = useState(false);
  const [realSize, setRealSize] = useState(false);
  const saved = useSavedSet();
  const toggle = useToggleSaved();
  const { data: similar } = useQuery({
    queryKey: ["similar", a.id],
    queryFn: () => api<{ items: Artwork[] }>(`/api/artworks/${a.id}/similar`),
  });

  return (
    <main
      className={`room plaster min-h-svh ${isDark(wall) ? "room-dark" : "room-light"}`}
      style={{ ["--wall" as string]: wall, ["--ink" as string]: tone.ink, ["--soft" as string]: tone.soft, ["--line" as string]: tone.line }}
    >
      <SiteHeader />
      <article className="mx-auto grid max-w-[1440px] gap-12 px-5 pb-24 pt-6 sm:px-8 lg:grid-cols-[minmax(0,7fr)_minmax(0,4fr)] lg:gap-16">
        <div className="lg:sticky lg:top-6 lg:self-start">
          <Stage artwork={a} realSize={realSize} />
          <div className="mt-8 flex flex-wrap items-center justify-center gap-2">
            <button className="btn btn-solid" onClick={() => setZoom(true)}>
              Look closer
            </button>
            <button className="btn btn-line" aria-pressed={realSize} onClick={() => setRealSize((r) => !r)}>
              {realSize ? "Back to the wall" : "See it at real size"}
            </button>
            <button className="btn btn-line" aria-pressed={saved.has(a.id)} onClick={() => toggle(a.id, !saved.has(a.id))}>
              {saved.has(a.id) ? "Saved to your selection" : "Save"}
            </button>
            <AddToExhibition artwork={a} />
          </div>
        </div>

        <div>
          <WallLabel artwork={a} />
          {a.description && (
            <div className="prose-serif mt-8 max-w-[62ch] space-y-4 whitespace-pre-line">{a.description}</div>
          )}
          {a.palette.length > 0 && (
            <section className="mt-10" aria-labelledby="palette-title">
              <h2 id="palette-title" className="text-[15px] font-medium">
                The colours it&apos;s made of
              </h2>
              <div className="mt-3 flex h-10 overflow-hidden rounded-[3px] shadow-[inset_0_0_0_1px_rgba(0,0,0,.15)]">
                {a.palette.map((s) => (
                  <Link
                    key={s.hex}
                    href={`/collection?color=${encodeURIComponent(s.hex)}`}
                    title={`${Math.round(s.w * 100)}% of the picture. Find works with this colour.`}
                    className="transition-[flex-grow] duration-300 hover:grow-[2]"
                    style={{ background: s.hex, flexGrow: s.w }}
                  >
                    <span className="sr-only">
                      {s.hex}, {Math.round(s.w * 100)}%
                    </span>
                  </Link>
                ))}
              </div>
              <p className="mt-2 text-[13px] text-[var(--soft)]">Choose a colour to find other works that are full of it.</p>
            </section>
          )}
          <p className="mt-10 text-[15px]">
            <a href={a.museum.url} className="underline underline-offset-4" target="_blank" rel="noreferrer">
              See it on {a.museum.key === "met" ? "The Met's" : "the Cleveland Museum of Art's"} website
            </a>
          </p>
        </div>
      </article>

      {detail.moreByArtist.length > 0 && (
        <Row title={`More by ${a.artist}`} works={detail.moreByArtist} more={`/collection?artist=${encodeURIComponent(a.artist)}`} />
      )}
      {similar && similar.items.length > 0 && <Row title="In similar colours" works={similar.items} />}
      {detail.exhibitions.length > 0 && (
        <section className="mx-auto max-w-[1440px] px-5 pb-24 sm:px-8">
          <h2 className="lettering-sm text-[34px]">Hanging in</h2>
          <div className="mt-8 grid gap-10 sm:grid-cols-2 lg:grid-cols-3">
            {detail.exhibitions.map((e) => (
              <ExhibitionCard key={e.id} e={e} />
            ))}
          </div>
        </section>
      )}
      <DeepZoom id={a.id} title={a.title} open={zoom} onClose={() => setZoom(false)} />
    </main>
  );
}

/**
 * The work on the wall, or, at real size, beside a person 1.7 m tall and a
 * gallery bench, so a tiny print and a huge canvas look it.
 */
function Stage({ artwork: a, realSize }: { artwork: Artwork; realSize: boolean }) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const vh = useViewportHeight();
  const boxH = Math.min(vh * 0.66, 720);
  const boxW = Math.max(200, width);

  // On the wall: as large as fits.
  const wallScale = Math.min(boxW / a.hang.w, boxH / a.hang.h);
  // At real size: the scene is the work plus a person, in metres.
  const sceneW = a.hang.w + 1.4;
  const sceneH = Math.max(1.9, 1.48 + a.hang.h / 2 + 0.25);
  const realScale = Math.min(boxW / sceneW, boxH / sceneH);
  const scale = realSize ? realScale : wallScale;
  const fw = a.hang.w * scale;

  return (
    <div ref={ref} className="relative flex items-end justify-center" style={{ height: boxH }}>
      <div className="wash" style={{ width: fw * 1.9, height: a.hang.h * scale * 1.9, left: "50%", top: 0, transform: "translate(-50%, -18%)" }} />
      <motion.div
        layout
        transition={{ duration: 0.8, ease: [0.65, 0, 0.35, 1] }}
        className="relative z-10 flex items-end gap-[3%]"
        style={{ marginBottom: realSize ? 0 : (boxH - a.hang.h * scale) / 2 }}
      >
        <motion.div layout transition={{ duration: 0.8, ease: [0.65, 0, 0.35, 1] }} style={{ marginBottom: realSize ? (1.48 - a.hang.h / 2) * scale : 0 }}>
          <Framed artwork={a} width={fw} maxSrc={realSize ? 1200 : 2400} priority />
        </motion.div>
        {realSize && (
          <motion.svg
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            viewBox="0 0 40 170"
            style={{ height: 1.7 * scale, width: 0.4 * scale }}
            className="text-[var(--ink)] opacity-70"
            aria-label="A person 1.7 metres tall, for scale"
          >
            <circle cx="20" cy="11" r="10" fill="currentColor" />
            <path d="M8 26h24l4 58h-6l-2 86h-9l-1-55-1 55h-9l-2-86H4z" fill="currentColor" />
          </motion.svg>
        )}
      </motion.div>
      {realSize && (
        <>
          <div className="absolute inset-x-0 bottom-0 h-px bg-[var(--ink)] opacity-40" />
          <p className="absolute -bottom-7 left-0 text-[13px] text-[var(--soft)]">
            {a.hang.estimated
              ? `The museum doesn't give this ${kindName(a.kind).toLowerCase()}'s size; this is a guess.`
              : `Framed, ${Math.round(a.hang.w * 100)} × ${Math.round(a.hang.h * 100)} cm, beside a person 1.7 m tall.`}
          </p>
        </>
      )}
    </div>
  );
}

function Row({ title, works, more }: { title: string; works: Artwork[]; more?: string }) {
  return (
    <section className="mx-auto max-w-[1440px] px-5 pb-20 sm:px-8">
      <div className="flex items-baseline justify-between gap-4">
        <h2 className="lettering-sm text-[34px]">{title}</h2>
        {more && (
          <Link href={more} className="text-[15px] underline underline-offset-4">
            See all
          </Link>
        )}
      </div>
      <div className="-mx-2 mt-8 flex items-end gap-6 overflow-x-auto px-2 pb-6 pt-2">
        {works.map((w) => (
          <Link key={w.id} href={`/artwork/${w.id}`} className="shrink-0 transition-transform duration-300 hover:-translate-y-1" title={w.title}>
            <Framed artwork={w} width={Math.max(100, Math.min(300, 200 * (w.hang.w / w.hang.h)))} maxSrc={800} />
          </Link>
        ))}
      </div>
    </section>
  );
}

/** Put this work in one of your exhibitions, or start one with it. */
function AddToExhibition({ artwork }: { artwork: Artwork }) {
  const { data: me } = useMe();
  const ask = useUI((s) => s.askSignIn);
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  const { data } = useQuery({
    queryKey: ["mine"],
    queryFn: () => api<{ items: Exhibition[] }>("/api/me/exhibitions"),
    enabled: open && !!me?.user,
  });
  const create = useMutation({
    mutationFn: () =>
      api<ExhibitionDetail>("/api/exhibitions", { method: "POST", json: { title: "Untitled exhibition", artworkIds: [artwork.id] } }),
    onSuccess: (d) => router.push(`/studio/${d.exhibition.id}`),
  });
  useEffect(() => {
    const close = (e: PointerEvent) => !box.current?.contains(e.target as Node) && setOpen(false);
    window.addEventListener("pointerdown", close);
    return () => window.removeEventListener("pointerdown", close);
  }, []);
  return (
    <div ref={box} className="relative">
      <button
        className="btn btn-line"
        aria-expanded={open}
        onClick={() => (me?.user ? setOpen((o) => !o) : ask("Sign in to hang this work in an exhibition."))}
      >
        Hang it
      </button>
      {open && (
        <div className="bg-card text-ink absolute bottom-[calc(100%+8px)] left-1/2 z-40 w-72 -translate-x-1/2 overflow-hidden rounded-xl py-1.5 shadow-2xl">
          <button className="block w-full px-4 py-2.5 text-left font-medium hover:bg-black/5" onClick={() => create.mutate()} disabled={create.isPending}>
            {create.isPending ? "Starting…" : "In a new exhibition"}
          </button>
          {data?.items.map((e) => (
            <Link key={e.id} href={`/studio/${e.id}?add=${artwork.id}`} className="block truncate px-4 py-2.5 hover:bg-black/5">
              In <span className="italic">{e.title}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
