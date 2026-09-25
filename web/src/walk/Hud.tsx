"use client";

import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { DeepZoom } from "@/components/DeepZoom";
import { WallLabel } from "@/components/WallLabel";
import { initials, relativeDays } from "@/lib/format";
import type { Artwork, Exhibition, Placement, Room } from "@/lib/types";
import { PANEL, tourOrder, viewingPose, wallPoint } from "./geometry";
import { speak } from "./sound";
import { motion as live, useWalk } from "./store";
import { EMOJI } from "./Visitors";

type Live = ReturnType<typeof import("./live").useLive>;

const panel = "rounded-2xl bg-[#14110f]/78 text-[#f2ede3] shadow-2xl backdrop-blur-md";

export function Hud({ e, room, placements, works, conn }: { e: Exhibition; room: Room; placements: Placement[]; works: Map<number, Artwork>; conn: Live }) {
  const entered = useWalk((s) => s.entered);
  const viewing = useWalk((s) => s.viewing);
  const view = useWalk((s) => s.view);
  const glideTo = useWalk((s) => s.glideTo);
  const [book, setBook] = useState(false);
  const order = useMemo(() => tourOrder(placements), [placements]);

  // Choosing a work walks you to it and tells the room what you're looking at.
  useEffect(() => {
    conn.look(viewing);
    if (viewing === null) {
      speak(null);
      return;
    }
    const p = placements.find((q) => q.artworkId === viewing);
    const a = works.get(viewing);
    const wide = window.innerWidth > 900;
    const aspect = (window.innerWidth - (wide ? PANEL : 0)) / window.innerHeight;
    if (p && a) glideTo(viewingPose(room, p, a.hang.w, a.hang.h, aspect));
  }, [viewing]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === "Escape") view(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [view]);

  return (
    <div className="pointer-events-none fixed inset-0 z-10 select-none text-[15px]">
      <TopBar e={e} onBook={() => setBook((b) => !b)} />
      {entered && <Minimap room={room} placements={placements} works={works} />}
      {entered && <Reactions conn={conn} />}
      <AnimatePresence>
        {viewing !== null && works.get(viewing) && (
          <Viewing key="viewing" a={works.get(viewing)!} p={placements.find((q) => q.artworkId === viewing)!} order={order} works={works} />
        )}
      </AnimatePresence>
      <AnimatePresence>{book && <Guestbook key="book" conn={conn} onClose={() => setBook(false)} />}</AnimatePresence>
      <Notice />
      {!entered && <Welcome e={e} conn={conn} room={room} />}
    </div>
  );
}

function TopBar({ e, onBook }: { e: Exhibition; onBook: () => void }) {
  const peers = useWalk((s) => s.peers);
  const me = useWalk((s) => s.me);
  const status = useWalk((s) => s.status);
  const sound = useWalk((s) => s.sound);
  const setSound = useWalk((s) => s.setSound);
  const quality = useWalk((s) => s.quality);
  const setQuality = useWalk((s) => s.setQuality);
  const [copied, setCopied] = useState(false);
  const people = [...(me ? [me] : []), ...Object.values(peers)];
  return (
    <div className="pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between gap-4 p-4 sm:p-5">
      <div className={`${panel} pointer-events-auto flex items-center gap-3 py-2 pl-2 pr-4`}>
        <Link href={`/e/${e.slug}`} className="grid h-10 place-items-center rounded-full px-3 hover:bg-white/10" aria-label="Leave the room">
          ← Leave
        </Link>
        <div className="hidden min-w-0 sm:block">
          <p className="lettering-sm truncate text-[19px]">{e.title}</p>
          <p className="text-[13px] text-white/60">Curated by {e.owner.name}</p>
        </div>
      </div>
      <div className={`${panel} pointer-events-auto flex items-center gap-1 p-1.5`}>
        <div className="flex items-center pl-2 pr-1" title={people.map((p) => p.name).join(", ")}>
          <div className="flex -space-x-2">
            {people.slice(0, 5).map((p) => (
              <span key={p.id} className="grid h-8 w-8 place-items-center rounded-full border-2 border-[#14110f] text-[11px] font-semibold text-white" style={{ background: `hsl(${p.hue} 45% 48%)` }}>
                {initials(p.name)}
              </span>
            ))}
          </div>
          <span className="ml-2.5 mr-1 text-[14px] text-white/80">
            {status === "open" ? `${people.length} here` : status === "connecting" ? "Connecting…" : "Reconnecting…"}
          </span>
        </div>
        <button className="h-10 rounded-full px-3 hover:bg-white/10" onClick={onBook}>
          Guestbook
        </button>
        <button className="h-10 rounded-full px-3 hover:bg-white/10" aria-pressed={sound} onClick={() => setSound(!sound)}>
          {sound ? "Sound on" : "Sound off"}
        </button>
        <button
          className="hidden h-10 rounded-full px-3 hover:bg-white/10 md:block"
          onClick={() => setQuality(quality === "high" ? "medium" : quality === "medium" ? "low" : "high")}
          title="Lower quality runs faster on older computers"
        >
          Quality: {quality}
        </button>
        <button
          className="h-10 rounded-full px-3 hover:bg-white/10"
          onClick={() => {
            navigator.clipboard?.writeText(window.location.href);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
          }}
        >
          {copied ? "Copied" : "Invite"}
        </button>
      </div>
    </div>
  );
}

function Welcome({ e, conn, room }: { e: Exhibition; conn: Live; room: Room }) {
  const me = useWalk((s) => s.me);
  const enter = useWalk((s) => s.enter);
  const glideTo = useWalk((s) => s.glideTo);
  const [name, setName] = useState("");
  const opening = e.openingAt && new Date(e.openingAt).getTime() > Date.now() ? relativeDays(e.openingAt) : null;
  function go() {
    if (name.trim() && !me?.member) conn.rename(name.trim().slice(0, 24));
    enter();
    // Through the doors, into the middle of the room.
    glideTo({ x: 0, z: room.depth / 2 - Math.min(2.6, room.depth * 0.3), yaw: 0, pitch: 0 });
  }
  return (
    <motion.div className="pointer-events-auto absolute inset-x-0 bottom-0 flex justify-center p-4 sm:p-8" initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.4, duration: 0.6 }}>
      <form
        className={`${panel} w-full max-w-[520px] p-6 sm:p-7`}
        onSubmit={(ev) => {
          ev.preventDefault();
          go();
        }}
      >
        <p className="lettering-sm text-[30px]">{e.title}</p>
        <p className="mt-1 text-white/70">
          {e.workCount} works, curated by {e.owner.name}
          {opening && `. Opening night ${opening}`}
        </p>
        {me && !me.member && (
          <label className="mt-5 block">
            <span className="text-[14px] text-white/70">
              You&apos;ll appear to others as <span style={{ color: `hsl(${me.hue} 60% 70%)` }}>{me.name}</span>. Or give your name:
            </span>
            <input
              value={name}
              onChange={(ev) => setName(ev.target.value)}
              maxLength={24}
              placeholder="Your name"
              className="mt-2 w-full rounded-lg border border-white/20 bg-white/5 px-3 py-2.5 text-white placeholder:text-white/40 focus:border-white/60 focus:outline-none"
            />
          </label>
        )}
        <button className="mt-6 w-full rounded-full bg-[#f2ede3] py-3.5 text-[17px] font-medium text-[#1d1a16] hover:bg-white" autoFocus>
          Walk in
        </button>
        <p className="mt-4 text-center text-[13px] text-white/55">
          Drag to look around. Walk with W A S D or the arrow keys, or click the floor. Click a work to look at it.
        </p>
      </form>
    </motion.div>
  );
}

function Viewing({ a, p, order, works }: { a: Artwork; p: Placement; order: Placement[]; works: Map<number, Artwork> }) {
  const view = useWalk((s) => s.view);
  const touring = useWalk((s) => s.touring);
  const setTouring = useWalk((s) => s.setTouring);
  const peers = useWalk((s) => s.peers);
  const [zoom, setZoom] = useState(false);
  const [more, setMore] = useState(false);
  const [listening, setListening] = useState(false);
  const i = order.findIndex((q) => q.artworkId === a.id);
  const step = (d: number) => view(order[(i + d + order.length) % order.length]!.artworkId);
  const others = Object.values(peers).filter((q) => q.looking === a.id);

  // The tour moves on by itself, after the audio guide if it's speaking.
  useEffect(() => {
    if (!touring) return;
    const t = setTimeout(() => step(1), listening ? 22000 : 11000);
    return () => clearTimeout(t);
  }, [touring, a.id, listening]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!listening) return;
    const text = [
      a.title,
      a.artist ? `by ${a.artist}` : "",
      a.date,
      p.label,
      (a.description ?? "").split(/(?<=\.)\s/).slice(0, 3).join(" "),
    ]
      .filter(Boolean)
      .join(". ");
    speak(text);
  }, [listening, a.id]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => () => void speak(null), []);

  return (
    <motion.aside
      className={`${panel} pointer-events-auto absolute bottom-4 right-4 top-24 flex w-[min(380px,calc(100vw-2rem))] flex-col overflow-hidden sm:bottom-5 sm:right-5`}
      initial={{ opacity: 0, x: 30 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: 30 }}
      transition={{ duration: 0.4, ease: [0.65, 0, 0.35, 1] }}
      aria-label="The work you're looking at"
    >
      <div className="flex-1 overflow-y-auto p-5">
        <WallLabel artwork={a} note={p.label || undefined} />
        {others.length > 0 && (
          <p className="mt-3 text-[13px] text-white/70">
            {others.map((o) => o.name).join(", ")} {others.length === 1 ? "is" : "are"} looking at this too.
          </p>
        )}
        {a.description && (
          <div className="mt-4">
            <p className={`prose-serif text-[15px] text-white/85 ${more ? "" : "line-clamp-5"}`}>{a.description}</p>
            <button className="mt-1 text-[13px] text-white/60 underline underline-offset-4" onClick={() => setMore((m) => !m)}>
              {more ? "Less" : "Read more"}
            </button>
          </div>
        )}
      </div>
      <div className="grid grid-cols-2 gap-2 border-t border-white/10 p-4">
        <button className="rounded-full bg-[#f2ede3] py-2.5 font-medium text-[#1d1a16]" onClick={() => setZoom(true)}>
          Look closer
        </button>
        <button className="rounded-full border border-white/20 py-2.5 hover:border-white/60" aria-pressed={listening} onClick={() => setListening((l) => !l)}>
          {listening ? "Stop listening" : "Listen"}
        </button>
        <button className="rounded-full border border-white/20 py-2.5 hover:border-white/60" onClick={() => step(-1)}>
          ← Previous
        </button>
        <button className="rounded-full border border-white/20 py-2.5 hover:border-white/60" onClick={() => step(1)}>
          Next →
        </button>
        <button className="rounded-full py-2 text-white/75 hover:bg-white/10" aria-pressed={touring} onClick={() => setTouring(!touring)}>
          {touring ? "Stop the tour" : "Take the tour"}
        </button>
        <button className="rounded-full py-2 text-white/75 hover:bg-white/10" onClick={() => view(null)}>
          Back to the room
        </button>
      </div>
      <p className="px-4 pb-3 text-center text-[12px] text-white/45">
        {i + 1} of {order.length}
        {works.size > order.length ? "" : ""}
      </p>
      <DeepZoom id={a.id} title={a.title} open={zoom} onClose={() => setZoom(false)} />
    </motion.aside>
  );
}

function Reactions({ conn }: { conn: Live }) {
  return (
    <div className={`${panel} pointer-events-auto absolute bottom-4 left-1/2 flex -translate-x-1/2 gap-1 p-1.5 sm:bottom-5`} role="group" aria-label="React">
      {Object.entries(EMOJI).map(([k, e]) => (
        <button key={k} className="grid h-11 w-11 place-items-center rounded-full text-[22px] transition-transform hover:scale-110 hover:bg-white/10 active:scale-95" onClick={() => conn.react(k)} aria-label={k}>
          {e}
        </button>
      ))}
    </div>
  );
}

function Guestbook({ conn, onClose }: { conn: Live; onClose: () => void }) {
  const entries = useWalk((s) => s.guestbook);
  const [text, setText] = useState("");
  return (
    <motion.aside
      className={`${panel} pointer-events-auto absolute bottom-24 left-4 top-24 flex w-[min(360px,calc(100vw-2rem))] flex-col overflow-hidden sm:left-5`}
      initial={{ opacity: 0, x: -30 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: -30 }}
      aria-label="Guestbook"
    >
      <div className="flex items-center justify-between border-b border-white/10 p-4">
        <p className="lettering-sm text-[24px]">Guestbook</p>
        <button className="rounded-full px-3 py-1.5 hover:bg-white/10" onClick={onClose}>
          Close
        </button>
      </div>
      <form
        className="border-b border-white/10 p-4"
        onSubmit={(ev) => {
          ev.preventDefault();
          if (!text.trim()) return;
          conn.sign(text.trim());
          setText("");
        }}
      >
        <textarea
          value={text}
          onChange={(ev) => setText(ev.target.value)}
          maxLength={280}
          rows={3}
          placeholder="Leave a few words for the curator"
          className="w-full resize-none rounded-lg border border-white/20 bg-white/5 p-3 text-white placeholder:text-white/40 focus:border-white/60 focus:outline-none"
        />
        <button className="mt-2 rounded-full bg-[#f2ede3] px-5 py-2 font-medium text-[#1d1a16]">Sign</button>
      </form>
      <ul className="flex-1 space-y-4 overflow-y-auto p-4">
        {entries.length === 0 && <li className="text-white/60">No one has signed yet. Be the first.</li>}
        {entries.map((g) => (
          <li key={g.id}>
            <p className="lettering-sm text-[17px] italic leading-snug">“{g.message}”</p>
            <p className="mt-1 text-[13px]" style={{ color: `hsl(${g.hue} 60% 72%)` }}>
              {g.name}
              <span className="text-white/45"> {relativeDays(g.createdAt)}</span>
            </p>
          </li>
        ))}
      </ul>
    </motion.aside>
  );
}

function Notice() {
  const notice = useWalk((s) => s.notice);
  const set = useWalk((s) => s.set);
  useEffect(() => {
    if (!notice) return;
    const t = setTimeout(() => set({ notice: null }), 3500);
    return () => clearTimeout(t);
  }, [notice, set]);
  return (
    <AnimatePresence>
      {notice && (
        <motion.p className={`${panel} absolute left-1/2 top-24 -translate-x-1/2 px-4 py-2.5`} initial={{ opacity: 0, y: -8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
          {notice}
        </motion.p>
      )}
    </AnimatePresence>
  );
}

/** The room from above: the works on the walls, you, and everyone else. */
function Minimap({ room, placements, works }: { room: Room; placements: Placement[]; works: Map<number, Artwork> }) {
  const [, force] = useState(0);
  const glideTo = useWalk((s) => s.glideTo);
  const peers = useWalk((s) => s.peers);
  const svg = useRef<SVGSVGElement>(null);
  useEffect(() => {
    const t = setInterval(() => force((n) => n + 1), 120);
    return () => clearInterval(t);
  }, []);
  const s = 150 / Math.max(room.width, room.depth + 2);
  const w = room.width * s;
  const h = (room.depth + 2) * s;
  const px = (x: number) => (x + room.width / 2) * s;
  const pz = (z: number) => (z + room.depth / 2) * s;
  const me = live.me;
  return (
    <div className={`${panel} pointer-events-auto absolute bottom-4 left-4 hidden p-3 sm:bottom-5 sm:left-5 sm:block`}>
      <svg
        ref={svg}
        width={w}
        height={h}
        className="cursor-crosshair"
        onClick={(ev) => {
          const r = svg.current!.getBoundingClientRect();
          const x = (ev.clientX - r.left) / s - room.width / 2;
          const z = (ev.clientY - r.top) / s - room.depth / 2;
          if (Math.abs(x) < room.width / 2 - 0.4 && Math.abs(z) < room.depth / 2 - 0.4) glideTo({ x, z, yaw: me.yaw, pitch: 0 });
        }}
        aria-label="Map of the room. Click to walk there."
      >
        <rect x={1} y={1} width={w - 2} height={room.depth * s - 2} fill="rgba(255,255,255,.04)" stroke="rgba(255,255,255,.5)" strokeWidth={1.5} />
        <rect x={px(-room.door.width / 2)} y={room.depth * s - 2.5} width={room.door.width * s} height={4} fill="#14110f" />
        {placements.map((p) => {
          const a = works.get(p.artworkId);
          if (!a) return null;
          const f0 = wallPoint(room, p.wall, p.x - a.hang.w / 2);
          const f1 = wallPoint(room, p.wall, p.x + a.hang.w / 2);
          return <line key={p.artworkId} x1={px(f0.x)} y1={pz(f0.z)} x2={px(f1.x)} y2={pz(f1.z)} stroke="#e3c983" strokeWidth={3.5} />;
        })}
        {Object.values(peers).map((p) => {
          const m = live.peers.get(p.id);
          return m ? <circle key={p.id} cx={px(m.x)} cy={pz(m.z)} r={4} fill={`hsl(${p.hue} 55% 60%)`} /> : null;
        })}
        <g transform={`translate(${px(me.x)} ${pz(me.z)}) rotate(${(-me.yaw * 180) / Math.PI})`}>
          <path d="M0 -7 L5 5 L0 2 L-5 5 Z" fill="#fff" />
        </g>
      </svg>
    </div>
  );
}
