import type { CSSProperties } from "react";
import type { FrameStyle } from "./types";

// Picture frames drawn in CSS. A moulding has a profile: an outer edge, a
// broad face that catches the light, and, next to the picture, a dark
// sight-edge lip with a burnished bead beside it. The light comes from the
// ceiling at the front left, so top and left faces are bright and the
// frame throws a shadow down onto the wall and a sliver onto the picture.

const faces: Record<FrameStyle, string> = {
  gilt: "linear-gradient(135deg, #6e5220 0%, #c9a557 11%, #f1dd9b 21%, #b38c44 33%, #e6c97c 47%, #8f6b2e 61%, #d9b96d 75%, #7a5a24 88%, #b8944f 100%)",
  oak: "linear-gradient(135deg, #5f432a 0%, #9c7751 26%, #b58f65 42%, #8a6844 60%, #a7825a 78%, #5c412a 100%)",
  black: "linear-gradient(135deg, #2e2c2a 0%, #0f0e0d 30%, #262422 48%, #0c0b0a 70%, #1c1a18 100%)",
  silk: "repeating-linear-gradient(45deg, rgba(255,255,255,.07) 0 2px, transparent 2px 7px), linear-gradient(135deg, #a8936c, #d4c29c 40%, #bda983 70%, #9e8a64)",
};

const lips: Record<FrameStyle, [string, string]> = {
  // [dark lip, bright bead]
  gilt: ["#4d3810", "#f6e3a3"],
  oak: ["#3b2918", "#c9a57a"],
  black: ["#000000", "#4a4744"],
  silk: ["#6b5b3d", "#e8dcc0"],
};

export function frameCss(style: FrameStyle, border: number, shadow = true): { outer: CSSProperties; sight: CSSProperties } {
  const b = Math.max(2, border);
  const bevel = Math.max(1, b * 0.22);
  const [lip, bead] = lips[style] ?? lips.gilt;
  const lipW = Math.max(1, b * 0.1);
  const beadW = Math.max(1, b * 0.08);
  return {
    outer: {
      padding: b,
      background: faces[style] ?? faces.gilt,
      boxShadow: [
        shadow && "0 1px 1px rgba(0,0,0,.35), 0 12px 20px -8px rgba(0,0,0,.55), 0 34px 60px -30px rgba(0,0,0,.65)",
        "inset 0 0 0 1px rgba(0,0,0,.5)",
        `inset ${bevel}px ${bevel}px ${bevel * 1.4}px rgba(255,255,255,${style === "black" ? 0.12 : 0.28})`,
        `inset ${-bevel}px ${-bevel}px ${bevel * 1.6}px rgba(0,0,0,.42)`,
      ]
        .filter(Boolean)
        .join(", "),
    },
    sight: {
      boxShadow: [
        `0 0 0 ${lipW}px ${lip}`,
        `0 0 0 ${lipW + beadW}px ${bead}`,
        `0 0 0 ${lipW + beadW + Math.max(1, b * 0.1)}px rgba(0,0,0,.22)`,
      ].join(", "),
    },
  };
}

/** The shadow a frame casts onto the picture it holds. */
export const pictureShade = "inset 0 3px 7px -2px rgba(0,0,0,.55), inset 2px 0 5px -3px rgba(0,0,0,.4)";
