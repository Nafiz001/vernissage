"use client";

import { useEffect, useRef } from "react";
import { api, wsURL } from "@/lib/api";
import type { GuestbookEntry } from "@/lib/types";
import { motion as live, useWalk, type Peer } from "./store";

type PeerInfo = Peer & { x: number; z: number; yaw: number };

type Inbound =
  | { t: "welcome"; you: PeerInfo; peers: PeerInfo[]; guestbook: GuestbookEntry[] }
  | { t: "join"; peer: PeerInfo }
  | { t: "leave"; id: string }
  | { t: "poses"; d: [string, number, number, number][] }
  | { t: "react"; id: string; e: string }
  | { t: "look"; id: string; a: number }
  | { t: "rename"; id: string; name: string }
  | { t: "signed"; entry: GuestbookEntry; id: string }
  | { t: "error"; message: string };

function place(p: PeerInfo) {
  live.peers.set(p.id, { x: p.x, z: p.z, yaw: p.yaw, tx: p.x, tz: p.z, tyaw: p.yaw });
}

/**
 * The room's live connection: sends where you are ten times a second when
 * you move, and keeps everyone else's positions, reactions and signatures
 * up to date. Reconnects with backoff if the connection drops.
 */
export function useLive(slug: string) {
  const socket = useRef<WebSocket | null>(null);
  const set = useWalk((s) => s.set);

  useEffect(() => {
    let closed = false;
    let retry = 0;
    let timer: ReturnType<typeof setTimeout>;
    let last = { x: NaN, z: NaN, yaw: NaN };

    const connect = async () => {
      set({ status: "connecting" });
      // The socket may be on another domain than the page (Vercel and
      // Render), where the browser won't send our cookies; a short-lived
      // ticket from the same-origin API says who we are instead.
      let ticket = "";
      try {
        ticket = (await api<{ ticket: string }>("/api/live-ticket")).ticket;
      } catch {}
      if (closed) return;
      const ws = new WebSocket(wsURL(`/ws/exhibitions/${encodeURIComponent(slug)}?ticket=${encodeURIComponent(ticket)}`));
      socket.current = ws;
      ws.onopen = () => {
        retry = 0;
        set({ status: "open" });
        const saved = localStorage.getItem("vernissage.name");
        if (saved) ws.send(JSON.stringify({ t: "name", name: saved }));
      };
      ws.onmessage = (ev) => {
        const m = JSON.parse(ev.data) as Inbound;
        const s = useWalk.getState();
        switch (m.t) {
          case "welcome":
            live.peers.clear();
            m.peers.forEach(place);
            set({
              me: m.you,
              peers: Object.fromEntries(m.peers.map((p) => [p.id, p])),
              guestbook: m.guestbook,
            });
            break;
          case "join":
            place(m.peer);
            set({ peers: { ...s.peers, [m.peer.id]: m.peer } });
            break;
          case "leave": {
            live.peers.delete(m.id);
            const { [m.id]: _gone, ...rest } = s.peers;
            void _gone;
            set({ peers: rest });
            break;
          }
          case "poses":
            for (const [id, x, z, yaw] of m.d) {
              if (id === s.me?.id) continue;
              const p = live.peers.get(id);
              if (p) Object.assign(p, { tx: x, tz: z, tyaw: yaw });
            }
            break;
          case "react":
            s.react({ peer: m.id, e: m.e });
            break;
          case "look":
            if (s.peers[m.id]) set({ peers: { ...s.peers, [m.id]: { ...s.peers[m.id]!, looking: m.a || undefined } } });
            break;
          case "rename":
            if (m.id === s.me?.id && s.me) set({ me: { ...s.me, name: m.name } });
            else if (s.peers[m.id]) set({ peers: { ...s.peers, [m.id]: { ...s.peers[m.id]!, name: m.name } } });
            break;
          case "signed":
            set({ guestbook: [m.entry, ...s.guestbook] });
            break;
          case "error":
            set({ notice: m.message });
            break;
        }
      };
      ws.onclose = () => {
        socket.current = null;
        if (closed) return;
        set({ status: "closed" });
        retry++;
        timer = setTimeout(connect, Math.min(15000, 600 * 2 ** retry));
      };
    };
    connect();

    // Tell the room where we are, but only when that changes.
    const pose = setInterval(() => {
      const ws = socket.current;
      const me = live.me;
      if (ws?.readyState !== WebSocket.OPEN) return;
      if (Math.abs(me.x - last.x) < 0.02 && Math.abs(me.z - last.z) < 0.02 && Math.abs(me.yaw - last.yaw) < 0.02) return;
      last = { x: me.x, z: me.z, yaw: me.yaw };
      ws.send(JSON.stringify({ t: "pose", p: [me.x, me.z], r: me.yaw }));
    }, 100);

    return () => {
      closed = true;
      clearTimeout(timer);
      clearInterval(pose);
      socket.current?.close();
    };
  }, [slug, set]);

  const send = (msg: object) => {
    const ws = socket.current;
    if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
  };
  return {
    react: (e: string) => {
      send({ t: "react", e });
      const me = useWalk.getState().me;
      if (me) useWalk.getState().react({ peer: me.id, e });
    },
    look: (a: number | null) => send({ t: "look", a: a ?? 0 }),
    sign: (m: string) => send({ t: "sign", m }),
    rename: (name: string) => {
      localStorage.setItem("vernissage.name", name);
      send({ t: "name", name });
    },
  };
}
