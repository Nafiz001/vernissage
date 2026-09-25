import type { Metadata } from "next";
import { cookies } from "next/headers";
import { notFound } from "next/navigation";
import { ApiError, serverApi } from "@/lib/api";
import type { ArtworkDetail } from "@/lib/types";
import { ArtworkView } from "./ArtworkView";

async function load(id: string) {
  const jar = await cookies();
  try {
    return await serverApi<ArtworkDetail>(`/api/artworks/${encodeURIComponent(id)}`, jar.toString());
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound();
    throw e;
  }
}

export async function generateMetadata({ params }: { params: Promise<{ id: string }> }): Promise<Metadata> {
  const { id } = await params;
  const { artwork: a } = await load(id);
  const title = a.artist ? `${a.title}, ${a.artist}` : a.title;
  return {
    title,
    description: a.description?.slice(0, 200) || `${a.medium}. ${a.museum.name}.`,
    openGraph: { title, images: [{ url: `/img/${a.id}/1200.jpg` }] },
  };
}

export default async function ArtworkPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const detail = await load(id);
  return <ArtworkView detail={detail} />;
}
