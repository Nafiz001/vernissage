"use client";

import { useEffect, useRef, useState } from "react";
import { Framed } from "@/components/Framed";
import { inkOn } from "@/lib/color";
import type { Artwork, Placement, Room, Rooms } from "@/lib/types";
import { doorSpan, wallLength } from "./validate";

const SNAP = 0.06;

function useSize<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });
  useEffect(() => {
    if (!ref.current) return;
    const ro = new ResizeObserver(([e]) => setSize({ width: e!.contentRect.width, height: e!.contentRect.height }));
    ro.observe(ref.current);
    return () => ro.disconnect();
  }, []);
  return [ref, size] as const;
}

/**
 * One wall, drawn to scale the way an exhibition designer draws an
 * elevation: the door, the skirting, the eye line, a person for scale, and
 * every frame at its real size. Frames drag; they snap to the eye line and
 * the wall's centre, and the gaps to their neighbours are measured.
 */
export function Elevation({
  room,
  rules,
  wall,
  paint,
  placements,
  works,
  problems,
  selected,
  onSelect,
  onMove,
  onCommit,
  onDrop,
  close,
}: {
  /** Close up: the wall as tall as the space allows, scrolling sideways. */
  close: boolean;
  room: Room;
  rules: Rooms["rules"];
  wall: number;
  paint: string;
  placements: Placement[];
  works: Map<number, Artwork>;
  problems: Map<number, string>;
  selected: number | null;
  onSelect: (id: number | null) => void;
  onMove: (id: number, x: number, y: number) => void;
  onCommit: () => void;
  onDrop: (id: number, x: number, y: number) => void;
}) {
  const [box, { width, height }] = useSize<HTMLDivElement>();
  const L = wallLength(room, wall);
  const H = room.height;
  const avail = { w: Math.max(200, width - 40), h: Math.max(200, height - 70) };
  const fit = Math.min(avail.w / (L + 1.2), avail.h / H);
  const s = close ? Math.max(fit, avail.h / H) : fit; // px per metre
  const wallW = L * s;
  const wallH = H * s;
  const drag = useRef<{ id: number; dx: number; dy: number; moved: boolean } | null>(null);
  const surface = useRef<HTMLDivElement>(null);
  const tone = inkOn(paint);
  const onWall = placements.filter((p) => p.wall === wall);

  const toMetres = (clientX: number, clientY: number) => {
    const r = surface.current!.getBoundingClientRect();
    return { x: (clientX - r.left) / s, y: (r.bottom - clientY) / s };
  };

  function snap(id: number, x: number, y: number) {
    const a = works.get(id);
    if (!a) return { x, y };
    if (Math.abs(y - rules.eyeLine) < SNAP) y = rules.eyeLine;
    if (Math.abs(x - L / 2) < SNAP) x = L / 2;
    // Keep the frame on the wall while dragging; the checks catch the rest.
    x = Math.min(Math.max(x, a.hang.w / 2), L - a.hang.w / 2);
    y = Math.min(Math.max(y, a.hang.h / 2), H - a.hang.h / 2);
    return { x: Math.round(x * 100) / 100, y: Math.round(y * 100) / 100 };
  }

  const sel = onWall.find((p) => p.artworkId === selected);
  const selWork = sel ? works.get(sel.artworkId) : undefined;

  return (
    <div
      ref={box}
      className="relative h-full min-h-[360px] overflow-x-auto overflow-y-hidden"
      onPointerDown={(e) => e.target === e.currentTarget && onSelect(null)}
    >
      <div
        className="relative flex h-full items-end justify-center pb-10"
        style={{ width: Math.max(width, wallW + 1.2 * s + 40), paddingLeft: 20, paddingRight: 0.9 * s + 20 }}
        onPointerDown={(e) => e.target === e.currentTarget && onSelect(null)}
      >
      <div
        ref={surface}
        className="plaster relative shadow-[0_30px_60px_-30px_rgba(0,0,0,.5)]"
        style={{ width: wallW, height: wallH, backgroundColor: paint }}
        onPointerDown={(e) => e.target === e.currentTarget && onSelect(null)}
        onDragOver={(e) => {
          if (e.dataTransfer.types.includes("application/x-artwork")) e.preventDefault();
        }}
        onDrop={(e) => {
          const id = Number(e.dataTransfer.getData("application/x-artwork"));
          if (!id) return;
          e.preventDefault();
          const m = toMetres(e.clientX, e.clientY);
          onDrop(id, m.x, m.y);
        }}
        aria-label="Wall elevation"
      >
        {/* Skirting. */}
        <div className="absolute inset-x-0 bottom-0" style={{ height: 0.14 * s, background: "rgba(0,0,0,.18)" }} />
        {/* Eye line. */}
        <div className="pointer-events-none absolute inset-x-0 border-t border-dashed" style={{ bottom: rules.eyeLine * s, borderColor: tone.line }}>
          <span className="absolute -top-5 left-2 text-[11px]" style={{ color: tone.soft }}>
            Eye level, {rules.eyeLine} m
          </span>
        </div>
        {/* The door, seen from inside. */}
        {wall === 2 && (
          <div
            className="absolute bottom-0 bg-[#1b1714]"
            style={{ left: doorSpan(room)[0] * s, width: room.door.width * s, height: room.door.height * s, boxShadow: "inset 0 0 0 4px #efece6" }}
          >
            <span className="absolute inset-x-0 top-2 text-center text-[11px] text-white/60">Door</span>
          </div>
        )}

        {onWall.map((p) => {
          const a = works.get(p.artworkId);
          if (!a) return null;
          const bad = problems.get(p.artworkId);
          const isSel = selected === p.artworkId;
          return (
            <div
              key={p.artworkId}
              className={`absolute cursor-grab touch-none active:cursor-grabbing ${isSel ? "z-20" : "z-10"}`}
              style={{ left: (p.x - a.hang.w / 2) * s, bottom: (p.y - a.hang.h / 2) * s }}
              onPointerDown={(e) => {
                e.stopPropagation();
                onSelect(p.artworkId);
                const m = toMetres(e.clientX, e.clientY);
                drag.current = { id: p.artworkId, dx: m.x - p.x, dy: m.y - p.y, moved: false };
                try {
                  (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
                } catch {}
              }}
              onPointerMove={(e) => {
                const d = drag.current;
                if (!d || d.id !== p.artworkId) return;
                const m = toMetres(e.clientX, e.clientY);
                const next = snap(d.id, m.x - d.dx, m.y - d.dy);
                d.moved = true;
                onMove(d.id, next.x, next.y);
              }}
              onPointerUp={() => {
                if (drag.current?.moved) onCommit();
                drag.current = null;
              }}
              title={bad ?? `${a.title}${a.artist ? `, ${a.artist}` : ""}`}
            >
              <Framed artwork={a} width={a.hang.w * s} maxSrc={800} shadow />
              {(isSel || bad) && (
                <div
                  className="pointer-events-none absolute -inset-1.5 rounded-[2px]"
                  style={{ boxShadow: `0 0 0 2px ${bad ? "#d23a2f" : "#e3c983"}` }}
                />
              )}
            </div>
          );
        })}

        {/* Measured gaps either side of the selected frame. */}
        {sel && selWork && <Gaps room={room} wall={wall} p={sel} a={selWork} onWall={onWall} works={works} s={s} ink={tone.ink} />}
      </div>

      {/* A person, 1.7 m, beside the wall for scale. */}
      <svg
        className="pointer-events-none -ml-0 self-end"
        style={{ height: 1.7 * s, width: 0.42 * s, marginLeft: 0.2 * s, flexShrink: 0 }}
        viewBox="0 0 40 170"
        aria-hidden
      >
        <circle cx="20" cy="11" r="10" fill="#1d1a16" opacity=".35" />
        <path d="M8 26h24l4 58h-6l-2 86h-9l-1-55-1 55h-9l-2-86H4z" fill="#1d1a16" opacity=".35" />
      </svg>
      </div>
      <p className="pointer-events-none sticky bottom-2 left-0 text-center text-[12px] text-black/50">
        {L.toFixed(1)} m long, {H.toFixed(1)} m high. Drag works to move them; they snap to eye level and the middle of the wall.
      </p>
    </div>
  );
}

function Gaps({ room, wall, p, a, onWall, works, s, ink }: { room: Room; wall: number; p: Placement; a: Artwork; onWall: Placement[]; works: Map<number, Artwork>; s: number; ink: string }) {
  const L = wallLength(room, wall);
  const left = p.x - a.hang.w / 2;
  const right = p.x + a.hang.w / 2;
  let prev = 0;
  let next = L;
  for (const q of onWall) {
    if (q.artworkId === p.artworkId) continue;
    const b = works.get(q.artworkId);
    if (!b) continue;
    const overlapY = Math.abs(q.y - p.y) < (a.hang.h + b.hang.h) / 2;
    if (!overlapY) continue;
    const qr = q.x + b.hang.w / 2;
    const ql = q.x - b.hang.w / 2;
    if (qr <= left) prev = Math.max(prev, qr);
    if (ql >= right) next = Math.min(next, ql);
  }
  if (wall === 2) {
    const [d0, d1] = doorSpan(room);
    if (d1 <= left) prev = Math.max(prev, d1);
    if (d0 >= right) next = Math.min(next, d0);
  }
  const y = p.y * s;
  const seg = (from: number, to: number) =>
    to - from > 0.05 ? (
      <div className="pointer-events-none absolute z-30 flex items-center justify-center" style={{ left: from * s, width: (to - from) * s, bottom: y - 8, height: 16 }}>
        <div className="absolute inset-x-0 top-1/2 border-t" style={{ borderColor: ink, opacity: 0.6 }} />
        <span className="relative rounded bg-black/65 px-1.5 text-[11px] text-white">{(to - from).toFixed(2)} m</span>
      </div>
    ) : null;
  return (
    <>
      {seg(prev, left)}
      {seg(right, next)}
    </>
  );
}
