import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ExhibitionCard } from "@/components/ExhibitionCard";
import { SiteFooter } from "@/components/SiteFooter";
import { SiteHeader } from "@/components/SiteHeader";
import { ApiError, serverApi } from "@/lib/api";
import type { Exhibition, Owner } from "@/lib/types";

type Profile = { user: Owner & { createdAt: string }; exhibitions: Exhibition[] };

async function load(handle: string) {
  try {
    return await serverApi<Profile>(`/api/users/${encodeURIComponent(handle)}`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound();
    throw e;
  }
}

export async function generateMetadata({ params }: { params: Promise<{ handle: string }> }): Promise<Metadata> {
  const { handle } = await params;
  const { user } = await load(handle);
  return { title: user.name };
}

export default async function ProfilePage({ params }: { params: Promise<{ handle: string }> }) {
  const { handle } = await params;
  const { user, exhibitions } = await load(handle);
  const since = new Date(user.createdAt).toLocaleDateString("en", { month: "long", year: "numeric" });
  return (
    <main className="room room-dark plaster min-h-svh [--wall:var(--color-verdigris)]">
      <SiteHeader />
      <div className="mx-auto max-w-[1440px] px-5 pb-28 pt-10 sm:px-8">
        <div className="flex items-center gap-5">
          <span
            className="lettering grid h-20 w-20 place-items-center rounded-full text-[36px] text-white"
            style={{ background: `oklch(0.55 0.12 ${user.hue})` }}
            aria-hidden
          >
            {user.name.slice(0, 1)}
          </span>
          <div>
            <h1 className="lettering text-[clamp(44px,5vw,76px)]">{user.name}</h1>
            <p className="text-[var(--soft)]">Curating since {since}</p>
          </div>
        </div>
        {exhibitions.length ? (
          <div className="mt-16 grid gap-x-8 gap-y-16 sm:grid-cols-2 lg:grid-cols-3">
            {exhibitions.map((e) => (
              <ExhibitionCard key={e.id} e={e} />
            ))}
          </div>
        ) : (
          <p className="mt-14 text-[18px] text-[var(--soft)]">No exhibitions open yet.</p>
        )}
      </div>
      <SiteFooter />
    </main>
  );
}
