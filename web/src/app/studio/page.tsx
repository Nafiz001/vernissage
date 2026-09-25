"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { AuthForm } from "@/components/AuthDialog";
import { ExhibitionCard } from "@/components/ExhibitionCard";
import { SiteHeader } from "@/components/SiteHeader";
import { api } from "@/lib/api";
import { useMe } from "@/lib/session";
import type { Exhibition, ExhibitionDetail } from "@/lib/types";

export default function StudioIndex() {
  const { data: me, isLoading } = useMe();
  const router = useRouter();
  const mine = useQuery({
    queryKey: ["mine"],
    queryFn: () => api<{ items: Exhibition[] }>("/api/me/exhibitions"),
    enabled: !!me?.user,
  });
  const create = useMutation({
    mutationFn: () => api<ExhibitionDetail>("/api/exhibitions", { method: "POST", json: { title: "Untitled exhibition" } }),
    onSuccess: (d) => router.push(`/studio/${d.exhibition.id}`),
  });

  return (
    <main className="room room-light plaster min-h-svh [--wall:var(--color-chalk)]">
      <SiteHeader />
      <div className="mx-auto max-w-[1440px] px-5 pb-28 pt-10 sm:px-8">
        {isLoading ? null : !me?.user ? (
          <div className="mx-auto max-w-[420px]">
            <AuthForm reason="Sign in to hang and open exhibitions." />
          </div>
        ) : (
          <>
            <div className="flex flex-wrap items-end justify-between gap-6">
              <div>
                <h1 className="lettering text-[clamp(48px,6vw,92px)]">Your exhibitions</h1>
                <p className="mt-3 max-w-[52ch] text-[17px] text-[var(--soft)]">
                  Drafts are yours alone until you open the doors. Save works from the{" "}
                  <Link href="/collection" className="underline underline-offset-4">
                    collection
                  </Link>{" "}
                  and they&apos;ll be waiting in the studio.
                </p>
              </div>
              <button className="btn btn-solid" onClick={() => create.mutate()} disabled={create.isPending}>
                {create.isPending ? "Preparing a room…" : "New exhibition"}
              </button>
            </div>
            {mine.data && mine.data.items.length === 0 && (
              <p className="mt-16 text-[18px] text-[var(--soft)]">
                Nothing yet. Start a new exhibition, or save a few works first and hang them all at once.
              </p>
            )}
            <div className="mt-14 grid gap-x-8 gap-y-16 sm:grid-cols-2 lg:grid-cols-3">
              {mine.data?.items.map((e) => (
                <ExhibitionCard key={e.id} e={e} href={`/studio/${e.id}`} />
              ))}
            </div>
          </>
        )}
      </div>
    </main>
  );
}
