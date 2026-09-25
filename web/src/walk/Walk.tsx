"use client";

import { useEffect, useMemo } from "react";
import type { ExhibitionDetail, Room } from "@/lib/types";
import { Hud } from "./Hud";
import { useLive } from "./live";
import { Scene } from "./Scene";
import { useRoomSound } from "./sound";
import { useWalk } from "./store";

export default function Walk({ detail, room }: { detail: ExhibitionDetail; room: Room }) {
  const e = detail.exhibition;
  const works = useMemo(() => new Map(detail.works.map((w) => [w.id, w])), [detail.works]);
  const conn = useLive(e.slug);
  useRoomSound(room.width * room.depth > 120);

  // Phones and small laptops start at a lighter setting.
  useEffect(() => {
    const small = window.matchMedia("(max-width: 800px)").matches || navigator.hardwareConcurrency <= 4;
    useWalk.setState({ quality: small ? "medium" : "high", entered: false, viewing: null, touring: false });
  }, []);

  return (
    <main className="fixed inset-0 bg-[#0b0a09]">
      <Scene e={e} room={room} placements={detail.placements} works={works} />
      <Hud e={e} room={room} placements={detail.placements} works={works} conn={conn} />
    </main>
  );
}
