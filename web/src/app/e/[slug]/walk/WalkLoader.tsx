"use client";

import dynamic from "next/dynamic";
import type { ExhibitionDetail, Room } from "@/lib/types";

// WebGL only exists in the browser.
const Walk = dynamic(() => import("@/walk/Walk"), {
  ssr: false,
  loading: () => (
    <main className="grid min-h-svh place-items-center bg-[#0b0a09] text-[#f2ede3]">
      <p className="lettering-sm text-[26px] italic">Opening the doors…</p>
    </main>
  ),
});

export function WalkLoader({ detail, room }: { detail: ExhibitionDetail; room: Room }) {
  return <Walk detail={detail} room={room} />;
}
