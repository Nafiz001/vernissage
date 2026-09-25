"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { useMe, useUI } from "@/lib/session";

const links = [
  { href: "/collection", label: "Collection" },
  { href: "/exhibitions", label: "Exhibitions" },
];

/**
 * The top of every room. It takes its colours from the room it sits in, so
 * it reads on red, green, blue or white walls alike.
 */
export function SiteHeader({ className = "" }: { className?: string }) {
  const path = usePathname();
  const { data } = useMe();
  const ask = useUI((s) => s.askSignIn);
  const setTray = useUI((s) => s.setTray);
  const qc = useQueryClient();
  const router = useRouter();
  const [menu, setMenu] = useState(false);
  const user = data?.user;
  const saved = data?.saved?.length ?? 0;

  async function signOut() {
    await api("/api/auth/logout", { method: "POST" });
    await qc.invalidateQueries();
    setMenu(false);
    router.push("/");
  }

  return (
    <header className={`relative z-30 mx-auto flex max-w-[1440px] items-center gap-2 px-5 py-4 sm:px-8 ${className}`}>
      <Link href="/" className="lettering mr-auto text-[28px] italic leading-none sm:text-[32px]">
        Vernissage
      </Link>
      <nav className="flex items-center gap-1" aria-label="Main">
        {links.map((l) => (
          <Link
            key={l.href}
            href={l.href}
            aria-current={path.startsWith(l.href) ? "page" : undefined}
            className="btn btn-quiet hidden text-[15px] aria-[current=page]:underline aria-[current=page]:underline-offset-8 sm:inline-flex"
          >
            {l.label}
          </Link>
        ))}
        {user ? (
          <>
            {saved > 0 && (
              <button className="btn btn-quiet text-[15px]" onClick={() => setTray(true)}>
                Selection
                <span className="bg-gilt text-ink grid h-6 min-w-6 place-items-center rounded-full px-1.5 text-[12px] font-semibold">{saved}</span>
              </button>
            )}
            <div className="relative">
              <button
                className="btn btn-line ml-1 gap-2 pl-1.5 text-[15px]"
                aria-expanded={menu}
                aria-haspopup="menu"
                onClick={() => setMenu((m) => !m)}
              >
                <span
                  className="grid h-8 w-8 place-items-center rounded-full text-[12px] font-semibold text-white"
                  style={{ background: `oklch(0.55 0.12 ${user.hue})` }}
                  aria-hidden
                >
                  {user.name.slice(0, 1)}
                </span>
                <span className="hidden sm:inline">{user.name.split(" ")[0]}</span>
              </button>
              {menu && (
                <div
                  role="menu"
                  className="bg-card text-ink absolute right-0 top-[calc(100%+8px)] w-56 overflow-hidden rounded-xl py-1.5 shadow-2xl"
                  onMouseLeave={() => setMenu(false)}
                >
                  <MenuLink href="/studio" onClick={() => setMenu(false)}>Your exhibitions</MenuLink>
                  <MenuLink href={`/u/${user.handle}`} onClick={() => setMenu(false)}>Your public page</MenuLink>
                  <MenuLink href="/collection" onClick={() => setMenu(false)} className="sm:hidden">Collection</MenuLink>
                  <MenuLink href="/exhibitions" onClick={() => setMenu(false)} className="sm:hidden">Exhibitions</MenuLink>
                  <button role="menuitem" className="block w-full px-4 py-2.5 text-left text-[15px] hover:bg-black/5" onClick={signOut}>
                    Sign out
                  </button>
                </div>
              )}
            </div>
          </>
        ) : (
          <button className="btn btn-line ml-1 text-[15px]" onClick={() => ask()}>
            Sign in
          </button>
        )}
      </nav>
    </header>
  );
}

function MenuLink({ href, children, onClick, className = "" }: { href: string; children: React.ReactNode; onClick: () => void; className?: string }) {
  return (
    <Link role="menuitem" href={href} onClick={onClick} className={`block px-4 py-2.5 text-[15px] hover:bg-black/5 ${className}`}>
      {children}
    </Link>
  );
}
