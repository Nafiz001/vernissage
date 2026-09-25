import type { Metadata } from "next";
import Link from "next/link";
import { ExhibitionCard } from "@/components/ExhibitionCard";
import { SiteFooter } from "@/components/SiteFooter";
import { SiteHeader } from "@/components/SiteHeader";
import { serverApi } from "@/lib/api";
import type { Exhibition } from "@/lib/types";

export const metadata: Metadata = {
  title: "Exhibitions",
  description: "Exhibitions hung by visitors from public-domain works at The Met and the Cleveland Museum of Art.",
};

export const dynamic = "force-dynamic";

export default async function ExhibitionsPage({ searchParams }: { searchParams: Promise<{ sort?: string; cursor?: string; offset?: string }> }) {
  const sp = await searchParams;
  const sort = sp.sort === "popular" ? "popular" : "recent";
  const qs = new URLSearchParams({ sort, limit: "24" });
  if (sp.cursor) qs.set("cursor", sp.cursor);
  if (sp.offset) qs.set("offset", sp.offset);
  const { items, next } = await serverApi<{ items: Exhibition[]; next: string | null }>(`/api/exhibitions?${qs}`);

  return (
    <main className="room room-dark plaster min-h-svh [--wall:var(--color-prussian)]">
      <SiteHeader />
      <div className="mx-auto max-w-[1440px] px-5 pb-28 pt-10 sm:px-8">
        <h1 className="lettering text-[clamp(52px,7vw,110px)]">Now showing</h1>
        <p className="mt-5 max-w-[56ch] text-[18px] text-[var(--soft)]">
          Every room here was hung by someone who came to look. Walk in, look around, and applaud the ones you like.
        </p>
        <nav className="mt-10 flex gap-2" aria-label="Sort">
          <Link href="/exhibitions" aria-current={sort === "recent" ? "page" : undefined} className="btn btn-line aria-[current=page]:border-[var(--ink)]">
            Newest
          </Link>
          <Link href="/exhibitions?sort=popular" aria-current={sort === "popular" ? "page" : undefined} className="btn btn-line aria-[current=page]:border-[var(--ink)]">
            Most applauded
          </Link>
        </nav>
        {items.length ? (
          <div className="mt-14 grid gap-x-8 gap-y-16 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((e) => (
              <ExhibitionCard key={e.id} e={e} />
            ))}
          </div>
        ) : (
          <p className="mt-14 text-[18px] text-[var(--soft)]">
            Nothing is open yet.{" "}
            <Link href="/collection" className="underline underline-offset-4">
              Choose a few works
            </Link>{" "}
            and yours can be the first.
          </p>
        )}
        {next && (
          <div className="mt-16 text-center">
            <Link href={`/exhibitions?sort=${sort}&${sort === "popular" ? "offset" : "cursor"}=${next}`} className="btn btn-line">
              More exhibitions
            </Link>
          </div>
        )}
      </div>
      <SiteFooter />
    </main>
  );
}
