import type { Metadata } from "next";
import { cookies } from "next/headers";
import { notFound } from "next/navigation";
import { ApiError, serverApi } from "@/lib/api";
import type { ExhibitionDetail } from "@/lib/types";
import { ExhibitionView } from "./ExhibitionView";

async function load(slug: string) {
  const jar = await cookies();
  try {
    return await serverApi<ExhibitionDetail>(`/api/exhibitions/${encodeURIComponent(slug)}`, jar.toString());
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound();
    throw e;
  }
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const { exhibition: e } = await load(slug);
  const description = e.statement.slice(0, 200) || `An exhibition of ${e.workCount} works curated by ${e.owner.name}.`;
  return {
    title: e.title,
    description,
    openGraph: {
      title: e.title,
      description,
      images: e.status === "published" ? [{ url: `/poster/${e.slug}.jpg?v=${e.posterVersion}`, width: 1200, height: 630 }] : [],
    },
    twitter: { card: "summary_large_image" },
  };
}

export default async function ExhibitionPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  return <ExhibitionView detail={await load(slug)} />;
}
