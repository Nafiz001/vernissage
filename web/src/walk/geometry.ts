import type { Placement, Room } from "@/lib/types";

// The room's coordinates match the server's: centred on the origin, x
// across the width, z across the depth, y up. Wall 0 is the far (north)
// wall at z = -depth/2, then clockwise: 1 east, 2 south with the door, 3
// west. A position on a wall is measured from its left end as you face it.

export const EYE = 1.62;
export const WALL_T = 0.24; // wall thickness
export const VESTIBULE = 4.6; // depth of the entrance hall beyond the door

export interface WallFrame {
  /** Point on the wall's surface, inside the room. */
  x: number;
  z: number;
  /** Unit normal pointing into the room. */
  nx: number;
  nz: number;
  /** Rotation about y that turns a +z-facing plane to face into the room. */
  rotY: number;
}

export function wallPoint(room: Room, wall: number, u: number): WallFrame {
  const hw = room.width / 2;
  const hd = room.depth / 2;
  switch (wall) {
    case 0:
      return { x: -hw + u, z: -hd, nx: 0, nz: 1, rotY: 0 };
    case 1:
      return { x: hw, z: -hd + u, nx: -1, nz: 0, rotY: -Math.PI / 2 };
    case 2:
      return { x: hw - u, z: hd, nx: 0, nz: -1, rotY: Math.PI };
    default:
      return { x: -hw, z: hd - u, nx: 1, nz: 0, rotY: Math.PI / 2 };
  }
}

/** Yaw (about y) of a camera looking along direction (dx, dz). Yaw 0 looks toward -z. */
export function yawOf(dx: number, dz: number) {
  return Math.atan2(-dx, -dz);
}

export interface Pose {
  x: number;
  z: number;
  yaw: number;
  pitch: number;
}

export const FOV = 62;
/** Width of the label panel beside a work being looked at, on wide screens. */
export const PANEL = 410;
/** On narrow screens the panel is a sheet covering this share of the height. */
export const SHEET = 0.48;
export const WIDE = 900;

/**
 * How much of the view the label panel leaves visible, as widths and
 * heights in tangent units (a work of height h at distance d spans h/d).
 * On a wide screen the panel sits to the right and the picture shifts
 * left; on a phone it's a sheet along the bottom and the picture shifts
 * up, which also narrows the field of view by the sheet's share.
 */
export function visibleView(width: number, height: number) {
  const full = 2 * Math.tan(((FOV / 2) * Math.PI) / 180); // tangent height of the whole view
  if (width > WIDE) return { w: (full * (width - PANEL)) / height, h: full };
  const scale = full / ((1 + SHEET) * height); // tangent units per pixel
  return { w: width * scale, h: (1 - SHEET) * height * scale };
}

/**
 * Where to stand to look at a work: far enough that the whole frame, with
 * a little wall round it, fits the part of the screen not covered by the
 * label panel, and never further than the room allows.
 */
export function viewingPose(room: Room, p: Placement, w: number, h: number, view = { w: 1.9, h: 1.2 }): Pose {
  const f = wallPoint(room, p.wall, p.x);
  const fitH = (h * 1.35) / view.h;
  const fitW = (w * 1.2) / view.w;
  const across = p.wall % 2 === 0 ? room.depth : room.width;
  const dist = Math.min(Math.max(fitH, fitW, 1.2), across - 0.9);
  const x = f.x + f.nx * dist;
  const z = f.z + f.nz * dist;
  return { x, z, yaw: yawOf(-f.nx, -f.nz), pitch: Math.atan2(p.y - EYE, dist) * 0.85 };
}

export interface Rect {
  x0: number;
  x1: number;
  z0: number;
  z1: number;
}

const inside = (r: Rect, x: number, z: number) => x >= r.x0 && x <= r.x1 && z >= r.z0 && z <= r.z1;

/**
 * Where a visitor may stand: the room, the entrance hall and the doorway
 * between them, less the benches.
 */
export function walkable(room: Room) {
  const m = 0.35;
  const hw = room.width / 2;
  const hd = room.depth / 2;
  const hall = Math.min(3.6, hw);
  const open: Rect[] = [
    { x0: -hw + m, x1: hw - m, z0: -hd + m, z1: hd - m },
    { x0: -hall + m, x1: hall - m, z0: hd + WALL_T + m, z1: hd + VESTIBULE - m },
    { x0: -room.door.width / 2 + m, x1: room.door.width / 2 - m, z0: hd - m - 0.01, z1: hd + WALL_T + m + 0.01 },
  ];
  const blocked: Rect[] = (room.benches ?? []).map((b) => ({
    x0: b.x - b.width / 2 - 0.3,
    x1: b.x + b.width / 2 + 0.3,
    z0: b.z - b.depth / 2 - 0.3,
    z1: b.z + b.depth / 2 + 0.3,
  }));
  const ok = (x: number, z: number) => open.some((r) => inside(r, x, z)) && !blocked.some((r) => inside(r, x, z));
  return {
    ok,
    /** Moves from (px, pz) toward (x, z) as far as the walls allow, sliding along them. */
    step(px: number, pz: number, x: number, z: number): [number, number] {
      if (ok(x, z)) return [x, z];
      if (ok(x, pz)) return [x, pz];
      if (ok(px, z)) return [px, z];
      return [px, pz];
    },
  };
}

/** The entrance hall's half-width. */
export function hallHalf(room: Room) {
  return Math.min(3.6, room.width / 2);
}

/** Visiting order: clockwise from the far wall, left to right along each. */
export function tourOrder(placements: Placement[]) {
  return [...placements].sort((a, b) => a.wall - b.wall || a.x - b.x);
}
