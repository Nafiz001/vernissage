"use client";

import { decode } from "blurhash";
import { useState, type CSSProperties, type ReactNode } from "react";
import { srcSet } from "@/lib/api";
import { frameCss, pictureShade } from "@/lib/frames";
import type { Artwork, Hang } from "@/lib/types";

const blurCache = new Map<string, string>();

/**
 * A blurhash as a soft placeholder: decoded to a 6 x 4 grid of colours and
 * drawn as a blurred SVG. Pure string work, so the server and the browser
 * render the same thing and no canvas is needed.
 */
export function blurhashURL(hash?: string): string | undefined {
  if (!hash) return undefined;
  let url = blurCache.get(hash);
  if (url) return url;
  try {
    const W = 6;
    const H = 4;
    const px = decode(hash, W, H);
    let rects = "";
    for (let y = 0; y < H; y++) {
      for (let x = 0; x < W; x++) {
        const i = (y * W + x) * 4;
        rects += `<rect x='${x}' y='${y}' width='1.02' height='1.02' fill='rgb(${px[i]},${px[i + 1]},${px[i + 2]})'/>`;
      }
    }
    const svg = `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 ${W} ${H}' preserveAspectRatio='none'><filter id='b' x='-10%' y='-10%' width='120%' height='120%'><feGaussianBlur stdDeviation='.55'/></filter><g filter='url(#b)'>${rects}</g></svg>`;
    url = `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
  } catch {
    return undefined;
  }
  blurCache.set(hash, url);
  return url;
}

export interface FramedProps {
  artwork: Pick<Artwork, "id" | "title" | "image" | "hang">;
  /** Displayed width of the whole frame, px. Height follows. */
  width: number;
  /** Largest picture width to request. */
  maxSrc?: number;
  priority?: boolean;
  className?: string;
  style?: CSSProperties;
  shadow?: boolean;
  children?: ReactNode;
  onLoad?: () => void;
}

export function frameGeometry(hang: Hang, width: number) {
  const scale = width / hang.w;
  return {
    width,
    height: hang.h * scale,
    border: Math.max(2, hang.border * scale),
    mat: hang.mat * scale,
  };
}

/** A work in its frame, as a curator would hang it. */
export function Framed({ artwork, width, maxSrc = 1200, priority, className = "", style, shadow = true, children, onLoad }: FramedProps) {
  const g = frameGeometry(artwork.hang, width);
  const blur = blurhashURL(artwork.image.blurhash);
  const [loaded, setLoaded] = useState(false);
  const css = frameCss(artwork.hang.frame, g.border, shadow);
  return (
    <div className={`relative shrink-0 ${className}`} style={{ width: g.width, height: g.height, ...css.outer, ...style }}>
      <div className="relative h-full w-full" style={{ padding: g.mat, background: g.mat > 0 ? "#f3efe5" : undefined, ...css.sight }}>
        <div
          className="relative h-full w-full overflow-hidden"
          style={{
            backgroundColor: artwork.image.dominant ?? "#3a3330",
            backgroundImage: blur ? `url(${blur})` : undefined,
            backgroundSize: "cover",
            boxShadow: g.mat > 0 ? "0 0 0 1px rgba(0,0,0,.14), 0 0 0 3px #fbf9f3, 0 0 0 4px rgba(0,0,0,.08)" : undefined,
          }}
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={`/img/${artwork.id}/${width > 500 ? 1200 : width > 250 ? 800 : 400}.jpg`}
            srcSet={srcSet(artwork.id, maxSrc)}
            sizes={`${Math.round(width)}px`}
            alt={artwork.title}
            loading={priority ? "eager" : "lazy"}
            decoding="async"
            draggable={false}
            onLoad={() => {
              setLoaded(true);
              onLoad?.();
            }}
            className="absolute inset-0 h-full w-full object-cover transition-opacity duration-700"
            style={{ opacity: loaded ? 1 : 0 }}
          />
          {/* The frame's shadow on the picture, and the sheen of varnish. */}
          <div
            className="pointer-events-none absolute inset-0"
            style={{ boxShadow: g.mat > 0 ? undefined : pictureShade, background: "linear-gradient(160deg,rgba(255,255,255,.07),transparent 40%)" }}
          />
        </div>
      </div>
      {children}
    </div>
  );
}
