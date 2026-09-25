"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Framed } from "@/components/Framed";
import { api } from "@/lib/api";
import { WALL_NAMES, cm } from "@/lib/format";
import type { Artwork, ArtworkPage, Exhibition, Placement, Room, Rooms } from "@/lib/types";
import { doorSpan, wallLength } from "./validate";

const FLOOR_SWATCH: Record<string, string> = {
  oak: "repeating-linear-gradient(90deg,#b88a5a 0 8px,#a87c4f 8px 16px)",
  walnut: "repeating-linear-gradient(0deg,#5e3d27 0 10px,#6b4630 10px 20px)",
  concrete: "radial-gradient(circle at 30% 30%,#827d77,#6a6660)",
  marble: "linear-gradient(135deg,#ebe7df,#d9d4ca 45%,#efebe4)",
};

/** The works waiting to be hung: your selection, or a search of the collection. */
export function Tray({
  hung,
  extra,
  onAdd,
  onHangAll,
  busy,
  register,
}: {
  hung: Set<number>;
  extra: Artwork[];
  onAdd: (a: Artwork) => void;
  onHangAll: (ids: number[]) => void;
  busy: boolean;
  /** Tells the studio about works shown here, so they can be dropped on a wall. */
  register: (works: Artwork[]) => void;
}) {
  const [tab, setTab] = useState<"selection" | "search">("selection");
  const [q, setQ] = useState("");
  const [term, setTerm] = useState("");
  const saved = useQuery({ queryKey: ["saved"], queryFn: () => api<{ items: Artwork[] }>("/api/me/saved") });
  const search = useQuery({
    queryKey: ["studio-search", term],
    queryFn: () => api<ArtworkPage>(`/api/artworks?q=${encodeURIComponent(term)}&limit=30`),
    enabled: tab === "search" && term.length > 1,
  });
  const list = useMemo(
    () => (tab === "selection" ? [...extra, ...(saved.data?.items ?? []).filter((a) => !extra.some((e) => e.id === a.id))] : (search.data?.items ?? [])),
    [tab, extra, saved.data, search.data],
  );
  const waiting = list.filter((a) => !hung.has(a.id));
  useEffect(() => {
    if (list.length) register(list);
  }, [list, register]);

  return (
    <div className="flex h-full flex-col">
      <div className="flex gap-1 p-3" role="tablist">
        {(["selection", "search"] as const).map((t) => (
          <button key={t} role="tab" aria-selected={tab === t} onClick={() => setTab(t)} className="flex-1 rounded-full py-2 text-[14px] aria-selected:bg-black/80 aria-selected:text-white hover:bg-black/5 aria-selected:hover:bg-black/80">
            {t === "selection" ? "Your selection" : "Search"}
          </button>
        ))}
      </div>
      {tab === "search" && (
        <form
          className="px-3 pb-2"
          onSubmit={(e) => {
            e.preventDefault();
            setTerm(q.trim());
          }}
        >
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Artist, title, subject…" className="field !min-h-10 !rounded-full !bg-white text-[14px]" aria-label="Search the collection" />
        </form>
      )}
      <p className="px-4 pb-2 text-[13px] text-black/55">
        {tab === "selection"
          ? waiting.length
            ? "Drag a work onto the wall, or click it to hang it in the widest free space."
            : "Everything you saved is on the walls. Save more from the collection, or search here."
          : term
            ? `${search.data?.total ?? "…"} works`
            : "Search the whole collection."}
      </p>
      <ul className="grid flex-1 auto-rows-min grid-cols-2 gap-x-3 gap-y-4 overflow-y-auto px-3 pb-4">
        {list.map((a) => {
          const on = hung.has(a.id);
          return (
            <li key={a.id}>
              <button
                draggable={!on}
                onDragStart={(e) => {
                  e.dataTransfer.setData("application/x-artwork", String(a.id));
                  e.dataTransfer.effectAllowed = "copy";
                }}
                onClick={() => !on && onAdd(a)}
                disabled={on}
                className="group flex w-full flex-col items-center gap-1.5 rounded-lg p-2 text-left hover:bg-black/5 disabled:opacity-40"
                title={on ? "Already on a wall" : `Hang ${a.title}`}
              >
                <div className="grid h-[92px] place-items-center">
                  <Framed artwork={a} width={Math.min(120, 80 * Math.sqrt(a.hang.w / Math.max(0.2, a.hang.h)))} maxSrc={400} shadow={false} />
                </div>
                <span className="line-clamp-2 w-full text-[12px] leading-tight">
                  <span className="italic">{a.title}</span>
                  {a.artist && <span className="text-black/55">, {a.artist}</span>}
                </span>
              </button>
            </li>
          );
        })}
      </ul>
      {tab === "selection" && waiting.length > 1 && (
        <div className="border-t border-black/10 p-3">
          <button className="btn w-full bg-black/85 text-white hover:bg-black" disabled={busy} onClick={() => onHangAll(waiting.map((a) => a.id))}>
            {busy ? "Hanging…" : `Hang all ${waiting.length} for me`}
          </button>
        </div>
      )}
    </div>
  );
}

/** A plan of the room from above; click a wall to work on it. */
export function Plan({ room, wall, onWall, placements, works }: { room: Room; wall: number; onWall: (w: number) => void; placements: Placement[]; works: Map<number, Artwork> }) {
  const s = 120 / Math.max(room.width, room.depth);
  const W = room.width * s;
  const D = room.depth * s;
  const pad = 10;
  const [d0, d1] = doorSpan(room);
  const lines: Record<number, [number, number, number, number]> = {
    0: [0, 0, W, 0],
    1: [W, 0, W, D],
    2: [W, D, 0, D],
    3: [0, D, 0, 0],
  };
  const at = (w: number, u: number): [number, number] => {
    const [x1, y1, x2, y2] = lines[w]!;
    const t = u / wallLength(room, w);
    return [x1 + (x2 - x1) * t, y1 + (y2 - y1) * t];
  };
  return (
    <svg width={W + pad * 2} height={D + pad * 2} viewBox={`${-pad} ${-pad} ${W + pad * 2} ${D + pad * 2}`} aria-label="Plan of the room">
      <rect x={0} y={0} width={W} height={D} fill="rgba(0,0,0,.04)" />
      {[0, 1, 2, 3].map((w) => {
        const [x1, y1, x2, y2] = lines[w]!;
        return (
          <line
            key={w}
            x1={x1}
            y1={y1}
            x2={x2}
            y2={y2}
            stroke={w === wall ? "#b89455" : "rgba(0,0,0,.45)"}
            strokeWidth={w === wall ? 5 : 2}
            className="cursor-pointer"
            onClick={() => onWall(w)}
          >
            <title>{WALL_NAMES[w]}</title>
          </line>
        );
      })}
      {/* The doorway. */}
      <line x1={W - d0 * s} y1={D} x2={W - d1 * s} y2={D} stroke="#f7f5f0" strokeWidth={6} />
      {placements.map((p) => {
        const a = works.get(p.artworkId);
        if (!a) return null;
        const [x1, y1] = at(p.wall, p.x - a.hang.w / 2);
        const [x2, y2] = at(p.wall, p.x + a.hang.w / 2);
        return <line key={p.artworkId} x1={x1} y1={y1} x2={x2} y2={y2} stroke="#1d1a16" strokeWidth={3} />;
      })}
      {room.benches.map((b, i) => (
        <rect key={i} x={(b.x - b.width / 2 + room.width / 2) * s} y={(b.z - b.depth / 2 + room.depth / 2) * s} width={b.width * s} height={b.depth * s} fill="rgba(0,0,0,.2)" />
      ))}
    </svg>
  );
}

/** Settings for the whole room, shown when no work is selected. */
export function RoomSettings({ e, rooms, onChange, onDelete }: { e: Exhibition; rooms: Rooms; onChange: (patch: Record<string, unknown>) => void; onDelete: () => void }) {
  const [statement, setStatement] = useState(e.statement);
  const [saved, setSaved] = useState(e.statement);
  if (e.statement !== saved) {
    setSaved(e.statement);
    setStatement(e.statement);
  }
  const [opening, setOpening] = useState(e.openingAt ? toLocal(e.openingAt) : "");
  useEffect(() => {
    if (statement === e.statement) return;
    const t = setTimeout(() => onChange({ statement }), 800);
    return () => clearTimeout(t);
  }, [statement]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-7 p-5">
      <section>
        <h3 className="mb-2 text-[14px] font-semibold">Room</h3>
        <div className="grid grid-cols-2 gap-2">
          {rooms.rooms.map((r) => (
            <button
              key={r.key}
              aria-pressed={e.room === r.key}
              onClick={() => onChange({ room: r.key })}
              className="rounded-lg border border-black/10 p-2.5 text-left hover:border-black/40 aria-pressed:border-black aria-pressed:bg-white"
              title={r.blurb}
            >
              <svg width="100%" height="34" viewBox={`0 0 ${Math.max(r.width, 24)} 12`} preserveAspectRatio="xMinYMid meet" aria-hidden>
                <rect x="0.5" y="0.5" width={r.width} height={Math.min(11, r.depth)} fill="none" stroke="currentColor" strokeWidth="0.8" />
              </svg>
              <span className="mt-1 block text-[13px] font-medium">{r.name}</span>
              <span className="block text-[12px] text-black/55">
                {r.width} × {r.depth} m, {r.height} m high
              </span>
            </button>
          ))}
        </div>
      </section>
      <section>
        <h3 className="mb-2 text-[14px] font-semibold">Wall colour</h3>
        <div className="grid grid-cols-4 gap-2">
          {rooms.paints.map((p) => (
            <button key={p.key} aria-pressed={e.paint === p.key} onClick={() => onChange({ paint: p.key })} className="overflow-hidden rounded-[4px] bg-white text-left shadow-[0_1px_2px_rgba(0,0,0,.2)] aria-pressed:ring-2 aria-pressed:ring-black" title={p.name}>
              <span className="block h-9" style={{ background: p.hex }} />
              <span className="block truncate px-1.5 py-1 text-[11px]">{p.name}</span>
            </button>
          ))}
        </div>
      </section>
      <section className="grid grid-cols-2 gap-5">
        <div>
          <h3 className="mb-2 text-[14px] font-semibold">Floor</h3>
          <div className="space-y-1.5">
            {rooms.floors.map((f) => (
              <button key={f.key} aria-pressed={e.floor === f.key} onClick={() => onChange({ floor: f.key })} className="flex w-full items-center gap-2 rounded-md px-1.5 py-1 text-left text-[13px] hover:bg-black/5 aria-pressed:bg-black/80 aria-pressed:text-white">
                <span className="h-5 w-5 shrink-0 rounded-sm border border-black/15" style={{ background: FLOOR_SWATCH[f.key] }} />
                {f.name}
              </button>
            ))}
          </div>
        </div>
        <div>
          <h3 className="mb-2 text-[14px] font-semibold">Lighting</h3>
          <div className="space-y-1.5">
            {rooms.lights.map((l) => (
              <button key={l.key} aria-pressed={e.light === l.key} onClick={() => onChange({ light: l.key })} className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-black/5 aria-pressed:bg-black/80 aria-pressed:text-white">
                {l.name}
              </button>
            ))}
          </div>
        </div>
      </section>
      <section>
        <label className="block">
          <span className="mb-2 block text-[14px] font-semibold">Curator&apos;s statement</span>
          <textarea
            value={statement}
            onChange={(ev) => setStatement(ev.target.value)}
            maxLength={2000}
            rows={6}
            placeholder="What ties these works together? Visitors read this on the wall as they come in."
            className="w-full resize-y rounded-lg border border-black/15 bg-white p-3 text-[14px] leading-relaxed focus:border-black focus:outline-none"
          />
        </label>
      </section>
      <section>
        <label className="block">
          <span className="mb-2 block text-[14px] font-semibold">Opening night</span>
          <input
            type="datetime-local"
            value={opening}
            onChange={(ev) => {
              setOpening(ev.target.value);
              onChange({ openingAt: ev.target.value ? new Date(ev.target.value).toISOString() : null });
            }}
            className="w-full rounded-lg border border-black/15 bg-white p-2.5 text-[14px]"
          />
          <span className="mt-1 block text-[12px] text-black/55">When you&apos;ll be in the room to welcome visitors. Optional.</span>
        </label>
      </section>
      <section className="border-t border-black/10 pt-5">
        <button className="text-[13px] text-[#b3261e] underline underline-offset-4" onClick={onDelete}>
          Delete this exhibition
        </button>
      </section>
    </div>
  );
}

function toLocal(iso: string) {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** Details of the selected work: its size, its label, where it hangs. */
export function WorkSettings({
  a,
  p,
  problem,
  isCover,
  onLabel,
  onCover,
  onMoveWall,
  onRemove,
  onNudge,
}: {
  a: Artwork;
  p: Placement;
  problem?: string;
  isCover: boolean;
  onLabel: (label: string) => void;
  onCover: () => void;
  onMoveWall: (w: number) => void;
  onRemove: () => void;
  onNudge: (x: number, y: number) => void;
}) {
  return (
    <div className="space-y-5 p-5">
      <div className="grid place-items-center rounded-lg bg-black/[.04] py-5">
        <Framed artwork={a} width={Math.min(220, 150 * Math.sqrt(a.hang.w / Math.max(0.2, a.hang.h)))} maxSrc={800} />
      </div>
      <div>
        <p className="italic">{a.title}</p>
        <p className="text-[14px] text-black/60">{[a.artist, a.date].filter(Boolean).join(", ")}</p>
        <p className="mt-2 text-[13px] text-black/60">
          Picture {cm(a.hang.artH * 100)} × {cm(a.hang.artW * 100)}, framed {cm(a.hang.h * 100)} × {cm(a.hang.w * 100)}
          {a.hang.estimated && ". The museum doesn't give its size; this is a guess"}.
        </p>
      </div>
      {problem && <p className="rounded-lg bg-[#b3261e]/10 p-3 text-[14px] text-[#8c1d17]">{problem}</p>}
      <div className="grid grid-cols-2 gap-3 text-[13px]">
        <label>
          <span className="mb-1 block text-black/60">From the left</span>
          <input type="number" step={0.05} value={+p.x.toFixed(2)} onChange={(e) => onNudge(Number(e.target.value), p.y)} className="w-full rounded-md border border-black/15 bg-white p-2" />
        </label>
        <label>
          <span className="mb-1 block text-black/60">Centre height</span>
          <input type="number" step={0.05} value={+p.y.toFixed(2)} onChange={(e) => onNudge(p.x, Number(e.target.value))} className="w-full rounded-md border border-black/15 bg-white p-2" />
        </label>
      </div>
      <div>
        <p className="mb-1.5 text-[13px] text-black/60">Move to</p>
        <div className="grid grid-cols-4 gap-1.5">
          {WALL_NAMES.map((n, w) => (
            <button key={w} disabled={w === p.wall} onClick={() => onMoveWall(w)} className="rounded-md border border-black/15 py-1.5 text-[12px] hover:border-black disabled:bg-black/80 disabled:text-white">
              {n.split(" ")[0]}
            </button>
          ))}
        </div>
      </div>
      <label className="block">
        <span className="mb-1.5 block text-[14px] font-semibold">Your wall label</span>
        <textarea
          value={p.label}
          onChange={(e) => onLabel(e.target.value)}
          maxLength={600}
          rows={4}
          placeholder="Why this work is here. Printed under the museum's label, in your words."
          className="w-full resize-y rounded-lg border border-black/15 bg-white p-3 text-[14px] leading-relaxed focus:border-black focus:outline-none"
        />
      </label>
      <div className="flex flex-wrap gap-2">
        <button className="btn border border-black/15 text-[14px] hover:border-black" disabled={isCover} onClick={onCover}>
          {isCover ? "This is the cover" : "Use as the cover"}
        </button>
        <button className="btn text-[14px] text-[#b3261e] hover:bg-[#b3261e]/10" onClick={onRemove}>
          Take down
        </button>
      </div>
    </div>
  );
}
