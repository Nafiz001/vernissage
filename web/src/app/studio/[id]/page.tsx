import type { Metadata } from "next";
import { Suspense } from "react";
import { Studio } from "@/studio/Studio";

export const metadata: Metadata = { title: "Studio" };

export default async function StudioPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <Suspense>
      <Studio id={id} />
    </Suspense>
  );
}
