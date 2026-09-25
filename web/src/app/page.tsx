import Link from "next/link";
import { ExhibitionCard } from "@/components/ExhibitionCard";
import { Framed } from "@/components/Framed";
import { HeroReveal } from "@/components/HeroReveal";
import { PigmentSearch } from "@/components/PigmentSearch";
import { SalonCluster, Skirting } from "@/components/Salon";
import { SiteHeader } from "@/components/SiteHeader";
import { SiteFooter } from "@/components/SiteFooter";
import { serverApi } from "@/lib/api";
import type { Artwork, ArtworkPage, Exhibition, Stats } from "@/lib/types";

export const dynamic = "force-dynamic";

async function load() {
  const seed = Math.floor(Date.now() / 86_400_000);
  const [featured, stats, shows, salon] = await Promise.all([
    serverApi<{ items: Artwork[] }>("/api/artworks/featured"),
    serverApi<Stats>("/api/stats"),
    serverApi<{ items: Exhibition[] }>("/api/exhibitions?limit=6"),
    serverApi<ArtworkPage>(`/api/artworks?kind=painting&limit=24&seed=${seed}`),
  ]);
  return { featured: featured.items, stats: stats.collection, shows: shows.items, salon: salon.items };
}

export default async function Home() {
  let data: Awaited<ReturnType<typeof load>>;
  try {
    data = await load();
  } catch {
    return <Closed />;
  }
  const { featured, stats, shows, salon } = data;
  const hungWorks = salon.filter((a) => a.image.ready).slice(0, 8);

  return (
    <main>
      <section className="room room-dark plaster [--wall:var(--color-oxblood)]">
        <SiteHeader />
        <HeroReveal works={featured} total={stats.works} />
        <Skirting />
      </section>

      <section className="room room-dark plaster [--wall:var(--color-verdigris)]" aria-labelledby="collection-title">
        <div className="mx-auto grid max-w-[1440px] gap-14 px-6 py-28 sm:px-8 lg:grid-cols-[minmax(0,4fr)_minmax(0,8fr)] lg:gap-16">
          <div className="lg:sticky lg:top-24 lg:self-start">
            <h2 id="collection-title" className="lettering text-[clamp(44px,5.4vw,84px)]">
              Every work here belongs to everyone.
            </h2>
            <p className="mt-7 max-w-[38ch] text-[18px] text-[var(--soft)]">
              The Met and the Cleveland Museum of Art photograph the works in their collections that are out of copyright
              and give the pictures away. {stats.works.toLocaleString("en")} of them are here: paintings, woodblock prints
              and drawings from {stats.earliest} to {stats.latest}, by {stats.artists.toLocaleString("en")} artists.
            </p>
            <Link href="/collection" className="btn btn-solid mt-9">
              Browse the collection
            </Link>
          </div>
          <SalonCluster works={salon} />
        </div>
        <Skirting />
      </section>

      <section className="room room-light plaster [--wall:var(--color-chalk)]" aria-labelledby="colour-title">
        <div className="mx-auto max-w-[1440px] px-6 py-28 sm:px-8">
          <h2 id="colour-title" className="lettering max-w-[16ch] text-[clamp(44px,5.4vw,84px)]">
            Search by the colour in it.
          </h2>
          <p className="mt-6 max-w-[52ch] text-[18px] text-[var(--soft)]">
            The server has looked at every picture and knows the handful of colours it&apos;s made of, and how much of each.
            Pick a pigment to see the works that are full of it.
          </p>
          <PigmentSearch />
        </div>
        <Skirting />
      </section>

      <section className="room room-light plaster [--wall:var(--color-plaster)]" aria-labelledby="how-title">
        <div className="mx-auto max-w-[1440px] px-6 py-28 sm:px-8">
          <h2 id="how-title" className="lettering max-w-[18ch] text-[clamp(44px,5.4vw,84px)]">
            From a few favourites to an opening night.
          </h2>
          <ol className="mt-16 grid gap-14 md:grid-cols-3 md:gap-10">
            <Step n={1} title="Choose">
              Save the works that stop you as you browse. Search by artist, subject or century, or by a colour you
              can&apos;t stop looking at.
              <div className="mt-8 flex h-[150px] items-end gap-3">
                {hungWorks.slice(0, 3).map((a, i) => (
                  <Framed key={a.id} artwork={a} width={[96, 124, 84][i]!} maxSrc={400} />
                ))}
              </div>
            </Step>
            <Step n={2} title="Hang">
              Pick a room and a wall colour, then place your works on the walls. Everything hangs at its real size, so a
              print the size of a postcard looks like one. Or press Hang for me and adjust from there.
              <Elevation works={hungWorks.slice(3, 7)} />
            </Step>
            <Step n={3} title="Open the doors">
              Send the link. Friends walk through the room with you in 3D, see where everyone is standing, applaud,
              and sign the guestbook on the way out.
              <Plan />
            </Step>
          </ol>
        </div>
        <Skirting />
      </section>

      <section className="room room-dark plaster [--wall:var(--color-prussian)]" aria-labelledby="showing-title">
        <div className="mx-auto max-w-[1440px] px-6 py-28 sm:px-8">
          <div className="flex flex-wrap items-end justify-between gap-6">
            <h2 id="showing-title" className="lettering text-[clamp(44px,5.4vw,84px)]">
              Now showing
            </h2>
            <Link href="/exhibitions" className="btn btn-line">
              All exhibitions
            </Link>
          </div>
          {shows.length ? (
            <div className="mt-14 grid gap-x-8 gap-y-14 sm:grid-cols-2 lg:grid-cols-3">
              {shows.map((e) => (
                <ExhibitionCard key={e.id} e={e} />
              ))}
            </div>
          ) : (
            <p className="mt-10 max-w-[48ch] text-[18px] text-[var(--soft)]">
              No exhibitions are open yet. Choose a few works from the collection and yours can be the first.
            </p>
          )}
        </div>
        <Skirting />
      </section>

      <SiteFooter />
    </main>
  );
}

function Step({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <li className="flex flex-col">
      <span className="lettering text-[64px] leading-none text-[var(--soft)]" aria-hidden>
        {n}
      </span>
      <h3 className="lettering-sm mt-3 text-[34px]">{title}</h3>
      <div className="mt-3 max-w-[40ch] text-[17px] text-[var(--soft)]">{children}</div>
    </li>
  );
}

/** A wall elevation, the way the studio draws one. */
function Elevation({ works }: { works: Artwork[] }) {
  const scale = 46; // px per metre
  const wall = 6.4;
  const total = works.reduce((s, a) => s + a.hang.w, 0);
  const gap = (wall - total) / (works.length + 1);
  const lefts = works.map((_, i) => gap * (i + 1) + works.slice(0, i).reduce((s, a) => s + a.hang.w, 0));
  return (
    <div className="mt-8">
      <div className="relative h-[150px] border-b-2 border-[var(--ink)]/60" style={{ width: wall * scale, maxWidth: "100%" }}>
        <div className="absolute inset-x-0 border-t border-dashed border-[var(--ink)]/40" style={{ bottom: 1.48 * scale }} aria-hidden />
        {works.map((a, i) => {
          const left = lefts[i]! * scale;
          return (
            <div key={a.id} className="absolute" style={{ left, bottom: (1.48 - a.hang.h / 2) * scale }}>
              <Framed artwork={a} width={a.hang.w * scale} maxSrc={400} shadow={false} />
            </div>
          );
        })}
        {/* A person, for scale. */}
        <svg className="absolute bottom-0 right-1 text-[var(--ink)]/50" width={0.5 * scale} height={1.72 * scale} viewBox="0 0 20 70" aria-hidden>
          <circle cx="10" cy="6" r="5" fill="currentColor" />
          <path d="M4 14h12l2 26h-3l-1 30h-4l-1-20-1 20H4l-1-30H0z" fill="currentColor" />
        </svg>
      </div>
      <p className="mt-2 text-[13px] text-[var(--soft)]">Real sizes, a person for scale.</p>
    </div>
  );
}

/** A room from above, with visitors in it. */
function Plan() {
  const people = [
    { x: 30, y: 42, hue: 20 },
    { x: 62, y: 30, hue: 220 },
    { x: 70, y: 66, hue: 140 },
    { x: 44, y: 70, hue: 300 },
  ];
  return (
    <svg viewBox="0 0 100 80" className="mt-8 h-[150px] w-auto text-[var(--ink)]" aria-hidden>
      <rect x="4" y="4" width="92" height="72" fill="none" stroke="currentColor" strokeOpacity=".6" strokeWidth="1.6" />
      <rect x="42" y="74.5" width="16" height="3" className="fill-[var(--wall)]" />
      {[14, 34, 58, 78].map((x) => (
        <rect key={x} x={x} y="3" width="10" height="2.4" fill="currentColor" />
      ))}
      <rect x="2.6" y="26" width="2.4" height="12" fill="currentColor" />
      <rect x="95" y="34" width="2.4" height="16" fill="currentColor" />
      {people.map((p, i) => (
        <g key={i}>
          <circle cx={p.x} cy={p.y} r="3.2" fill={`oklch(0.6 0.13 ${p.hue})`} />
          <circle cx={p.x} cy={p.y} r="6" fill={`oklch(0.6 0.13 ${p.hue})`} opacity=".18" />
        </g>
      ))}
    </svg>
  );
}

function Closed() {
  return (
    <main className="room room-dark plaster grid min-h-svh place-items-center px-6 text-center [--wall:var(--color-lamp)]">
      <div>
        <p className="lettering text-[56px]">The doors are closed.</p>
        <p className="mt-4 text-[var(--soft)]">The server isn&apos;t answering. Start it with make dev, then reload this page.</p>
      </div>
    </main>
  );
}
