import type { Artwork, Placement, Problem, Room, Rooms } from "@/lib/types";

// The same checks the server makes (internal/gallery/layout.go), run on
// every drag so a frame turns red the moment it stops fitting. The server
// still decides what's saved.

export function wallLength(room: Room, wall: number) {
  return wall % 2 === 0 ? room.width : room.depth;
}

export function doorSpan(room: Room): [number, number] {
  const mid = room.width / 2;
  return [mid - room.door.width / 2, mid + room.door.width / 2];
}

export function validate(room: Room, rules: Rooms["rules"], ps: Placement[], works: Map<number, Artwork>): Problem[] {
  const out: Problem[] = [];
  const eps = 1e-6;
  const boxes: { id: number; wall: number; x0: number; x1: number; y0: number; y1: number }[] = [];
  const seen = new Set<number>();
  if (ps.length > rules.maxWorks) out.push({ artworkId: 0, message: `An exhibition can hang up to ${rules.maxWorks} works.` });
  for (const p of ps) {
    const a = works.get(p.artworkId);
    if (!a) continue;
    if (seen.has(p.artworkId)) {
      out.push({ artworkId: p.artworkId, message: "This work is hung twice." });
      continue;
    }
    seen.add(p.artworkId);
    const b = { id: p.artworkId, wall: p.wall, x0: p.x - a.hang.w / 2, x1: p.x + a.hang.w / 2, y0: p.y - a.hang.h / 2, y1: p.y + a.hang.h / 2 };
    let inside = b.x0 >= -eps && b.x1 <= wallLength(room, p.wall) + eps;
    if (p.wall === 2) {
      const [d0, d1] = doorSpan(room);
      inside = inside && (b.x1 <= d0 + eps || b.x0 >= d1 - eps);
    }
    if (!inside) {
      out.push({ artworkId: p.artworkId, message: p.wall === 2 ? "This work runs into the doorway or off the end of the wall." : "This work runs off the end of the wall." });
      continue;
    }
    if (b.y0 < rules.floorClear - eps || b.y1 > room.height - rules.ceilingClear + eps) {
      out.push({
        artworkId: p.artworkId,
        message:
          a.hang.h > room.height - rules.floorClear - rules.ceilingClear
            ? `This work is too tall for the ${room.name}; try a taller room.`
            : "This work hangs too close to the floor or the ceiling.",
      });
      continue;
    }
    boxes.push(b);
  }
  const g = rules.minGap;
  for (let i = 0; i < boxes.length; i++) {
    for (let j = i + 1; j < boxes.length; j++) {
      const a = boxes[i]!;
      const b = boxes[j]!;
      if (a.wall === b.wall && a.x0 < b.x1 + g && b.x0 < a.x1 + g && a.y0 < b.y1 + g && b.y0 < a.y1 + g) {
        out.push({ artworkId: b.id, message: "This work overlaps another frame." });
      }
    }
  }
  return out;
}

/**
 * Where a newly added work could go on a wall: the centre of the widest
 * empty stretch, on the eye line.
 */
export function freeSpot(room: Room, rules: Rooms["rules"], wall: number, ps: Placement[], works: Map<number, Artwork>, w: number, h: number): { x: number; y: number } | null {
  const L = wallLength(room, wall);
  const taken: [number, number][] = ps
    .filter((p) => p.wall === wall)
    .map((p) => {
      const a = works.get(p.artworkId);
      const half = (a?.hang.w ?? 0) / 2;
      return [p.x - half - rules.minGap * 2, p.x + half + rules.minGap * 2] as [number, number];
    });
  if (wall === 2) {
    const [d0, d1] = doorSpan(room);
    taken.push([d0 - rules.minGap, d1 + rules.minGap]);
  }
  taken.sort((a, b) => a[0] - b[0]);
  let best: [number, number] | null = null;
  let cursor = rules.corner;
  for (const [s, e] of [...taken, [L - rules.corner, L + 1] as [number, number]]) {
    if (s - cursor > (best ? best[1] - best[0] : 0)) best = [cursor, s];
    cursor = Math.max(cursor, e);
  }
  if (!best || best[1] - best[0] < w) return null;
  const y = Math.min(Math.max(rules.eyeLine, rules.floorClear + h / 2 + 0.2), room.height - rules.ceilingClear - h / 2);
  return { x: (best[0] + best[1]) / 2, y };
}
