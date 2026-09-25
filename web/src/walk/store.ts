import { create } from "zustand";
import type { GuestbookEntry } from "@/lib/types";
import type { Pose } from "./geometry";

export type Quality = "high" | "medium" | "low";

export interface Peer {
  id: string;
  name: string;
  hue: number;
  member: boolean;
  looking?: number;
}

export interface Reaction {
  key: number;
  peer: string;
  e: string;
}

/**
 * Fast-changing positions live outside React: the scene reads and writes
 * them every frame, and the minimap samples them a few times a second.
 */
export const motion = {
  me: { x: 0, z: 0, yaw: 0, pitch: 0 } as Pose,
  moving: false,
  peers: new Map<string, { x: number; z: number; yaw: number; tx: number; tz: number; tyaw: number }>(),
};

interface WalkState {
  entered: boolean;
  quality: Quality;
  sound: boolean;
  viewing: number | null;
  /** A pose the camera should glide to. */
  glide: (Pose & { key: number }) | null;
  touring: boolean;
  me: Peer | null;
  peers: Record<string, Peer>;
  reactions: Reaction[];
  guestbook: GuestbookEntry[];
  status: "connecting" | "open" | "closed";
  notice: string | null;

  enter: () => void;
  setQuality: (q: Quality) => void;
  setSound: (on: boolean) => void;
  view: (id: number | null) => void;
  glideTo: (p: Pose) => void;
  setTouring: (on: boolean) => void;
  react: (r: Omit<Reaction, "key">) => void;
  set: (s: Partial<WalkState>) => void;
}

let reactionKey = 0;

export const useWalk = create<WalkState>((set) => ({
  entered: false,
  quality: "high",
  sound: false,
  viewing: null,
  glide: null,
  touring: false,
  me: null,
  peers: {},
  reactions: [],
  guestbook: [],
  status: "connecting",
  notice: null,

  enter: () => set({ entered: true }),
  setQuality: (quality) => set({ quality }),
  setSound: (sound) => set({ sound }),
  view: (viewing) => set({ viewing }),
  glideTo: (p) => set({ glide: { ...p, key: Date.now() + Math.random() } }),
  setTouring: (touring) => set({ touring }),
  react: (r) => {
    const key = ++reactionKey;
    set((s) => ({ reactions: [...s.reactions.slice(-30), { ...r, key }] }));
    setTimeout(() => set((s) => ({ reactions: s.reactions.filter((x) => x.key !== key) })), 2600);
  },
  set: (s) => set(s),
}));
