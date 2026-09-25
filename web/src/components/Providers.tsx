"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Lenis from "lenis";
import { usePathname } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { gsap } from "gsap";
import { ScrollTrigger } from "gsap/ScrollTrigger";

gsap.registerPlugin(ScrollTrigger);

export function Providers({ children }: { children: ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } },
      }),
  );
  return (
    <QueryClientProvider client={client}>
      <SmoothScroll />
      {children}
    </QueryClientProvider>
  );
}

/**
 * Lenis gives long pages a weighted, even scroll, driven by GSAP's ticker so
 * scroll-linked animation stays in step. The studio and the 3D rooms need
 * the wheel for themselves and go without.
 */
function SmoothScroll() {
  const path = usePathname();
  const off = path.startsWith("/studio") || path.endsWith("/walk");
  useEffect(() => {
    if (off || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const lenis = new Lenis({ duration: 1.1, smoothWheel: true });
    lenis.on("scroll", ScrollTrigger.update);
    const tick = (t: number) => lenis.raf(t * 1000);
    gsap.ticker.add(tick);
    gsap.ticker.lagSmoothing(0);
    return () => {
      gsap.ticker.remove(tick);
      lenis.destroy();
    };
  }, [off]);
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [path]);
  return null;
}
