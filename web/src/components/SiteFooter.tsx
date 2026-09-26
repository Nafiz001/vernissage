import Link from "next/link";

export function SiteFooter() {
  return (
    <footer className="room room-dark [--wall:var(--color-lamp)]">
      <div className="mx-auto grid max-w-[1440px] gap-10 px-6 py-16 text-[15px] sm:px-8 md:grid-cols-[2fr_1fr_1fr]">
        <div>
          <p className="lettering text-[34px] italic">Vernissage</p>
          <p className="mt-3 max-w-[52ch] text-[var(--soft)]">
            Pictures and information from{" "}
            <a className="underline underline-offset-4" href="https://www.metmuseum.org/about-the-met/policies-and-documents/open-access">
              The Metropolitan Museum of Art
            </a>{" "}
            and{" "}
            <a className="underline underline-offset-4" href="https://www.clevelandart.org/open-access">
              the Cleveland Museum of Art
            </a>
            , both released under Creative Commons Zero. Neither museum is involved in this site.
          </p>
        </div>
        <nav aria-label="Footer" className="flex flex-col gap-2">
          <Link href="/collection" className="hover:underline">Collection</Link>
          <Link href="/exhibitions" className="hover:underline">Exhibitions</Link>
          <Link href="/studio" className="hover:underline">Your exhibitions</Link>
        </nav>
        <div className="flex flex-col gap-2 text-[var(--soft)]">
          <a href="https://github.com/Nafiz001/vernissage" className="hover:underline">Source code</a>
          <span>Type set in Bodoni Moda and Archivo</span>
        </div>
      </div>
    </footer>
  );
}
