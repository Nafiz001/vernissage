import type { Metadata } from "next";
import { cookies } from "next/headers";
import { notFound } from "next/navigation";
import { ApiError, serverApi } from "@/lib/api";
import type { ExhibitionDetail, Rooms } from "@/lib/types";
import { WalkLoader } from "./WalkLoader";

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  return { title: `Walking through ${slug.replace(/-[a-z0-9]{4}$/, "").replace(/-/g, " ")}` };
}

export default async function WalkPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const jar = await cookies();
  let detail: ExhibitionDetail;
  let rooms: Rooms;
  try {
    [detail, rooms] = await Promise.all([
      serverApi<ExhibitionDetail>(`/api/exhibitions/${encodeURIComponent(slug)}`, jar.toString()),
      serverApi<Rooms>("/api/rooms", undefined, 300),
    ]);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound();
    throw e;
  }
  const room = rooms.rooms.find((r) => r.key === detail.exhibition.room) ?? rooms.rooms[0]!;
  return <WalkLoader detail={detail} room={room} />;
}
