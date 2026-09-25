"use client";

import { useSyncExternalStore } from "react";

const noop = () => () => {};

// Values that only exist in the browser. The server renders with null and
// the browser fills in the real value right after hydration, without a
// mismatch warning.

const clientNow = typeof window === "undefined" ? 0 : Date.now();

/** When the page was opened, or null while rendering on the server. */
export function useNow(): number | null {
  return useSyncExternalStore(noop, () => clientNow, () => null);
}

function subscribeResize(cb: () => void) {
  window.addEventListener("resize", cb);
  return () => window.removeEventListener("resize", cb);
}

/** The window's inner height, or a fallback on the server. */
export function useViewportHeight(fallback = 800): number {
  return useSyncExternalStore(subscribeResize, () => window.innerHeight, () => fallback);
}

const STORAGE_EVENT = "vernissage-storage";

function subscribeStorage(cb: () => void) {
  window.addEventListener("storage", cb);
  window.addEventListener(STORAGE_EVENT, cb);
  return () => {
    window.removeEventListener("storage", cb);
    window.removeEventListener(STORAGE_EVENT, cb);
  };
}

function read(kind: "local" | "session", key: string) {
  try {
    return (kind === "local" ? localStorage : sessionStorage).getItem(key);
  } catch {
    return null;
  }
}

/** A string kept in the browser's storage, shared by every component that reads it. */
export function useStored(kind: "local" | "session", key: string): [string | null, (v: string) => void] {
  const value = useSyncExternalStore(subscribeStorage, () => read(kind, key), () => null);
  const set = (v: string) => {
    try {
      (kind === "local" ? localStorage : sessionStorage).setItem(key, v);
    } catch {}
    window.dispatchEvent(new Event(STORAGE_EVENT));
  };
  return [value, set];
}

let seed: number | null = null;

function sessionSeed() {
  if (seed === null) {
    seed = Number(read("session", "vernissage.seed")) || Math.floor(Math.random() * 1e9) + 1;
    try {
      sessionStorage.setItem("vernissage.seed", String(seed));
    } catch {}
  }
  return seed;
}

/**
 * A random number fixed for this browser tab, made on first use: the
 * collection's shuffle order, stable while paging, new each visit.
 */
export function useSessionSeed(): number | null {
  return useSyncExternalStore(noop, sessionSeed, () => null);
}
