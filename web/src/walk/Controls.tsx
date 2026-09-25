"use client";

import { useFrame, useThree } from "@react-three/fiber";
import { useEffect, useMemo, useRef } from "react";
import * as THREE from "three";
import type { Room } from "@/lib/types";
import { EYE, VESTIBULE, walkable } from "./geometry";
import { motion as live, useWalk } from "./store";

const WALK = 1.5; // m/s
const RUN = 3.2;

function angleLerp(a: number, b: number, t: number) {
  const d = Math.atan2(Math.sin(b - a), Math.cos(b - a));
  return a + d * t;
}

/**
 * First-person looking and walking. Drag to look; WASD or the arrow keys to
 * walk; click the floor to walk there; a work, when chosen, pulls the camera
 * to a good place to stand in front of it.
 */
export function Controls({ room }: { room: Room }) {
  const { camera, gl } = useThree();
  const area = useMemo(() => walkable(room), [room]);
  const keys = useRef(new Set<string>());
  const drag = useRef<{ x: number; y: number; moved: number } | null>(null);
  const target = useRef<{ x: number; z: number; yaw?: number; pitch?: number } | null>(null);
  const glide = useWalk((s) => s.glide);
  const entered = useWalk((s) => s.entered);
  const bob = useRef(0);

  // Start in the entrance hall, facing the door.
  useEffect(() => {
    live.me = { x: 0, z: room.depth / 2 + VESTIBULE - 0.5, yaw: 0, pitch: 0.2 };
    camera.rotation.order = "YXZ";
  }, [room, camera]);

  useEffect(() => {
    if (glide) target.current = { x: glide.x, z: glide.z, yaw: glide.yaw, pitch: glide.pitch };
  }, [glide]);

  useEffect(() => {
    const el = gl.domElement;
    const typing = () => {
      const a = document.activeElement;
      return a instanceof HTMLInputElement || a instanceof HTMLTextAreaElement;
    };
    const down = (e: KeyboardEvent) => {
      if (typing()) return;
      const k = e.key.toLowerCase();
      if (["w", "a", "s", "d", "arrowup", "arrowdown", "arrowleft", "arrowright", "shift"].includes(k)) {
        keys.current.add(k);
        target.current = null;
        if (k.startsWith("arrow")) e.preventDefault();
      }
    };
    const up = (e: KeyboardEvent) => keys.current.delete(e.key.toLowerCase());
    const blur = () => keys.current.clear();
    const pdown = (e: PointerEvent) => {
      drag.current = { x: e.clientX, y: e.clientY, moved: 0 };
      try {
        el.setPointerCapture(e.pointerId);
      } catch {}
    };
    const pmove = (e: PointerEvent) => {
      const d = drag.current;
      if (!d) return;
      const dx = e.clientX - d.x;
      const dy = e.clientY - d.y;
      d.x = e.clientX;
      d.y = e.clientY;
      d.moved += Math.abs(dx) + Math.abs(dy);
      if (d.moved < 4) return;
      const s = e.pointerType === "touch" ? 0.006 : 0.0034;
      live.me.yaw += dx * s;
      live.me.pitch = Math.max(-0.7, Math.min(0.55, live.me.pitch + dy * s));
      if (target.current) {
        target.current.yaw = undefined;
        target.current.pitch = undefined;
      }
    };
    const pup = (e: PointerEvent) => {
      drag.current = null;
      try {
        el.releasePointerCapture(e.pointerId);
      } catch {}
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", blur);
    el.addEventListener("pointerdown", pdown);
    el.addEventListener("pointermove", pmove);
    el.addEventListener("pointerup", pup);
    el.addEventListener("pointercancel", pup);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", blur);
      el.removeEventListener("pointerdown", pdown);
      el.removeEventListener("pointermove", pmove);
      el.removeEventListener("pointerup", pup);
      el.removeEventListener("pointercancel", pup);
    };
  }, [gl]);


  useFrame((_, rawDt) => {
    const dt = Math.min(rawDt, 0.05);
    const me = live.me;
    const k = keys.current;
    let fwd = 0;
    let strafe = 0;
    if (k.has("w") || k.has("arrowup")) fwd += 1;
    if (k.has("s") || k.has("arrowdown")) fwd -= 1;
    if (k.has("d")) strafe += 1;
    if (k.has("a")) strafe -= 1;
    // Arrow keys turn, as in most walking games.
    if (k.has("arrowleft")) me.yaw += dt * 1.8;
    if (k.has("arrowright")) me.yaw -= dt * 1.8;

    let moving = false;
    if (entered && (fwd || strafe)) {
      const speed = (k.has("shift") ? RUN : WALK) * dt;
      const len = Math.hypot(fwd, strafe);
      const sin = Math.sin(me.yaw);
      const cos = Math.cos(me.yaw);
      const dx = ((-sin * fwd + cos * strafe) / len) * speed;
      const dz = ((-cos * fwd - sin * strafe) / len) * speed;
      [me.x, me.z] = area.step(me.x, me.z, me.x + dx, me.z + dz);
      moving = true;
    } else if (target.current) {
      const t = target.current;
      const a = 1 - Math.exp(-dt * 2.6);
      const nx = me.x + (t.x - me.x) * a;
      const nz = me.z + (t.z - me.z) * a;
      // Glides go through the doorway, so they may cross the hall's walls'
      // lines; only keyboard walking is held to the plan.
      me.x = nx;
      me.z = nz;
      if (t.yaw !== undefined) me.yaw = angleLerp(me.yaw, t.yaw, a);
      if (t.pitch !== undefined) me.pitch += (t.pitch - me.pitch) * a;
      moving = Math.hypot(t.x - me.x, t.z - me.z) > 0.05;
      if (!moving && (t.yaw === undefined || Math.abs(Math.atan2(Math.sin(t.yaw - me.yaw), Math.cos(t.yaw - me.yaw))) < 0.01)) target.current = null;
    }
    live.moving = moving;

    bob.current += moving ? dt * 9 : 0;
    const y = EYE + (moving ? Math.sin(bob.current) * 0.012 : 0);
    camera.position.set(me.x, y, me.z);
    camera.rotation.set(me.pitch, me.yaw, 0, "YXZ");
  });

  return null;
}

/** Turns a click on the floor into a walk to that spot. */
export function useFloorWalk(room: Room) {
  const area = useMemo(() => walkable(room), [room]);
  const glideTo = useWalk((s) => s.glideTo);
  return (point: THREE.Vector3) => {
    if (!area.ok(point.x, point.z)) return;
    glideTo({ x: point.x, z: point.z, yaw: live.me.yaw, pitch: live.me.pitch });
  };
}
