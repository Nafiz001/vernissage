import { dimensions } from "@/lib/format";
import type { Artwork } from "@/lib/types";

/**
 * The card beside a work in a museum: who made it, what it is, and whose it
 * is, one fact to a line in the order curators write them.
 */
export function WallLabel({
  artwork,
  className = "",
  note,
  credit = true,
}: {
  artwork: Artwork;
  className?: string;
  note?: string;
  credit?: boolean;
}) {
  const dims = dimensions(artwork);
  return (
    <div
      className={`bg-card text-ink rounded-[2px] px-5 py-4 text-[14px] leading-[1.45] shadow-[0_1px_1px_rgba(0,0,0,.25),0_8px_20px_-12px_rgba(0,0,0,.5)] ${className}`}
    >
      {artwork.artist && <p className="font-semibold">{artwork.artist}</p>}
      {artwork.artistBio && <p className="narrow text-ink/70">{artwork.artistBio}</p>}
      <p className="mt-3">
        <span className="italic">{artwork.title}</span>
        {artwork.date && <span>, {artwork.date}</span>}
      </p>
      {artwork.medium && <p className="narrow text-ink/80">{artwork.medium}</p>}
      {dims && <p className="narrow text-ink/80">{dims}</p>}
      {note && <p className="lettering-sm mt-3 border-t border-ink/15 pt-3 text-[15px] italic">{note}</p>}
      {credit && (
        <p className="narrow mt-3 text-[12.5px] text-ink/60">
          {artwork.museum.name}
          {artwork.credit ? <><br />{artwork.credit}</> : null}
        </p>
      )}
    </div>
  );
}
