"use client";

import { Environment, Lightformer, PerformanceMonitor } from "@react-three/drei";
import { Canvas, useFrame, useThree, type ThreeEvent } from "@react-three/fiber";
import { Bloom, EffectComposer, N8AO, SMAA, ToneMapping, Vignette } from "@react-three/postprocessing";
import { ToneMappingMode } from "postprocessing";
import { Suspense, useEffect, useMemo, useRef } from "react";
import * as THREE from "three";
import { RectAreaLightUniformsLib } from "three/examples/jsm/lights/RectAreaLightUniformsLib.js";
import type { Artwork, Exhibition, Placement, Room } from "@/lib/types";
import { Artworks } from "./Artworks";
import { Controls, useFloorWalk } from "./Controls";
import { FOV, PANEL, SHEET, VESTIBULE, WALL_T, WIDE, wallPoint } from "./geometry";
import { LIGHTS, RoomShell, type Lighting } from "./Room";
import { useWalk } from "./store";
import { Visitors } from "./Visitors";

RectAreaLightUniformsLib.init();

/** Spotlights on the largest works, and a track fixture above every one. */
function Spots({ room, placements, works, light, count }: { room: Room; placements: Placement[]; works: Map<number, Artwork>; light: Lighting; count: number }) {
  const lit = useMemo(() => {
    const sorted = [...placements].sort((a, b) => {
      const ha = works.get(a.artworkId)?.hang;
      const hb = works.get(b.artworkId)?.hang;
      return (hb ? hb.w * hb.h : 0) - (ha ? ha.w * ha.h : 0);
    });
    return new Set(sorted.slice(0, count).map((p) => p.artworkId));
  }, [placements, works, count]);

  const off = Math.min(1.6, room.height * 0.35);
  const H = room.height;
  return (
    <group>
      {/* A lighting track along each wall, a fixture above each work. */}
      {[0, 1, 2, 3].map((w) => {
        const long = w % 2 === 0;
        const len = (long ? room.width : room.depth) - off * 2;
        const f = wallPoint(room, w, (long ? room.width : room.depth) / 2);
        return (
          <mesh key={w} position={[f.x + f.nx * off, H - 0.03, f.z + f.nz * off]} rotation-y={f.rotY}>
            <boxGeometry args={[len, 0.035, 0.04]} />
            <meshStandardMaterial color="#1a1918" metalness={0.5} roughness={0.4} />
          </mesh>
        );
      })}
      {placements.map((p) => {
        const f = wallPoint(room, p.wall, p.x);
        const pos: [number, number, number] = [f.x + f.nx * off, H - 0.16, f.z + f.nz * off];
        return (
          <group key={p.artworkId}>
            <mesh position={pos} rotation={[0.6, f.rotY, 0, "YXZ"]}>
              <cylinderGeometry args={[0.045, 0.06, 0.2, 14]} />
              <meshStandardMaterial color="#1a1918" metalness={0.6} roughness={0.35} />
            </mesh>
            {lit.has(p.artworkId) && <SpotOn from={pos} to={[f.x, p.y, f.z]} light={light} />}
          </group>
        );
      })}
    </group>
  );
}

function SpotOn({ from, to, light }: { from: [number, number, number]; to: [number, number, number]; light: Lighting }) {
  const target = useMemo(() => {
    const o = new THREE.Object3D();
    o.position.set(...to);
    return o;
  }, [to]);
  return (
    <>
      <primitive object={target} />
      <spotLight position={from} target={target} angle={0.5} penumbra={0.85} intensity={light.spot} distance={14} decay={1.5} color="#ffe9cc" />
    </>
  );
}

/**
 * While the label panel is open, shift the picture so the work sits in the
 * middle of what's still visible: left of the panel on a wide screen, above
 * the sheet on a phone.
 */
function ViewShift() {
  const viewing = useWalk((s) => s.viewing);
  const { camera, size } = useThree();
  const off = useRef({ x: 0, y: 0 });
  useFrame((_, dt) => {
    const wide = size.width > WIDE;
    const tx = viewing !== null && wide ? PANEL : 0;
    const ty = viewing !== null && !wide ? size.height * SHEET : 0;
    const k = 1 - Math.exp(-dt * 5);
    off.current.x += (tx - off.current.x) * k;
    off.current.y += (ty - off.current.y) * k;
    const { x, y } = off.current;
    const cam = camera as THREE.PerspectiveCamera;
    if (Math.abs(x) < 0.5 && Math.abs(y) < 0.5) {
      if (cam.view?.enabled) cam.clearViewOffset();
      return;
    }
    cam.setViewOffset(size.width + x, size.height + y, x, y, size.width, size.height);
  });
  return null;
}

function FloorPicker({ room }: { room: Room }) {
  const walk = useFloorWalk(room);
  const view = useWalk((s) => s.view);
  const length = room.depth + VESTIBULE + WALL_T;
  return (
    <mesh
      rotation-x={-Math.PI / 2}
      position={[0, 0.002, (VESTIBULE + WALL_T) / 2]}
      visible={false}
      onClick={(e: ThreeEvent<MouseEvent>) => {
        if (e.delta > 6) return;
        e.stopPropagation();
        view(null);
        walk(e.point);
      }}
    >
      <planeGeometry args={[room.width, length]} />
    </mesh>
  );
}

function Effects({ quality, light }: { quality: string; light: Lighting }) {
  if (quality === "low") return null;
  if (quality === "high") {
    return (
      <EffectComposer multisampling={4}>
        <N8AO halfRes aoRadius={0.7} intensity={2.2} distanceFalloff={0.6} />
        <Bloom luminanceThreshold={0.82} intensity={light.bloom} mipmapBlur />
        <Vignette offset={0.28} darkness={0.5} />
        <ToneMapping mode={ToneMappingMode.ACES_FILMIC} />
      </EffectComposer>
    );
  }
  return (
    <EffectComposer multisampling={0}>
      <Bloom luminanceThreshold={0.82} intensity={light.bloom} mipmapBlur />
      <Vignette offset={0.28} darkness={0.5} />
      <SMAA />
      <ToneMapping mode={ToneMappingMode.ACES_FILMIC} />
    </EffectComposer>
  );
}

export function Scene({ e, room, placements, works }: { e: Exhibition; room: Room; placements: Placement[]; works: Map<number, Artwork> }) {
  const quality = useWalk((s) => s.quality);
  const setQuality = useWalk((s) => s.setQuality);
  const light = LIGHTS[e.light] ?? LIGHTS.gallery!;

  useEffect(() => () => void (document.body.style.cursor = ""), []);

  return (
    <Canvas
      dpr={quality === "high" ? [1, 1.5] : quality === "medium" ? [1, 1.2] : 1}
      gl={{ antialias: quality === "low", powerPreference: "high-performance", toneMapping: THREE.ACESFilmicToneMapping, toneMappingExposure: light.exposure }}
      camera={{ fov: FOV, near: 0.05, far: 120, position: [0, 1.62, room.depth / 2 + 3.4] }}
      className="!fixed inset-0 touch-none"
    >
      <color attach="background" args={["#0b0a09"]} />
      <PerformanceMonitor
        flipflops={2}
        onDecline={() => setQuality(quality === "high" ? "medium" : "low")}
      />
      <ambientLight intensity={light.ambient} color="#fff6ea" />
      <hemisphereLight intensity={light.hemi} color={light.hemiSky} groundColor={light.hemiGround} />
      <Suspense fallback={null}>
        <Environment resolution={128} frames={1}>
          <Lightformer form="rect" intensity={2.2} position={[0, 5, 0]} rotation-x={Math.PI / 2} scale={[8, 5, 1]} />
          <Lightformer form="rect" intensity={0.8} position={[-6, 2, 0]} rotation-y={Math.PI / 2} scale={[6, 2, 1]} color="#ffe2bd" />
          <Lightformer form="rect" intensity={0.8} position={[6, 2, 0]} rotation-y={-Math.PI / 2} scale={[6, 2, 1]} color="#ffe2bd" />
          <Lightformer form="ring" intensity={1.4} position={[0, 3, 6]} scale={2} />
        </Environment>
        <RoomShell room={room} e={e} quality={quality} light={light} />
        <Artworks room={room} placements={placements} works={works} light={light} />
        <Spots room={room} placements={placements} works={works} light={light} count={quality === "high" ? 6 : quality === "medium" ? 3 : 0} />
        <Visitors />
      </Suspense>
      <FloorPicker room={room} />
      <Controls room={room} />
      <ViewShift />
      <Effects quality={quality} light={light} />
    </Canvas>
  );
}
