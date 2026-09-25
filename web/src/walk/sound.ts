"use client";

import { useEffect } from "react";
import { motion as live, useWalk } from "./store";

/**
 * The sound of a quiet gallery, synthesised: a low room tone (filtered
 * brown noise) and soft footsteps while you walk. Nothing plays until the
 * visitor turns sound on.
 */
export function useRoomSound(big: boolean) {
  const on = useWalk((s) => s.sound);
  useEffect(() => {
    if (!on) return;
    const ctx = new AudioContext();
    const master = ctx.createGain();
    master.gain.value = 0;
    master.gain.linearRampToValueAtTime(1, ctx.currentTime + 1.5);
    master.connect(ctx.destination);

    // Brown noise, two seconds, looped.
    const len = ctx.sampleRate * 2;
    const buf = ctx.createBuffer(1, len, ctx.sampleRate);
    const data = buf.getChannelData(0);
    let last = 0;
    for (let i = 0; i < len; i++) {
      last = (last + 0.02 * (Math.random() * 2 - 1)) / 1.02;
      data[i] = last * 3.5;
    }
    const tone = ctx.createBufferSource();
    tone.buffer = buf;
    tone.loop = true;
    const low = ctx.createBiquadFilter();
    low.type = "lowpass";
    low.frequency.value = big ? 320 : 460;
    const toneGain = ctx.createGain();
    toneGain.gain.value = 0.05;
    tone.connect(low).connect(toneGain).connect(master);
    tone.start();

    // A small echo, bigger in a bigger room.
    const echo = ctx.createDelay();
    echo.delayTime.value = big ? 0.11 : 0.06;
    const echoGain = ctx.createGain();
    echoGain.gain.value = 0.25;
    echo.connect(echoGain).connect(master);

    const step = () => {
      const t = ctx.currentTime;
      const src = ctx.createBufferSource();
      src.buffer = buf;
      const band = ctx.createBiquadFilter();
      band.type = "bandpass";
      band.frequency.value = 240 + Math.random() * 160;
      band.Q.value = 1.4;
      const g = ctx.createGain();
      g.gain.setValueAtTime(0, t);
      g.gain.linearRampToValueAtTime(0.5, t + 0.01);
      g.gain.exponentialRampToValueAtTime(0.001, t + 0.16);
      src.connect(band).connect(g);
      g.connect(master);
      g.connect(echo);
      src.start(t, Math.random() * 1.5, 0.2);
    };
    const timer = setInterval(() => {
      if (live.moving) {
        step();
      }
    }, 520);

    return () => {
      clearInterval(timer);
      master.gain.linearRampToValueAtTime(0, ctx.currentTime + 0.3);
      setTimeout(() => ctx.close(), 400);
    };
  }, [on, big]);
}

/** Reads a work's label aloud, as an audio guide would. */
export function speak(text: string | null) {
  if (typeof window === "undefined" || !("speechSynthesis" in window)) return false;
  window.speechSynthesis.cancel();
  if (!text) return true;
  const u = new SpeechSynthesisUtterance(text);
  const voices = window.speechSynthesis.getVoices();
  u.voice = voices.find((v) => v.lang === "en-GB") ?? voices.find((v) => v.lang.startsWith("en")) ?? null;
  u.rate = 0.96;
  window.speechSynthesis.speak(u);
  return true;
}
