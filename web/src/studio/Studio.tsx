"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { WALL_NAMES, count } from "@/lib/format";
import type { Artwork, ExhibitionDetail, Placement, Rooms } from "@/lib/types";
import { Elevation } from "./Elevation";
import { Plan, RoomSettings, Tray, WorkSettings } from "./Panels";
import { freeSpot, validate } from "./validate";

type SaveState = { state: "saved" | "saving" | "error"; message?: string };

export function Studio({ id }: { id: string }) {
  const qc = useQueryClient();
  const router = useRouter();
  const add = useSearchParams().get("add");
  const rooms = useQuery({ queryKey: ["rooms"], queryFn: () => api<Rooms>("/api/rooms"), staleTime: Infinity });
  const detail = useQuery({ queryKey: ["exhibition", id], queryFn: () => api<ExhibitionDetail>(`/api/exhibitions/${id}`) });

  const [ps, setPs] = useState<Placement[] | null>(null);
  const history = useRef<{ past: Placement[][]; future: Placement[][] }>({ past: [], future: [] });
  const [known, setKnown] = useState(() => new Map<number, Artwork>());
  const [extra, setExtra] = useState<Artwork[]>([]);
  const [wall, setWall] = useState(0);
  const [selected, setSelected] = useState<number | null>(null);
  const [save, setSave] = useState<SaveState>({ state: "saved" });
  const [notice, setNotice] = useState<string | null>(null);
  const [opened, setOpened] = useState(false);
  const [close, setClose] = useState<boolean | null>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const e = detail.data?.exhibition;
  const room = rooms.data?.rooms.find((r) => r.key === e?.room);
  const paint = rooms.data?.paints.find((p) => p.key === e?.paint)?.hex ?? "#5b1d1f";

  // First load: take the server's hang and works.
  if (detail.data && ps === null) {
    setPs(detail.data.placements);
    setKnown((m) => {
      const n = new Map(m);
      detail.data!.works.forEach((w) => n.set(w.id, w));
      return n;
    });
  }

  const register = useCallback((list: Artwork[]) => {
    setKnown((m) => {
      if (list.every((a) => m.has(a.id))) return m;
      const n = new Map(m);
      list.forEach((a) => n.set(a.id, a));
      return n;
    });
  }, []);

  // A work sent from its page with ?add=.
  useEffect(() => {
    if (!add) return;
    api<{ artwork: Artwork }>(`/api/artworks/${add}`).then(({ artwork }) => {
      register([artwork]);
      setExtra((x) => (x.some((a) => a.id === artwork.id) ? x : [artwork, ...x]));
    });
  }, [add, register]);

  const problems = useMemo(() => {
    if (!room || !rooms.data || !ps) return new Map<number, string>();
    return new Map(validate(room, rooms.data.rules, ps, known).map((p) => [p.artworkId, p.message]));
  }, [room, rooms.data, ps, known]);

  const persist = useCallback(
    (next: Placement[]) => {
      clearTimeout(saveTimer.current);
      setSave({ state: "saving" });
      saveTimer.current = setTimeout(async () => {
        try {
          const d = await api<ExhibitionDetail>(`/api/exhibitions/${id}/placements`, { method: "PUT", json: { placements: next } });
          qc.setQueryData(["exhibition", id], (old: ExhibitionDetail | undefined) => (old ? { ...old, exhibition: d.exhibition, works: d.works } : d));
          setSave({ state: "saved" });
        } catch (err) {
          setSave({ state: "error", message: err instanceof ApiError ? err.message : "Couldn't save." });
        }
      }, 700);
    },
    [id, qc],
  );

  /** Records a change for undo, and saves it. */
  const commit = useCallback(
    (next: Placement[], before?: Placement[]) => {
      if (before) history.current.past.push(before);
      history.current.future = [];
      if (history.current.past.length > 80) history.current.past.shift();
      setPs(next);
      persist(next);
    },
    [persist],
  );

  const dragStart = useRef<Placement[] | null>(null);
  const move = (aid: number, x: number, y: number) => {
    setPs((cur) => {
      if (!cur) return cur;
      if (!dragStart.current) dragStart.current = cur;
      return cur.map((p) => (p.artworkId === aid ? { ...p, x, y } : p));
    });
  };
  const endDrag = () => {
    if (ps && dragStart.current) commit(ps, dragStart.current);
    dragStart.current = null;
  };

  function place(a: Artwork, at?: { x: number; y: number }, onWall = wall) {
    if (!ps || !room || !rooms.data) return;
    register([a]);
    if (ps.some((p) => p.artworkId === a.id)) return;
    if (ps.length >= rooms.data.rules.maxWorks) return setNotice(`An exhibition can hang up to ${rooms.data.rules.maxWorks} works.`);
    let target = onWall;
    let spot = at ?? freeSpot(room, rooms.data.rules, onWall, ps, known, a.hang.w, a.hang.h);
    if (!spot) {
      for (const w of [0, 1, 3, 2]) {
        spot = freeSpot(room, rooms.data.rules, w, ps, known, a.hang.w, a.hang.h);
        if (spot) {
          target = w;
          break;
        }
      }
    }
    if (!spot) return setNotice("There's no free wall left for this work. Take something down, or choose a bigger room.");
    const y = at ? Math.min(Math.max(at.y, a.hang.h / 2), room.height - a.hang.h / 2) : spot.y;
    commit([...ps, { artworkId: a.id, wall: target, x: Math.round(spot.x * 100) / 100, y: Math.round(y * 100) / 100, label: "" }], ps);
    setWall(target);
    setSelected(a.id);
  }

  const autoHang = useMutation({
    mutationFn: (ids: number[]) => api<{ placements: Placement[] }>(`/api/exhibitions/${id}/autohang`, { method: "POST", json: { artworkIds: ids } }),
    onSuccess: (r) => {
      if (ps) commit(r.placements, ps);
      setNotice(`Hung ${count(r.placements.length, "work", "works")}. Drag any of them to adjust.`);
    },
    onError: (err) => setNotice((err as ApiError).message),
  });

  const patch = useMutation({
    mutationFn: (body: Record<string, unknown>) => api<ExhibitionDetail>(`/api/exhibitions/${id}`, { method: "PATCH", json: body }),
    onSuccess: (d) => qc.setQueryData(["exhibition", id], (old: ExhibitionDetail | undefined) => (old ? { ...old, exhibition: d.exhibition } : d)),
    onError: (err) => setNotice((err as ApiError).message),
  });

  const publish = useMutation({
    mutationFn: (on: boolean) => api<ExhibitionDetail>(`/api/exhibitions/${id}/${on ? "publish" : "unpublish"}`, { method: "POST" }),
    onSuccess: (d, on) => {
      qc.setQueryData(["exhibition", id], (old: ExhibitionDetail | undefined) => (old ? { ...old, exhibition: d.exhibition } : d));
      if (on) setOpened(true);
    },
    onError: (err) => setNotice((err as ApiError).message),
  });

  const remove = useMutation({
    mutationFn: () => api(`/api/exhibitions/${id}`, { method: "DELETE" }),
    onSuccess: () => router.push("/studio"),
  });

  // Keyboard: nudge, take down, undo and redo.
  useEffect(() => {
    const onKey = (ev: KeyboardEvent) => {
      const t = ev.target as HTMLElement;
      if (t.closest("input, textarea, select")) return;
      if (!ps) return;
      const mod = ev.ctrlKey || ev.metaKey;
      if (mod && ev.key.toLowerCase() === "z") {
        ev.preventDefault();
        const h = history.current;
        const back = ev.shiftKey ? h.future.pop() : h.past.pop();
        if (!back) return;
        (ev.shiftKey ? h.past : h.future).push(ps);
        setPs(back);
        persist(back);
        return;
      }
      if (selected === null) return;
      const step = ev.shiftKey ? 0.01 : 0.05;
      const d = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, step], ArrowDown: [0, -step] }[ev.key];
      if (d) {
        ev.preventDefault();
        commit(ps.map((p) => (p.artworkId === selected ? { ...p, x: +(p.x + d[0]!).toFixed(2), y: +(p.y + d[1]!).toFixed(2) } : p)), ps);
      } else if (ev.key === "Delete" || ev.key === "Backspace") {
        commit(ps.filter((p) => p.artworkId !== selected), ps);
        setSelected(null);
      } else if (ev.key === "Escape") {
        setSelected(null);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [ps, selected, commit, persist]);

  if (detail.isError) {
    const err = detail.error as ApiError;
    return (
      <div className="grid min-h-svh place-items-center p-6 text-center">
        <div>
          <p className="lettering text-[44px]">{err.status === 401 ? "Sign in to open the studio." : "This exhibition isn't yours to edit."}</p>
          <Link href={err.status === 401 ? `/signin?next=/studio/${id}` : "/studio"} className="btn btn-solid mt-6">
            {err.status === 401 ? "Sign in" : "Your exhibitions"}
          </Link>
        </div>
      </div>
    );
  }
  if (!e || !room || !rooms.data || !ps) {
    return <div className="grid min-h-svh place-items-center text-black/50">Opening the studio…</div>;
  }

  const sel = ps.find((p) => p.artworkId === selected);
  const selWork = sel ? known.get(sel.artworkId) : undefined;
  const hung = new Set(ps.map((p) => p.artworkId));
  const problemList = [...problems.entries()];
  const perWall = [0, 1, 2, 3].map((w) => ps.filter((p) => p.wall === w).length);

  return (
    <div className="room room-light flex h-svh flex-col [--wall:#e9e7e1]">
      <header className="flex flex-wrap items-center gap-3 border-b border-black/10 bg-[#f4f2ed] px-4 py-2.5">
        <Link href="/studio" className="btn btn-quiet text-[14px] hover:bg-black/5" aria-label="Your exhibitions">
          ←
        </Link>
        <input
          defaultValue={e.title}
          key={e.title}
          onBlur={(ev) => ev.target.value.trim() && ev.target.value !== e.title && patch.mutate({ title: ev.target.value })}
          onKeyDown={(ev) => ev.key === "Enter" && (ev.target as HTMLInputElement).blur()}
          maxLength={90}
          aria-label="Exhibition title"
          className="lettering-sm min-w-0 flex-1 rounded-md bg-transparent px-2 py-1 text-[26px] hover:bg-black/5 focus:bg-white focus:outline-none"
        />
        <span className={`rounded-full px-3 py-1 text-[13px] ${e.status === "published" ? "bg-emerald-700 text-white" : "bg-black/10"}`}>
          {e.status === "published" ? "Open" : "Draft"}
        </span>
        <span className="min-w-[90px] text-[13px] text-black/55" aria-live="polite">
          {save.state === "saving" ? "Saving…" : save.state === "error" ? <span className="text-[#b3261e]">{save.message}</span> : "All changes saved"}
        </span>
        <Link href={`/e/${e.slug}/walk`} className="btn border border-black/20 text-[14px] hover:border-black">
          Walk through it
        </Link>
        {e.status === "published" ? (
          <>
            <Link href={`/e/${e.slug}`} className="btn border border-black/20 text-[14px] hover:border-black">
              Public page
            </Link>
            <button className="btn text-[14px] hover:bg-black/5" onClick={() => publish.mutate(false)}>
              Close the doors
            </button>
          </>
        ) : (
          <button className="btn bg-[#1d1a16] text-[14px] text-white hover:bg-black" disabled={publish.isPending || ps.length === 0} onClick={() => publish.mutate(true)}>
            Open the doors
          </button>
        )}
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[260px_minmax(0,1fr)_340px]">
        <aside className="hidden min-h-0 border-r border-black/10 bg-[#f4f2ed] lg:block">
          <Tray hung={hung} extra={extra} onAdd={(a) => place(a)} onHangAll={(ids) => autoHang.mutate([...hung, ...ids].slice(0, rooms.data!.rules.maxWorks))} busy={autoHang.isPending} register={register} />
        </aside>

        <section className="flex min-h-0 flex-col">
          <div className="flex flex-wrap items-center gap-4 px-5 pt-4">
            <Plan room={room} wall={wall} onWall={setWall} placements={ps} works={known} />
            <div role="tablist" aria-label="Walls" className="flex flex-wrap gap-1">
              {WALL_NAMES.map((n, w) => (
                <button key={w} role="tab" aria-selected={wall === w} onClick={() => setWall(w)} className="rounded-full px-3.5 py-2 text-[14px] hover:bg-black/5 aria-selected:bg-[#1d1a16] aria-selected:text-white">
                  {n}
                  <span className="ml-1.5 opacity-60">{perWall[w]}</span>
                </button>
              ))}
            </div>
            <div className="ml-auto flex gap-2">
              <div role="radiogroup" aria-label="Zoom" className="flex rounded-full border border-black/15 p-0.5 text-[13px]">
                {[
                  { v: false, label: "Whole wall" },
                  { v: true, label: "Close up" },
                ].map((o) => (
                  <button key={o.label} role="radio" aria-checked={close === o.v} onClick={() => setClose(o.v)} className="rounded-full px-3 py-1.5 aria-checked:bg-black/80 aria-checked:text-white">
                    {o.label}
                  </button>
                ))}
              </div>
              <button
                className="btn border border-black/20 text-[14px] hover:border-black"
                disabled={autoHang.isPending || ps.length === 0}
                onClick={() => autoHang.mutate(ps.map((p) => p.artworkId))}
                title="Lay out the works already hung, the way a curator would start"
              >
                Rehang for me
              </button>
            </div>
          </div>
          <div className="min-h-0 flex-1 px-5">
            <Elevation
              room={room}
              close={close}
              rules={rooms.data.rules}
              wall={wall}
              paint={paint}
              placements={ps}
              works={known}
              problems={problems}
              selected={selected}
              onSelect={setSelected}
              onMove={move}
              onCommit={endDrag}
              onDrop={(aid, x, y) => {
                const a = known.get(aid);
                if (!a) return;
                if (hung.has(aid)) return;
                place(a, { x, y }, wall);
              }}
            />
          </div>
          {problemList.length > 0 && (
            <div className="mx-5 mb-4 rounded-xl bg-[#b3261e]/10 p-3 text-[14px] text-[#8c1d17]">
              {problemList.length === 1 ? "One work doesn't fit where it is." : `${problemList.length} works don't fit where they are.`} Move {problemList.length === 1 ? "it" : "them"}, or{" "}
              <button className="underline underline-offset-4" onClick={() => autoHang.mutate(ps.map((p) => p.artworkId))}>
                rehang everything for this room
              </button>
              .
            </div>
          )}
        </section>

        <aside className="min-h-0 overflow-y-auto border-l border-black/10 bg-[#f4f2ed]">
          {sel && selWork ? (
            <WorkSettings
              a={selWork}
              p={sel}
              problem={problems.get(sel.artworkId)}
              isCover={e.coverId === sel.artworkId}
              onLabel={(label) => {
                const next = ps.map((p) => (p.artworkId === sel.artworkId ? { ...p, label } : p));
                setPs(next);
                persist(next);
              }}
              onCover={() => patch.mutate({ coverId: sel.artworkId })}
              onMoveWall={(w) => {
                const rest = ps.filter((p) => p.artworkId !== sel.artworkId);
                const spot = freeSpot(room, rooms.data!.rules, w, rest, known, selWork.hang.w, selWork.hang.h);
                if (!spot) return setNotice(`The ${WALL_NAMES[w]!.toLowerCase()} is full.`);
                commit([...rest, { ...sel, wall: w, x: +spot.x.toFixed(2), y: +spot.y.toFixed(2) }], ps);
                setWall(w);
              }}
              onRemove={() => {
                commit(ps.filter((p) => p.artworkId !== sel.artworkId), ps);
                setSelected(null);
              }}
              onNudge={(x, y) => commit(ps.map((p) => (p.artworkId === sel.artworkId ? { ...p, x, y } : p)), ps)}
            />
          ) : (
            <RoomSettings
              e={e}
              rooms={rooms.data}
              onChange={(body) => patch.mutate(body)}
              onDelete={() => {
                if (window.confirm(`Delete “${e.title}”? This can't be undone.`)) remove.mutate();
              }}
            />
          )}
        </aside>
      </div>

      <AnimatePresence>
        {notice && <Toast key={notice} text={notice} onDone={() => setNotice(null)} />}
        {opened && (
          <motion.div className="fixed inset-0 z-50 grid place-items-center bg-black/50 p-4" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={() => setOpened(false)}>
            <motion.div
              className="w-full max-w-md rounded-2xl p-7 text-[#f2ede3] shadow-2xl"
              style={{ background: paint }}
              initial={{ y: 20 }}
              animate={{ y: 0 }}
              onClick={(ev) => ev.stopPropagation()}
              role="dialog"
              aria-label="The doors are open"
            >
              <p className="lettering text-[40px]">The doors are open.</p>
              <p className="mt-3 text-white/80">Anyone with the link can walk in now. Send it to the people you&apos;d like to see there.</p>
              <div className="mt-6 flex flex-wrap gap-2">
                <button
                  className="btn bg-[#f2ede3] text-[#1d1a16]"
                  onClick={() => navigator.clipboard?.writeText(`${window.location.origin}/e/${e.slug}`).then(() => setNotice("Link copied."))}
                >
                  Copy the link
                </button>
                <Link href={`/e/${e.slug}/walk`} className="btn border border-white/40">
                  Walk in
                </Link>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function Toast({ text, onDone }: { text: string; onDone: () => void }) {
  useEffect(() => {
    const t = setTimeout(onDone, 3800);
    return () => clearTimeout(t);
  }, [onDone]);
  return (
    <motion.p
      role="status"
      className="fixed bottom-6 left-1/2 z-50 max-w-[90vw] -translate-x-1/2 rounded-full bg-[#1d1a16] px-5 py-3 text-[14px] text-white shadow-2xl"
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0 }}
    >
      {text}
    </motion.p>
  );
}
