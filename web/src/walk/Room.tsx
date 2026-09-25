"use client";

import { MeshReflectorMaterial, Text } from "@react-three/drei";
import { useMemo } from "react";
import * as THREE from "three";
import { hexToLab, labToHex } from "@/lib/color";
import type { Exhibition, Room as RoomSpec } from "@/lib/types";
import { hallHalf, VESTIBULE, WALL_T } from "./geometry";
import type { Quality } from "./store";
import { blobTexture, floorTexture, plasterBump } from "./textures";

export interface Lighting {
  ambient: number;
  hemi: number;
  hemiSky: string;
  hemiGround: string;
  skylight: number;
  emissive: number;
  wash: number;
  exposure: number;
  bloom: number;
  reflect: number;
  spot: number;
}

export const LIGHTS: Record<string, Lighting> = {
  daylight: { ambient: 0.55, hemi: 1.1, hemiSky: "#f1f4f7", hemiGround: "#8a7a66", skylight: 1.4, emissive: 0.16, wash: 0.12, exposure: 1.0, bloom: 0.15, reflect: 0.5, spot: 14 },
  gallery: { ambient: 0.3, hemi: 0.7, hemiSky: "#f6efe4", hemiGround: "#5a4b3c", skylight: 0.5, emissive: 0.3, wash: 0.26, exposure: 1.0, bloom: 0.25, reflect: 0.7, spot: 26 },
  evening: { ambient: 0.1, hemi: 0.22, hemiSky: "#8d93b5", hemiGround: "#2a211a", skylight: 0.08, emissive: 0.5, wash: 0.42, exposure: 1.05, bloom: 0.55, reflect: 1.1, spot: 36 },
};

const FONT_SERIF = "/fonts/BodoniModa-Regular.ttf";
const FONT_ITALIC = "/fonts/BodoniModa-Italic.ttf";
const FONT_SANS = "/fonts/Archivo-Regular.ttf";

function shade(hex: string, dl: number) {
  const c = hexToLab(hex);
  return labToHex({ ...c, L: Math.max(0, Math.min(1, c.L + dl)) });
}

/** One wall as a box, its inner face on the plane given. */
function Slab({ w, h, x, y, z, rotY, mat }: { w: number; h: number; x: number; y: number; z: number; rotY: number; mat: THREE.Material }) {
  return (
    <mesh position={[x, y, z]} rotation-y={rotY} material={mat} receiveShadow>
      <boxGeometry args={[w, h, WALL_T]} />
    </mesh>
  );
}

export function RoomShell({ room, e, quality, light }: { room: RoomSpec; e: Exhibition; quality: Quality; light: Lighting }) {
  const { width: W, depth: D, height: H } = room;
  const hw = W / 2;
  const hd = D / 2;
  const hall = hallHalf(room);
  const dw = room.door.width;
  const dh = room.door.height;
  const paint = e.paintHex;
  const dark = hexToLab(paint).L < 0.55;

  const mats = useMemo(() => {
    const bump = plasterBump();
    bump.repeat.set(W / 3, H / 3);
    const wall = new THREE.MeshStandardMaterial({ color: paint, roughness: 0.94, bumpMap: bump, bumpScale: 0.35 });
    const hallWall = new THREE.MeshStandardMaterial({ color: shade(paint, -0.06), roughness: 0.95, bumpMap: bump, bumpScale: 0.35 });
    // A ceiling is lit by what bounces off the floor and walls; fake that.
    const ceiling = new THREE.MeshStandardMaterial({ color: "#ebe8e2", roughness: 0.96, emissive: "#ebe8e2", emissiveIntensity: light.ambient * 0.55 });
    const skirting = new THREE.MeshStandardMaterial({ color: shade(paint, dark ? -0.1 : -0.25), roughness: 0.6 });
    const trim = new THREE.MeshStandardMaterial({ color: "#f1eee8", roughness: 0.8 });
    return { wall, hallWall, ceiling, skirting, trim };
  }, [paint, W, H, light.ambient, dark]);

  const floor = useMemo(() => {
    const { map, metres } = floorTexture(e.floor);
    const t = map.clone();
    t.needsUpdate = true;
    t.repeat.set(W / metres, (D + VESTIBULE + 1) / metres);
    return t;
  }, [e.floor, W, D]);

  const floorLength = D + VESTIBULE + WALL_T;
  const floorZ = (-hd + hd + VESTIBULE + WALL_T) / 2;
  const reflect = quality !== "low" && e.floor !== "walnut";

  // South wall pieces around the door.
  const side = (W - dw) / 2;
  const doorWallZ = hd + WALL_T / 2;

  // Lettering above the door, facing the entrance hall.
  const titleSize = Math.min(0.36, (H - dh - 0.45) * 0.5, (hall * 2 - 0.8) / Math.max(8, e.title.length * 0.52));
  const ink = dark ? "#f2ede3" : "#1d1a16";

  return (
    <group>
      {/* Floor through the room, the doorway and the hall. */}
      <mesh rotation-x={-Math.PI / 2} position={[0, 0, floorZ]} receiveShadow>
        <planeGeometry args={[W, floorLength]} />
        {reflect ? (
          <MeshReflectorMaterial
            map={floor}
            resolution={quality === "high" ? 768 : 384}
            blur={[500, 120]}
            mixBlur={1}
            mixStrength={light.reflect * (e.floor === "marble" || e.floor === "concrete" ? 1.3 : 0.9)}
            mixContrast={1}
            mirror={0}
            roughness={e.floor === "marble" ? 0.45 : 0.7}
            metalness={0}
            depthScale={0}
            color="#ffffff"
          />
        ) : (
          <meshStandardMaterial map={floor} roughness={0.75} />
        )}
      </mesh>

      {/* Ceiling. */}
      <mesh rotation-x={Math.PI / 2} position={[0, H, 0]} material={mats.ceiling}>
        <planeGeometry args={[W, D]} />
      </mesh>
      {room.skylight && <Skylight w={W * 0.56} d={D * 0.5} h={H} strength={light.skylight} />}

      {/* Walls: far, east, west, and the south wall around the door. */}
      <Slab w={W + WALL_T * 2} h={H} x={0} y={H / 2} z={-hd - WALL_T / 2} rotY={0} mat={mats.wall} />
      <Slab w={D} h={H} x={hw + WALL_T / 2} y={H / 2} z={0} rotY={Math.PI / 2} mat={mats.wall} />
      <Slab w={D} h={H} x={-hw - WALL_T / 2} y={H / 2} z={0} rotY={Math.PI / 2} mat={mats.wall} />
      <Slab w={side + WALL_T} h={H} x={-(dw / 2 + side / 2) - WALL_T / 2} y={H / 2} z={doorWallZ} rotY={0} mat={mats.wall} />
      <Slab w={side + WALL_T} h={H} x={dw / 2 + side / 2 + WALL_T / 2} y={H / 2} z={doorWallZ} rotY={0} mat={mats.wall} />
      <Slab w={dw} h={H - dh} x={0} y={dh + (H - dh) / 2} z={doorWallZ} rotY={0} mat={mats.wall} />

      {/* Door casing. */}
      <mesh position={[-dw / 2 - 0.04, dh / 2, doorWallZ]} material={mats.trim}>
        <boxGeometry args={[0.08, dh, WALL_T + 0.04]} />
      </mesh>
      <mesh position={[dw / 2 + 0.04, dh / 2, doorWallZ]} material={mats.trim}>
        <boxGeometry args={[0.08, dh, WALL_T + 0.04]} />
      </mesh>
      <mesh position={[0, dh + 0.04, doorWallZ]} material={mats.trim}>
        <boxGeometry args={[dw + 0.16, 0.08, WALL_T + 0.04]} />
      </mesh>

      {/* Skirting and cornice round the room. */}
      <Trim room={room} mat={mats.skirting} y={0.07} h={0.14} depth={0.025} />
      <Trim room={room} mat={mats.trim} y={H - 0.09} h={0.18} depth={0.07} />

      {/* The entrance hall. */}
      <group>
        <mesh rotation-x={Math.PI / 2} position={[0, H, hd + WALL_T + VESTIBULE / 2]} material={mats.ceiling}>
          <planeGeometry args={[hall * 2, VESTIBULE]} />
        </mesh>
        <Slab w={hall * 2 + WALL_T * 2} h={H} x={0} y={H / 2} z={hd + WALL_T + VESTIBULE + WALL_T / 2} rotY={0} mat={mats.hallWall} />
        <Slab w={VESTIBULE} h={H} x={hall + WALL_T / 2} y={H / 2} z={hd + WALL_T + VESTIBULE / 2} rotY={Math.PI / 2} mat={mats.hallWall} />
        <Slab w={VESTIBULE} h={H} x={-hall - WALL_T / 2} y={H / 2} z={hd + WALL_T + VESTIBULE / 2} rotY={Math.PI / 2} mat={mats.hallWall} />
        <pointLight position={[0, H - 0.4, hd + WALL_T + VESTIBULE / 2]} intensity={light.spot * 0.6} distance={H * 3} decay={1.6} color="#ffe7c7" />

        {/* Title in vinyl above the door; the curator's statement on the side wall. */}
        <Text
          font={FONT_SERIF}
          position={[0, dh + 0.24 + titleSize * 0.34 + 0.1, hd + WALL_T + 0.003]}
          fontSize={titleSize}
          maxWidth={hall * 2 - 0.6}
          textAlign="center"
          anchorX="center"
          anchorY="bottom"
          color={ink}
          letterSpacing={-0.01}
        >
          {e.title}
        </Text>
        <Text
          font={FONT_ITALIC}
          position={[0, dh + 0.24, hd + WALL_T + 0.003]}
          fontSize={titleSize * 0.34}
          anchorX="center"
          anchorY="bottom"
          color={ink}
          fillOpacity={0.75}
        >
          {`Curated by ${e.owner.name}`}
        </Text>
        {e.statement && (
          <Text
            font={FONT_SANS}
            position={[-hall + 0.003, 1.72, hd + WALL_T + VESTIBULE / 2]}
            rotation-y={Math.PI / 2}
            fontSize={0.052}
            lineHeight={1.5}
            maxWidth={Math.min(2.6, VESTIBULE - 1)}
            anchorX="center"
            anchorY="middle"
            color={ink}
            fillOpacity={0.85}
          >
            {e.statement}
          </Text>
        )}
      </group>

      {(room.benches ?? []).map((b, i) => (
        <Bench key={i} x={b.x} z={b.z} w={b.width} d={b.depth} />
      ))}
    </group>
  );
}

/** A band round the room at a height: skirting or cornice. */
function Trim({ room, mat, y, h, depth }: { room: RoomSpec; mat: THREE.Material; y: number; h: number; depth: number }) {
  const hw = room.width / 2;
  const hd = room.depth / 2;
  const side = (room.width - room.door.width) / 2;
  const doorGap = y < room.door.height;
  return (
    <group>
      <mesh position={[0, y, -hd + depth / 2]} material={mat}>
        <boxGeometry args={[room.width, h, depth]} />
      </mesh>
      <mesh position={[hw - depth / 2, y, 0]} material={mat}>
        <boxGeometry args={[depth, h, room.depth]} />
      </mesh>
      <mesh position={[-hw + depth / 2, y, 0]} material={mat}>
        <boxGeometry args={[depth, h, room.depth]} />
      </mesh>
      {doorGap ? (
        <>
          <mesh position={[-(room.door.width / 2 + side / 2), y, hd - depth / 2]} material={mat}>
            <boxGeometry args={[side, h, depth]} />
          </mesh>
          <mesh position={[room.door.width / 2 + side / 2, y, hd - depth / 2]} material={mat}>
            <boxGeometry args={[side, h, depth]} />
          </mesh>
        </>
      ) : (
        <mesh position={[0, y, hd - depth / 2]} material={mat}>
          <boxGeometry args={[room.width, h, depth]} />
        </mesh>
      )}
    </group>
  );
}

/** A laylight: frosted glass in the ceiling with a grid of glazing bars. */
function Skylight({ w, d, h, strength }: { w: number; d: number; h: number; strength: number }) {
  const bars = useMemo(() => {
    const out: [number, number, number, number][] = [];
    const nx = Math.max(2, Math.round(w / 1.6));
    const nz = Math.max(2, Math.round(d / 1.6));
    for (let i = 1; i < nx; i++) out.push([-w / 2 + (w * i) / nx, 0, 0.05, d]);
    for (let j = 1; j < nz; j++) out.push([0, -d / 2 + (d * j) / nz, w, 0.05]);
    return out;
  }, [w, d]);
  return (
    <group position={[0, h - 0.01, 0]}>
      <mesh rotation-x={Math.PI / 2}>
        <planeGeometry args={[w, d]} />
        <meshStandardMaterial color="#ffffff" emissive="#f7f5ef" emissiveIntensity={0.04 + strength * 0.9} roughness={1} toneMapped={false} />
      </mesh>
      {bars.map(([x, z, bw, bd], i) => (
        <mesh key={i} position={[x, -0.02, z]}>
          <boxGeometry args={[bw, 0.04, bd]} />
          <meshStandardMaterial color="#8d8a84" roughness={0.6} />
        </mesh>
      ))}
      <rectAreaLight width={w} height={d} intensity={strength * 3.2} color="#f6f4ee" rotation-x={-Math.PI / 2} position={[0, -0.05, 0]} />
    </group>
  );
}

function Bench({ x, z, w, d }: { x: number; z: number; w: number; d: number }) {
  const legs: [number, number][] = [
    [-w / 2 + 0.08, -d / 2 + 0.08],
    [w / 2 - 0.08, -d / 2 + 0.08],
    [-w / 2 + 0.08, d / 2 - 0.08],
    [w / 2 - 0.08, d / 2 - 0.08],
  ];
  return (
    <group position={[x, 0, z]}>
      <mesh position={[0, 0.42, 0]} castShadow>
        <boxGeometry args={[w, 0.08, d]} />
        <meshStandardMaterial color="#3a2a22" roughness={0.55} />
      </mesh>
      {legs.map(([lx, lz], i) => (
        <mesh key={i} position={[lx, 0.19, lz]}>
          <boxGeometry args={[0.05, 0.38, 0.05]} />
          <meshStandardMaterial color="#1c1814" roughness={0.4} metalness={0.4} />
        </mesh>
      ))}
      <mesh rotation-x={-Math.PI / 2} position={[0, 0.005, 0]}>
        <planeGeometry args={[w * 1.5, d * 2]} />
        <meshBasicMaterial map={blobTexture()} transparent depthWrite={false} opacity={0.8} />
      </mesh>
    </group>
  );
}
