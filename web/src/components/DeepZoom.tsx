"use client";

import { AnimatePresence, motion } from "motion/react";
import { useEffect, useEffectEvent, useRef, useState } from "react";

/**
 * Close looking: the server cuts each picture into a Deep Zoom pyramid and
 * OpenSeadragon fetches only the tiles on screen at the zoom being shown.
 */
export function DeepZoom({ id, title, open, onClose }: { id: number; title: string; open: boolean; onClose: () => void }) {
  const host = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const close = useEffectEvent(() => onClose());

  useEffect(() => {
    if (!open || !host.current) return;
    let viewer: { destroy(): void } | undefined;
    let cancelled = false;
    import("openseadragon").then(({ default: OSD }) => {
      if (cancelled || !host.current) return;
      const v = OSD({
        element: host.current,
        tileSources: `/dzi/${id}.dzi`,
        showNavigationControl: false,
        showNavigator: true,
        navigatorPosition: "BOTTOM_RIGHT",
        navigatorBackground: "#000",
        navigatorBorderColor: "rgba(255,255,255,.3)",
        navigatorDisplayRegionColor: "#e3c983",
        animationTime: 0.9,
        springStiffness: 8,
        blendTime: 0.2,
        maxZoomPixelRatio: 2.5,
        visibilityRatio: 0.8,
        gestureSettingsMouse: { clickToZoom: true, dblClickToZoom: false, scrollToZoom: true },
        crossOriginPolicy: "Anonymous",
      });
      v.addHandler("open", () => setStatus("ready"));
      // Without tiles (a server handing pictures to a CDN), fall back to
      // one large picture in the same viewer.
      let fellBack = false;
      v.addHandler("open-failed", () => {
        if (fellBack) return setStatus("error");
        fellBack = true;
        v.open({ type: "image", url: `/img/${id}/2400.jpg`, crossOriginPolicy: "Anonymous" } as never);
      });
      viewer = v;
    });
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && close();
    window.addEventListener("keydown", onKey);
    document.documentElement.style.overflow = "hidden";
    return () => {
      cancelled = true;
      setStatus("loading");
      viewer?.destroy();
      window.removeEventListener("keydown", onKey);
      document.documentElement.style.overflow = "";
    };
  }, [open, id]);

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          role="dialog"
          aria-modal="true"
          aria-label={`${title}, up close`}
          className="fixed inset-0 z-[120] bg-[#0b0a09] text-[#f2ede3]"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
        >
          <div ref={host} className="absolute inset-0" />
          <div className="pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between gap-6 bg-gradient-to-b from-black/70 to-transparent p-5 sm:p-7">
            <p className="lettering-sm max-w-[60ch] text-[22px] italic">{title}</p>
            <button className="btn pointer-events-auto border border-white/25 hover:border-white" onClick={onClose}>
              Close
            </button>
          </div>
          <p className="pointer-events-none absolute bottom-6 left-6 text-[14px] text-white/60">
            {status === "loading" && "Cutting the picture into tiles… the first look at a work takes a few seconds."}
            {status === "ready" && "Scroll or pinch to zoom, drag to move."}
            {status === "error" && "The museum's picture couldn't be loaded. Try again in a moment."}
          </p>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
