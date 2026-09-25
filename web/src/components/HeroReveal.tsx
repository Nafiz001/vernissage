"use client";

import { useGSAP } from "@gsap/react";
import { gsap } from "gsap";
import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { srcSet } from "@/lib/api";
import { frameCss, pictureShade } from "@/lib/frames";
import { useNow } from "@/lib/hooks";
import type { Artwork } from "@/lib/types";
import { blurhashURL } from "./Framed";
import { WallLabel } from "./WallLabel";

gsap.registerPlugin(useGSAP);

/** Fits a work's frame into a box, keeping its real proportions. */
function fitFrame(a: Artwork, boxW: number, boxH: number) {
  const s = Math.min(boxW / a.hang.w, boxH / a.hang.h);
  return { w: a.hang.w * s, h: a.hang.h * s, border: Math.max(6, a.hang.border * s), mat: a.hang.mat * s };
}

function useBox() {
  const [box, setBox] = useState({ w: 520, h: 520 });
  useLayoutEffect(() => {
    const measure = () => {
      const vw = window.innerWidth;
      const vh = window.innerHeight;
      // Leave room beside the frame for its label on wide screens.
      if (vw >= 1280) setBox({ w: Math.min(560, vw * 0.58 - 330), h: Math.min(vh * 0.6, 620) });
      else if (vw >= 1024) setBox({ w: Math.min(560, vw * 0.5), h: Math.min(vh * 0.6, 620) });
      else setBox({ w: vw - 48, h: Math.min(vh * 0.46, 460) });
    };
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, []);
  return box;
}

/**
 * The opening: close enough to a masterpiece to see the brushstrokes, then
 * stepping back until it hangs on the wall in its frame, the room's light
 * comes up, and the lettering appears beside it. Any key, click or scroll
 * hurries it along; people who prefer less motion see the end at once.
 */
export function HeroReveal({ works, total }: { works: Artwork[]; total: number }) {
  const now = useNow();
  const [step, setStep] = useState(0);
  const ready = now !== null;
  const index = ready ? (Math.floor(now / 86_400_000) + step) % Math.max(1, works.length) : 0;
  const [played, setPlayed] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const box = useBox();
  const root = useRef<HTMLDivElement>(null);
  const frame = useRef<HTMLDivElement>(null);
  const work = works[index];
  const blur = blurhashURL(work?.image.blurhash);

  // If the picture is slow, don't leave anyone in the dark.
  useEffect(() => {
    const t = setTimeout(() => setLoaded(true), 4000);
    return () => clearTimeout(t);
  }, []);

  useGSAP(
    () => {
      if (!loaded || played || !frame.current) return;
      const el = frame.current;
      const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      const rect = el.getBoundingClientRect();
      const vw = window.innerWidth;
      const vh = window.innerHeight;
      // Close enough that one passage of paint fills the screen.
      const S = Math.max(vw / rect.width, vh / rect.height) * 2.1;
      const fx = 0.5;
      const fy = 0.42;
      const x = vw / 2 - rect.left - S * fx * rect.width;
      const y = vh / 2 - rect.top - S * fy * rect.height;

      const tl = gsap.timeline({ onComplete: () => setPlayed(true) });
      tl.set(el, { transformOrigin: "0 0", x, y, scale: S })
        .set(".hero-reveal", { autoAlpha: 0, y: 28 })
        .set(".hero-light", { autoAlpha: 0 })
        .to(".hero-dark", { autoAlpha: 0, duration: 1.1, ease: "power1.out" }, 0)
        .to(el, { x: 0, y: 0, scale: 1, duration: 4.6, ease: "power3.inOut" }, 0.5)
        .to(".hero-light", { autoAlpha: 1, duration: 1.6, ease: "power2.out" }, 3.4)
        .to(".hero-reveal", { autoAlpha: 1, y: 0, duration: 1.1, ease: "power3.out", stagger: 0.12 }, 4.2);
      if (reduce) tl.progress(1);

      const hurry = () => tl.timeScale(5);
      window.addEventListener("wheel", hurry, { once: true, passive: true });
      window.addEventListener("keydown", hurry, { once: true });
      window.addEventListener("pointerdown", hurry, { once: true });
      return () => {
        window.removeEventListener("wheel", hurry);
        window.removeEventListener("keydown", hurry);
        window.removeEventListener("pointerdown", hurry);
      };
    },
    { scope: root, dependencies: [loaded] },
  );

  if (!work) return null;
  const f = fitFrame(work, box.w, box.h);
  const css = frameCss(work.hang.frame, f.border);
  const go = (d: number) => setStep((s) => s + d + works.length);

  return (
    <div ref={root} className="relative mx-auto grid max-w-[1440px] items-center gap-10 px-6 pb-20 pt-6 sm:px-8 lg:min-h-[calc(100svh-76px)] lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:gap-6 lg:pb-16">
      <div className="relative z-10 order-2 lg:order-1">
        <h1 className="hero-reveal lettering text-[clamp(50px,5.5vw,92px)]">
          Hang your own exhibition.
        </h1>
        <p className="hero-reveal mt-7 max-w-[34ch] text-[19px] leading-[1.55] text-[var(--soft)]">
          Choose from {total.toLocaleString("en")} paintings, prints and drawings that The Met and the Cleveland Museum of
          Art have given to the public. Hang them in a room, send the link, and walk through it with friends.
        </p>
        <div className="hero-reveal mt-9 flex flex-wrap gap-3">
          <Link href="/collection" className="btn btn-solid">
            Browse the collection
          </Link>
          <Link href="/exhibitions" className="btn btn-line">
            See what&apos;s showing
          </Link>
        </div>
      </div>

      <div className="relative order-1 flex min-h-[300px] items-center justify-center gap-10 lg:order-2 lg:min-h-[70vh]">
        {/* The pool of light the frame hangs in. */}
        <div
          className="hero-light wash"
          style={{ width: f.w * 2.2, height: f.h * 2.1, left: "50%", top: "46%", transform: "translate(-50%, -54%)" }}
        />
        <motion.div
          ref={frame}
          className="relative z-10"
          animate={{ width: f.w, height: f.h }}
          initial={false}
          transition={{ duration: 0.9, ease: [0.65, 0, 0.35, 1] }}
          style={css.outer}
        >
          <div className="relative h-full w-full" style={{ padding: f.mat, background: f.mat > 0 ? "#f3efe5" : undefined, ...css.sight }}>
            <div
              className="relative h-full w-full overflow-hidden"
              style={{ background: `${work.image.dominant ?? "#2b2522"} center/cover`, backgroundImage: blur ? `url(${blur})` : undefined }}
            >
              <AnimatePresence initial={false}>
                {ready && <motion.img
                  key={work.id}
                  src={`/img/${work.id}/1600.jpg`}
                  srcSet={srcSet(work.id, 2400)}
                  sizes={played ? `${Math.round(f.w)}px` : "200vw"}
                  alt={`${work.title}${work.artist ? `, by ${work.artist}` : ""}`}
                  className="absolute inset-0 h-full w-full object-cover"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.8 }}
                  onLoad={() => setLoaded(true)}
                  draggable={false}
                />}
              </AnimatePresence>
              <div
                className="pointer-events-none absolute inset-0"
                style={{ boxShadow: f.mat > 0 ? undefined : pictureShade, background: "linear-gradient(160deg,rgba(255,255,255,.08),transparent 45%)" }}
              />
            </div>
          </div>
        </motion.div>

        <div className="hero-reveal relative z-20 hidden w-[240px] shrink-0 self-end xl:mb-[12vh] xl:block">
          <Link href={`/artwork/${work.id}`} className="block transition-transform duration-300 hover:-translate-y-0.5">
            <WallLabel artwork={work} credit={false} className="text-[13px]" />
          </Link>
          {works.length > 1 && (
            <div className="mt-3 flex items-center justify-end gap-1">
              <button className="btn btn-quiet h-10 min-h-0 w-10 p-0 text-[20px]" onClick={() => go(-1)} aria-label="Previous work">
                ‹
              </button>
              <button className="btn btn-quiet h-10 min-h-0 w-10 p-0 text-[20px]" onClick={() => go(1)} aria-label="Next work">
                ›
              </button>
            </div>
          )}
        </div>
      </div>

      <p className="hero-reveal order-3 text-[14px] text-[var(--soft)] xl:hidden">
        <Link href={`/artwork/${work.id}`} className="underline-offset-4 hover:underline">
          <span className="italic">{work.title}</span>
          {work.artist ? `, ${work.artist}` : ""}
          {work.date ? `, ${work.date}` : ""}
        </Link>
      </p>

      {/* The lights are off when you arrive. */}
      <div className="hero-dark pointer-events-none fixed inset-0 z-40 bg-[#0c0a09]" style={{ visibility: played ? "hidden" : undefined }} />
    </div>
  );
}
