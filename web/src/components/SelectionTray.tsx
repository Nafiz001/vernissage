"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import { count } from "@/lib/format";
import { useMe, useToggleSaved, useUI } from "@/lib/session";
import type { Artwork, ExhibitionDetail } from "@/lib/types";
import { Framed } from "./Framed";

/**
 * Your selection: the works you've set aside while browsing, ready to be
 * hung together. It slides up from the bottom of any page.
 */
export function SelectionTray() {
  const open = useUI((s) => s.trayOpen);
  const setOpen = useUI((s) => s.setTray);
  const path = usePathname();
  const router = useRouter();
  const { data: me } = useMe();
  const toggle = useToggleSaved();
  const [title, setTitle] = useState("");
  const { data } = useQuery({
    queryKey: ["saved"],
    queryFn: () => api<{ items: Artwork[] }>("/api/me/saved"),
    enabled: open && !!me?.user,
  });
  const create = useMutation({
    mutationFn: (ids: number[]) =>
      api<ExhibitionDetail>("/api/exhibitions", { method: "POST", json: { title: title || "Untitled exhibition", artworkIds: ids } }),
    onSuccess: (d) => {
      setOpen(false);
      router.push(`/studio/${d.exhibition.id}`);
    },
  });
  if (path.endsWith("/walk") || !me?.user) return null;
  const items = data?.items ?? [];
  const fits = items.slice(0, 40);

  return (
    <AnimatePresence>
      {open && (
        <motion.aside
          aria-label="Your selection"
          className="room room-light fixed inset-x-0 bottom-0 z-[90] border-t border-black/10 shadow-[0_-20px_60px_-20px_rgba(0,0,0,.5)] [--wall:var(--color-chalk)]"
          initial={{ y: "100%" }}
          animate={{ y: 0 }}
          exit={{ y: "100%" }}
          transition={{ duration: 0.45, ease: [0.65, 0, 0.35, 1] }}
        >
          <div className="mx-auto max-w-[1440px] px-5 py-5 sm:px-8">
            <div className="flex flex-wrap items-center gap-3">
              <h2 className="lettering text-[30px]">Your selection</h2>
              <span className="text-[15px] text-[var(--soft)]">{count(items.length, "work", "works")}</span>
              <button className="btn btn-quiet ml-auto" onClick={() => setOpen(false)}>
                Close
              </button>
            </div>
            {items.length === 0 ? (
              <p className="py-8 text-[var(--soft)]">
                Nothing set aside yet. Save works from the{" "}
                <Link className="underline" href="/collection" onClick={() => setOpen(false)}>
                  collection
                </Link>{" "}
                and they&apos;ll wait for you here.
              </p>
            ) : (
              <>
                <ul className="-mx-1 mt-4 flex items-end gap-5 overflow-x-auto px-1 pb-4">
                  {items.map((a) => (
                    <li key={a.id} className="group relative shrink-0">
                      <Link href={`/artwork/${a.id}`} onClick={() => setOpen(false)}>
                        <Framed artwork={a} width={Math.max(70, Math.min(150, 120 * Math.sqrt(a.hang.w / Math.max(a.hang.h, 0.3))))} maxSrc={400} />
                      </Link>
                      <button
                        className="bg-card text-ink absolute -right-2 -top-2 hidden h-7 w-7 place-items-center rounded-full text-[15px] shadow group-hover:grid"
                        aria-label={`Remove ${a.title}`}
                        onClick={() => toggle(a.id, false)}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
                <form
                  className="flex flex-wrap items-center gap-3 border-t border-[var(--line)] pt-4"
                  onSubmit={(e) => {
                    e.preventDefault();
                    create.mutate(fits.map((a) => a.id));
                  }}
                >
                  <input
                    className="field max-w-sm flex-1"
                    placeholder="Name the exhibition"
                    value={title}
                    maxLength={90}
                    onChange={(e) => setTitle(e.target.value)}
                    aria-label="Exhibition title"
                  />
                  <button className="btn btn-solid" disabled={create.isPending}>
                    {create.isPending ? "Hanging…" : `Hang ${fits.length === items.length ? "these" : `the first ${fits.length}`} in a new exhibition`}
                  </button>
                  {create.error && <p className="text-alarm text-[14px]">{(create.error as ApiError).message}</p>}
                </form>
              </>
            )}
          </div>
        </motion.aside>
      )}
    </AnimatePresence>
  );
}
