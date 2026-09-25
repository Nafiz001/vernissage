"use client";

import { Text } from "@react-three/drei";
import { useFrame, type ThreeEvent } from "@react-three/fiber";
import { useEffect, useMemo, useRef, useState } from "react";
import * as THREE from "three";
import type { Artwork, Placement, Room } from "@/lib/types";
import { wallPoint } from "./geometry";
import type { Lighting } from "./Room";
import { motion as live, useWalk } from "./store";
import { washTexture } from "./textures";

const loader = new THREE.TextureLoader();
loader.setCrossOrigin("anonymous");
const textures = new Map<string, Promise<THREE.Texture>>();

function load(url: string) {
  let p = textures.get(url);
  if (!p) {
    p = loader.loadAsync(url).then((t) => {
      t.colorSpace = THREE.SRGBColorSpace;
      t.anisotropy = 8;
      return t;
    });
    textures.set(url, p);
  }
  return p;
}

/**
 * A picture's texture, sharpened as you approach: a small one at once, the
 * large one when you're within a few metres.
 */
function usePicture(id: number, near: boolean) {
  const [tex, setTex] = useState<THREE.Texture | null>(null);
  const [sharp, setSharp] = useState(false);
  useEffect(() => {
    let alive = true;
    load(`/img/${id}/400.jpg`).then((t) => alive && !sharp && setTex(t));
    return () => {
      alive = false;
    };
  }, [id]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!near || sharp) return;
    let alive = true;
    load(`/img/${id}/1200.jpg`).then((t) => {
      if (!alive) return;
      setTex(t);
      setSharp(true);
    });
    return () => {
      alive = false;
    };
  }, [id, near, sharp]);
  return tex;
}

const frameMats: Record<string, () => THREE.MeshStandardMaterial> = {
  gilt: () => new THREE.MeshStandardMaterial({ color: "#c9a256", metalness: 0.92, roughness: 0.3 }),
  oak: () => new THREE.MeshStandardMaterial({ color: "#8e6a44", metalness: 0, roughness: 0.55 }),
  black: () => new THREE.MeshStandardMaterial({ color: "#121110", metalness: 0.1, roughness: 0.32 }),
  silk: () => new THREE.MeshStandardMaterial({ color: "#c9b893", metalness: 0, roughness: 0.92 }),
};

/** A frame moulding: a rectangle with a rectangular hole, extruded and bevelled. */
function frameGeometry(w: number, h: number, border: number, depth: number) {
  const shape = new THREE.Shape();
  shape.moveTo(-w / 2, -h / 2);
  shape.lineTo(w / 2, -h / 2);
  shape.lineTo(w / 2, h / 2);
  shape.lineTo(-w / 2, h / 2);
  shape.closePath();
  const hole = new THREE.Path();
  const iw = w / 2 - border;
  const ih = h / 2 - border;
  hole.moveTo(-iw, -ih);
  hole.lineTo(-iw, ih);
  hole.lineTo(iw, ih);
  hole.lineTo(iw, -ih);
  hole.closePath();
  shape.holes.push(hole);
  const bevel = Math.min(border * 0.42, depth * 0.8);
  const g = new THREE.ExtrudeGeometry(shape, {
    depth: Math.max(0.004, depth - bevel),
    bevelEnabled: true,
    bevelThickness: bevel,
    bevelSize: bevel * 0.9,
    bevelOffset: -bevel * 0.9,
    bevelSegments: 4,
    curveSegments: 1,
  });
  return g;
}

function Piece({
  room,
  p,
  a,
  light,
  labelRoom,
}: {
  room: Room;
  p: Placement;
  a: Artwork;
  light: Lighting;
  labelRoom: boolean;
}) {
  const f = wallPoint(room, p.wall, p.x);
  const h = a.hang;
  const [near, setNear] = useState(false);
  const tex = usePicture(a.id, near);
  const hover = useRef(false);
  const viewing = useWalk((s) => s.viewing === a.id);
  const view = useWalk((s) => s.view);
  const depth = h.frame === "gilt" ? 0.07 : 0.045;

  const geom = useMemo(() => frameGeometry(h.w, h.h, h.border, depth), [h.w, h.h, h.border, depth]);
  const frameMat = useMemo(() => (frameMats[h.frame] ?? frameMats.gilt)(), [h.frame]);
  const artMat = useMemo(
    () => new THREE.MeshStandardMaterial({ color: "#ffffff", roughness: 0.78, emissive: "#ffffff", emissiveIntensity: light.emissive }),
    [light.emissive],
  );
  useEffect(() => {
    if (!tex) return;
    artMat.map = tex;
    artMat.emissiveMap = tex;
    artMat.needsUpdate = true;
  }, [tex, artMat]);
  useEffect(() => () => geom.dispose(), [geom]);

  // Sharpen when close; checked twice a second.
  const tick = useRef(0);
  useFrame((_, dt) => {
    tick.current += dt;
    if (tick.current < 0.5) return;
    tick.current = 0;
    const d = Math.hypot(live.me.x - f.x, live.me.z - f.z);
    if (!near && d < 7) setNear(true);
  });

  const onClick = (e: ThreeEvent<MouseEvent>) => {
    e.stopPropagation();
    if ((e as unknown as { delta: number }).delta > 6) return;
    view(a.id);
  };

  // Everything below is in the wall's own frame: x along the wall, y up,
  // z out into the room.
  return (
    <group position={[f.x, 0, f.z]} rotation-y={f.rotY}>
      <group position={[0, p.y, 0]}>
        {/* The light pool on the wall. */}
        <mesh position={[0, h.h * 0.1, 0.004]} renderOrder={1}>
          <planeGeometry args={[h.w * 1.45 + 0.35, h.h * 1.6 + 0.45]} />
          <meshBasicMaterial map={washTexture()} transparent opacity={light.wash} depthWrite={false} blending={THREE.AdditiveBlending} toneMapped={false} />
        </mesh>
        <mesh geometry={geom} material={frameMat} position={[0, 0, 0.006]} castShadow />
        {h.mat > 0 && (
          <mesh position={[0, 0, 0.012]}>
            <planeGeometry args={[h.w - h.border * 2 + 0.004, h.h - h.border * 2 + 0.004]} />
            <meshStandardMaterial color="#f3efe5" roughness={0.9} emissive="#f3efe5" emissiveIntensity={light.emissive * 0.5} />
          </mesh>
        )}
        <mesh
          position={[0, 0, 0.016]}
          material={artMat}
          onClick={onClick}
          onPointerOver={(e) => {
            e.stopPropagation();
            hover.current = true;
            document.body.style.cursor = "zoom-in";
          }}
          onPointerOut={() => {
            hover.current = false;
            document.body.style.cursor = "";
          }}
          userData={{ artworkId: a.id }}
        >
          <planeGeometry args={[h.artW, h.artH]} />
        </mesh>
        {viewing && (
          <mesh position={[0, 0, 0.002]}>
            <planeGeometry args={[h.w + 0.06, h.h + 0.06]} />
            <meshBasicMaterial color="#fff6e0" transparent opacity={0.12} toneMapped={false} />
          </mesh>
        )}
      </group>
      {labelRoom && <Label a={a} x={h.w / 2 + 0.2} y={Math.min(1.36, p.y)} />}
    </group>
  );
}

/** The small card beside a work, readable when you walk up to it. */
function Label({ a, x, y }: { a: Artwork; x: number; y: number }) {
  const lines = [a.artist || "Unknown artist", a.title.length > 60 ? a.title.slice(0, 58) + "…" : a.title, a.date].filter(Boolean);
  return (
    <group position={[x + 0.08, y, 0.006]}>
      <mesh>
        <boxGeometry args={[0.17, 0.1, 0.004]} />
        <meshStandardMaterial color="#fbfaf6" roughness={0.9} />
      </mesh>
      <Text
        font="/fonts/Archivo-Regular.ttf"
        position={[-0.072, 0.036, 0.003]}
        fontSize={0.0105}
        lineHeight={1.35}
        maxWidth={0.145}
        anchorX="left"
        anchorY="top"
        color="#1d1a16"
      >
        {lines.join("\n")}
      </Text>
    </group>
  );
}

export function Artworks({ room, placements, works, light }: { room: Room; placements: Placement[]; works: Map<number, Artwork>; light: Lighting }) {
  // A label goes to the right of a work only if the wall has room for it.
  const labelled = useMemo(() => {
    const ok = new Set<number>();
    for (const p of placements) {
      const a = works.get(p.artworkId);
      if (!a) continue;
      const right = p.x + a.hang.w / 2;
      const next = placements
        .filter((q) => q.wall === p.wall && q.x > p.x)
        .map((q) => q.x - (works.get(q.artworkId)?.hang.w ?? 0) / 2)
        .reduce((m, v) => Math.min(m, v), Infinity);
      const wallEnd = p.wall % 2 === 0 ? room.width : room.depth;
      let limit = Math.min(next, wallEnd);
      if (p.wall === 2 && right < room.width / 2) limit = Math.min(limit, room.width / 2 - room.door.width / 2);
      if (limit - right > 0.42) ok.add(p.artworkId);
    }
    return ok;
  }, [placements, works, room]);

  return (
    <group>
      {placements.map((p) => {
        const a = works.get(p.artworkId);
        return a ? <Piece key={a.id} room={room} p={p} a={a} light={light} labelRoom={labelled.has(a.id)} /> : null;
      })}
    </group>
  );
}
