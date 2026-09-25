import Link from "next/link";
import { SiteHeader } from "@/components/SiteHeader";

export default function NotFound() {
  return (
    <main className="room room-dark plaster min-h-svh [--wall:var(--color-lamp)]">
      <SiteHeader />
      <div className="mx-auto max-w-[1440px] px-5 py-32 text-center sm:px-8">
        <p className="lettering text-[clamp(52px,7vw,104px)]">This room is empty.</p>
        <p className="mx-auto mt-5 max-w-[46ch] text-[18px] text-[var(--soft)]">
          The page you asked for isn&apos;t here. It may have been taken down, or it was never hung.
        </p>
        <div className="mt-10 flex justify-center gap-3">
          <Link href="/collection" className="btn btn-solid">
            Browse the collection
          </Link>
          <Link href="/exhibitions" className="btn btn-line">
            See what&apos;s showing
          </Link>
        </div>
      </div>
    </main>
  );
}
