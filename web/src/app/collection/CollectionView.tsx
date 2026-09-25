"use client";

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useEffectEvent, useRef, useState } from "react";
import { Framed } from "@/components/Framed";
import { PIGMENTS } from "@/components/PigmentSearch";
import { salonRows, useWidth } from "@/components/Salon";
import { SearchBox } from "@/components/SearchBox";
import { SiteHeader } from "@/components/SiteHeader";
import { api } from "@/lib/api";
import { inkOn } from "@/lib/color";
import { count } from "@/lib/format";
import { useSessionSeed, useStored } from "@/lib/hooks";
import { useSavedSet, useToggleSaved } from "@/lib/session";
import type { Artwork, ArtworkPage, Stats } from "@/lib/types";

const WALLS = [
  { key: "verdigris", name: "Gallery green", hex: "#2d4a40" },
  { key: "oxblood", name: "Salon red", hex: "#5b1d1f" },
  { key: "prussian", name: "Prussian blue", hex: "#1e2e4a" },
  { key: "chalk", name: "Chalk white", hex: "#e9e7e1" },
];

const KINDS = [
  { v: "", label: "Everything" },
  { v: "painting", label: "Paintings" },
  { v: "print", label: "Prints" },
  { v: "drawing", label: "Drawings" },
];

const ERAS = [
  { v: "", label: "Any date" },
  { v: "-1400", label: "Before 1400" },
  { v: "1400-1599", label: "1400 to 1600" },
  { v: "1600-1799", label: "1600 to 1800" },
  { v: "1800-1899", label: "1800s" },
  { v: "1900-", label: "1900 and after" },
];

const MOODS = [
  { v: "", label: "Any light" },
  { v: "bright", label: "Bright" },
  { v: "dark", label: "Dark" },
  { v: "vivid", label: "Vivid" },
  { v: "muted", label: "Muted" },
];

const MUSEUMS = [
  { v: "", label: "Both museums" },
  { v: "met", label: "The Met" },
  { v: "cma", label: "Cleveland" },
];

/** The wall colour a visitor chose last time. */
function useWall() {
  const [key, setKey] = useStored("local", "vernissage.wall");
  const wall = WALLS.find((w) => w.key === key) ?? WALLS[0]!;
  return [wall, (w: (typeof WALLS)[number]) => setKey(w.key)] as const;
}

export function CollectionView() {
  const params = useSearchParams();
  const router = useRouter();
  const path = usePathname();
  const [wall, setWall] = useWall();
  const seed = useSessionSeed();
  const colors = (params.get("color") ?? "").split(",").filter(Boolean);
  const f = {
    q: params.get("q") ?? "",
    artist: params.get("artist") ?? "",
    kind: params.get("kind") ?? "",
    era: params.get("era") ?? "",
    mood: params.get("mood") ?? "",
    museum: params.get("museum") ?? "",
  };

  function set(changes: Record<string, string | null>) {
    const next = new URLSearchParams(params.toString());
    for (const [k, v] of Object.entries(changes)) {
      if (v) next.set(k, v);
      else next.delete(k);
    }
    router.replace(`${path}${next.size ? `?${next}` : ""}`, { scroll: false });
  }

  const query = (() => {
    const p = new URLSearchParams();
    if (f.q) p.set("q", f.q);
    if (f.artist) p.set("artist", f.artist);
    if (f.kind) p.set("kind", f.kind);
    if (f.mood) p.set("mood", f.mood);
    if (f.museum) p.set("museum", f.museum);
    if (colors.length) p.set("color", colors.join(","));
    if (f.era) {
      const [from, to] = f.era.split("-");
      if (from) p.set("from", from);
      if (to) p.set("to", to);
    }
    p.set("seed", String(seed ?? 1));
    p.set("limit", "48");
    return p.toString();
  })();

  const { data: stats } = useQuery({ queryKey: ["stats"], queryFn: () => api<Stats>("/api/stats"), staleTime: 300_000 });
  const results = useInfiniteQuery({
    queryKey: ["artworks", query],
    queryFn: ({ pageParam }) => api<ArtworkPage>(`/api/artworks?${query}&offset=${pageParam}`),
    initialPageParam: 0,
    getNextPageParam: (last) => last.next,
    enabled: seed !== null,
  });
  const works = results.data?.pages.flatMap((p) => p.items) ?? [];
  const total = results.data?.pages[0]?.total;
  const fuzzy = results.data?.pages[0]?.fuzzy;

  const sentinel = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = sentinel.current;
    if (!el) return;
    const io = new IntersectionObserver(
      ([e]) => e?.isIntersecting && results.hasNextPage && !results.isFetchingNextPage && results.fetchNextPage(),
      { rootMargin: "1200px" },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [results]);

  const tone = inkOn(wall.hex);
  const light = wall.key === "chalk";
  const filtered = !!(f.q || f.artist || f.kind || f.era || f.mood || f.museum || colors.length);

  return (
    <main
      className={`room plaster min-h-svh transition-colors duration-700 ${light ? "room-light" : "room-dark"}`}
      style={{ ["--wall" as string]: wall.hex, ["--ink" as string]: tone.ink, ["--soft" as string]: tone.soft, ["--line" as string]: tone.line }}
    >
      <SiteHeader />
      <div className="mx-auto max-w-[1440px] px-5 pt-8 sm:px-8">
        <SearchBox
          value={f.q}
          onSearch={(q) => set({ q })}
          onArtist={(artist) => set({ artist, q: null })}
          placeholder={stats ? `Search ${stats.collection.works.toLocaleString("en")} works` : "Search the collection"}
        />

        <div className="mt-6 flex flex-wrap items-center gap-2" role="group" aria-label="Filters">
          <Select label="Kind" value={f.kind} options={KINDS} onChange={(kind) => set({ kind })} />
          <Select label="Date" value={f.era} options={ERAS} onChange={(era) => set({ era })} />
          <Select label="Light" value={f.mood} options={MOODS} onChange={(mood) => set({ mood })} />
          <Select label="Museum" value={f.museum} options={MUSEUMS} onChange={(museum) => set({ museum })} />
          <ColourPicker colors={colors} onChange={(c) => set({ color: c.join(",") || null })} />
          <div className="ml-auto flex items-center gap-2" role="radiogroup" aria-label="Wall colour">
            <span className="hidden text-[14px] text-[var(--soft)] md:inline">Wall</span>
            {WALLS.map((w) => (
              <button
                key={w.key}
                role="radio"
                aria-checked={w.key === wall.key}
                aria-label={w.name}
                title={w.name}
                onClick={() => setWall(w)}
                className="h-7 w-7 rounded-full border border-[var(--line)] transition-transform aria-checked:scale-110 aria-checked:ring-2 aria-checked:ring-[var(--ink)] aria-checked:ring-offset-2 aria-checked:ring-offset-[var(--wall)]"
                style={{ background: w.hex }}
              />
            ))}
          </div>
        </div>

        <div className="mt-6 flex min-h-8 flex-wrap items-center gap-2 text-[15px]">
          <span className="text-[var(--soft)]" aria-live="polite">
            {total === undefined ? "Looking…" : count(total, "work", "works")}
            {fuzzy && ` with names close to “${f.q}”`}
          </span>
          {f.artist && <Chip onClear={() => set({ artist: null })}>by {f.artist}</Chip>}
          {f.q && <Chip onClear={() => set({ q: null })}>“{f.q}”</Chip>}
          {colors.map((c) => (
            <Chip key={c} onClear={() => set({ color: colors.filter((x) => x !== c).join(",") || null })}>
              <span className="h-3.5 w-3.5 rounded-full border border-white/30" style={{ background: c }} />
              {PIGMENTS.find((p) => p.hex === c)?.name ?? c}
            </Chip>
          ))}
          {filtered && (
            <button className="ml-1 text-[var(--soft)] underline underline-offset-4 hover:text-[var(--ink)]" onClick={() => router.replace(path, { scroll: false })}>
              Clear all
            </button>
          )}
        </div>
      </div>

      <div className="mx-auto max-w-[1440px] px-5 pb-32 pt-10 sm:px-8">
        {results.isError ? (
          <p className="py-24 text-center text-[var(--soft)]">{(results.error as Error).message}</p>
        ) : total === 0 ? (
          <div className="py-24 text-center">
            <p className="lettering-sm text-[34px]">Nothing matches all of that.</p>
            <p className="mt-3 text-[var(--soft)]">Try fewer filters, a nearby colour, or a different spelling.</p>
            <button className="btn btn-line mt-6" onClick={() => router.replace(path, { scroll: false })}>
              Clear all filters
            </button>
          </div>
        ) : (
          <Wall works={works} />
        )}
        <div ref={sentinel} className="h-px" />
        {results.isFetchingNextPage && <p className="py-10 text-center text-[var(--soft)]">Hanging more…</p>}
      </div>
    </main>
  );
}

/** The whole result set as one salon hang. */
function Wall({ works }: { works: Artwork[] }) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const saved = useSavedSet();
  const toggle = useToggleSaved();
  const mobile = width < 640;
  const gap = mobile ? 16 : 28;
  const rows = width ? salonRows(works, width, mobile ? [150, 120] : [250, 205, 230], gap) : [];
  return (
    <div ref={ref} className="flex flex-col" style={{ gap: gap * 1.4 }}>
      {rows.map((row, r) => (
        <div key={r} className="flex items-end justify-center [content-visibility:auto]" style={{ gap, containIntrinsicSize: `auto ${row.height}px` }}>
          {row.items.map(({ work, width }) => (
            <figure key={work.id} className="group relative" style={{ width }}>
              <Link href={`/artwork/${work.id}`} className="block transition-transform duration-300 ease-[var(--ease-museum)] group-hover:-translate-y-1">
                <Framed artwork={work} width={width} maxSrc={800} />
              </Link>
              <figcaption className="pointer-events-none absolute inset-x-0 top-full z-10 mt-2 opacity-0 transition-opacity duration-200 group-focus-within:opacity-100 group-hover:opacity-100">
                <span className="bg-card text-ink block rounded-[2px] px-3 py-2 text-[13px] leading-snug shadow-lg">
                  <span className="block truncate italic">{work.title}</span>
                  <span className="block truncate text-black/60">
                    {[work.artist, work.date].filter(Boolean).join(", ")}
                  </span>
                </span>
              </figcaption>
              <button
                onClick={() => toggle(work.id, !saved.has(work.id))}
                aria-pressed={saved.has(work.id)}
                aria-label={saved.has(work.id) ? `Remove ${work.title} from your selection` : `Save ${work.title}`}
                className="bg-card text-ink absolute -right-2.5 -top-2.5 z-10 grid h-9 w-9 place-items-center rounded-full opacity-0 shadow-lg transition group-focus-within:opacity-100 group-hover:opacity-100 aria-pressed:bg-[var(--color-gilt)] aria-pressed:opacity-100"
              >
                <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden>
                  <path d="M4 2h8v12l-4-3-4 3z" fill={saved.has(work.id) ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
                </svg>
              </button>
            </figure>
          ))}
        </div>
      ))}
    </div>
  );
}

function Select({ label, value, options, onChange }: { label: string; value: string; options: { v: string; label: string }[]; onChange: (v: string) => void }) {
  return (
    <label className="relative">
      <span className="sr-only">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`btn btn-line cursor-pointer appearance-none bg-transparent pr-9 text-[15px] ${value ? "border-[var(--ink)]" : ""}`}
      >
        {options.map((o) => (
          <option key={o.v} value={o.v} className="text-ink bg-white">
            {o.label}
          </option>
        ))}
      </select>
      <span className="pointer-events-none absolute right-4 top-1/2 -translate-y-1/2 text-[11px]" aria-hidden>
        ▼
      </span>
    </label>
  );
}

function Chip({ children, onClear }: { children: React.ReactNode; onClear: () => void }) {
  return (
    <span className="inline-flex items-center gap-2 rounded-full border border-[var(--line)] py-1 pl-3 pr-1">
      {children}
      <button onClick={onClear} className="grid h-6 w-6 place-items-center rounded-full hover:bg-[var(--line)]" aria-label="Remove filter">
        ×
      </button>
    </span>
  );
}

/** Paint chips in a popover, plus any colour at all; up to three at once. */
function ColourPicker({ colors, onChange }: { colors: string[]; onChange: (c: string[]) => void }) {
  const [open, setOpen] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const close = (e: PointerEvent) => !box.current?.contains(e.target as Node) && setOpen(false);
    window.addEventListener("pointerdown", close);
    return () => window.removeEventListener("pointerdown", close);
  }, []);
  const add = (hex: string) => {
    if (colors.includes(hex)) onChange(colors.filter((c) => c !== hex));
    else onChange([...colors, hex].slice(-3));
  };
  // The native change event fires once, when the picker closes; React's
  // onChange would fire for every colour dragged through.
  const custom = useRef<HTMLInputElement>(null);
  const pick = useEffectEvent((hex: string) => add(hex));
  useEffect(() => {
    const el = custom.current;
    if (!el) return;
    const onPick = () => pick(el.value);
    el.addEventListener("change", onPick);
    return () => el.removeEventListener("change", onPick);
  }, [open]);
  return (
    <div ref={box} className="relative">
      <button className={`btn btn-line text-[15px] ${colors.length ? "border-[var(--ink)]" : ""}`} aria-expanded={open} onClick={() => setOpen((o) => !o)}>
        <span className="flex -space-x-1.5" aria-hidden>
          {(colors.length ? colors : ["#c8452c", "#c39434", "#2f4d7a"]).map((c) => (
            <span key={c} className="h-4 w-4 rounded-full border border-black/20" style={{ background: c }} />
          ))}
        </span>
        Colour
      </button>
      {open && (
        <div className="bg-card text-ink absolute left-0 top-[calc(100%+8px)] z-40 w-[320px] rounded-xl p-4 shadow-2xl">
          <p className="text-[14px] text-black/60">Works that contain every colour you pick, up to three.</p>
          <div className="mt-3 grid grid-cols-4 gap-2">
            {PIGMENTS.map((p) => (
              <button
                key={p.hex}
                onClick={() => add(p.hex)}
                aria-pressed={colors.includes(p.hex)}
                className="overflow-hidden rounded-[3px] text-left shadow-[0_1px_2px_rgba(0,0,0,.25)] aria-pressed:ring-2 aria-pressed:ring-black"
              >
                <span className="block h-10" style={{ background: p.hex }} />
                <span className="block truncate px-1.5 py-1 text-[11px]">{p.name}</span>
              </button>
            ))}
          </div>
          <label className="mt-4 flex items-center gap-3 text-[14px]">
            <input ref={custom} type="color" defaultValue="#7a4a8a" className="h-9 w-12 cursor-pointer rounded border-0 bg-transparent" aria-label="Any colour" />
            Or any colour at all
          </label>
        </div>
      )}
    </div>
  );
}
