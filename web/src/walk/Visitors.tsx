"use client";

import { Billboard, Html, Text } from "@react-three/drei";
import { useFrame } from "@react-three/fiber";
import { useRef } from "react";
import * as THREE from "three";
import { motion as live, useWalk, type Peer } from "./store";
import { blobTexture } from "./textures";

const EMOJI: Record<string, string> = { clap: "👏", heart: "❤️", wow: "😮", spark: "✨" };

/** Another visitor: a figure in their colour, their name above, walking where they walk. */
function Figure({ peer }: { peer: Peer }) {
  const group = useRef<THREE.Group>(null);
  const body = useRef<THREE.Group>(null);
  const reactions = useWalk((s) => s.reactions.filter((r) => r.peer === peer.id));
  const colour = `hsl(${peer.hue} 45% 58%)`;

  useFrame((_, dt) => {
    const m = live.peers.get(peer.id);
    const g = group.current;
    if (!m || !g) return;
    // Ease toward the latest reported position; positions arrive 10 times a second.
    const k = 1 - Math.exp(-dt * 9);
    m.x += (m.tx - m.x) * k;
    m.z += (m.tz - m.z) * k;
    let d = m.tyaw - m.yaw;
    d = Math.atan2(Math.sin(d), Math.cos(d));
    m.yaw += d * k;
    g.position.set(m.x, 0, m.z);
    if (body.current) {
      body.current.rotation.y = m.yaw;
      const speed = Math.hypot(m.tx - m.x, m.tz - m.z);
      body.current.position.y = Math.abs(Math.sin(performance.now() / 160)) * Math.min(0.04, speed * 0.2);
    }
  });

  return (
    <group ref={group}>
      <group ref={body}>
        <mesh position={[0, 0.78, 0]} castShadow>
          <capsuleGeometry args={[0.2, 0.9, 6, 16]} />
          <meshStandardMaterial color={colour} roughness={0.55} />
        </mesh>
        <mesh position={[0, 1.52, 0]} castShadow>
          <sphereGeometry args={[0.14, 20, 16]} />
          <meshStandardMaterial color={colour} roughness={0.5} />
        </mesh>
        {/* Which way they're facing. */}
        <mesh position={[0, 1.54, -0.13]}>
          <sphereGeometry args={[0.035, 10, 8]} />
          <meshStandardMaterial color="#1d1a16" />
        </mesh>
      </group>
      <mesh rotation-x={-Math.PI / 2} position={[0, 0.006, 0]}>
        <planeGeometry args={[0.9, 0.9]} />
        <meshBasicMaterial map={blobTexture()} transparent depthWrite={false} />
      </mesh>
      <Billboard position={[0, 1.92, 0]}>
        <Text font="/fonts/Archivo-Medium.ttf" fontSize={0.1} color="#ffffff" outlineWidth={0.012} outlineColor="#1d1a16" anchorY="bottom">
          {peer.name}
        </Text>
      </Billboard>
      {reactions.map((r) => (
        <Html key={r.key} position={[0, 2.1, 0]} center zIndexRange={[20, 0]}>
          <span className="walk-float pointer-events-none block select-none text-[34px]">{EMOJI[r.e]}</span>
        </Html>
      ))}
    </group>
  );
}

export function Visitors() {
  const peers = useWalk((s) => s.peers);
  return (
    <group>
      {Object.values(peers).map((p) => (
        <Figure key={p.id} peer={p} />
      ))}
    </group>
  );
}

export { EMOJI };
