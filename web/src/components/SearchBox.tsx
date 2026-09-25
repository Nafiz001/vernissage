"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useId, useRef, useState } from "react";
import { api } from "@/lib/api";
import { blurhashURL } from "./Framed";

type Suggestion = {
  artists: { name: string; count: number }[];
  works: { id: number; title: string; artist: string; blurhash: string }[];
};

function useDebounced<T>(value: T, ms: number) {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

/**
 * The collection's search field. As you type it suggests artists (with how
 * many of their works are here) and titles; Enter searches everything.
 */
export function SearchBox({
  value,
  onSearch,
  onArtist,
  placeholder,
}: {
  value: string;
  onSearch: (q: string) => void;
  onArtist: (name: string) => void;
  placeholder: string;
}) {
  const [text, setText] = useState(value);
  const [shown, setShown] = useState(value);
  if (value !== shown) {
    setShown(value);
    setText(value);
  }
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  const q = useDebounced(text.trim(), 160);
  const listId = useId();
  const wrap = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const close = (e: PointerEvent) => !wrap.current?.contains(e.target as Node) && setOpen(false);
    window.addEventListener("pointerdown", close);
    return () => window.removeEventListener("pointerdown", close);
  }, []);

  const { data } = useQuery({
    queryKey: ["suggest", q],
    queryFn: () => api<Suggestion>(`/api/suggest?q=${encodeURIComponent(q)}`),
    enabled: q.length >= 2,
    staleTime: 60_000,
  });
  const options = [
    ...(data?.artists ?? []).map((a) => ({ kind: "artist" as const, key: `a-${a.name}`, a })),
    ...(data?.works ?? []).map((w) => ({ kind: "work" as const, key: `w-${w.id}`, w })),
  ];
  const show = open && q.length >= 2 && options.length > 0;

  function choose(i: number) {
    const o = options[i];
    if (!o) return;
    setOpen(false);
    if (o.kind === "artist") {
      setText("");
      onArtist(o.a.name);
    } else {
      router.push(`/artwork/${o.w.id}`);
    }
  }

  return (
    <div ref={wrap} className="relative">
      <form
        role="search"
        onSubmit={(e) => {
          e.preventDefault();
          if (show && active >= 0) return choose(active);
          setOpen(false);
          onSearch(text.trim());
        }}
      >
        <input
          type="search"
          value={text}
          onChange={(e) => {
            setText(e.target.value);
            setOpen(true);
            setActive(-1);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={(e) => {
            if (!show) return;
            if (e.key === "ArrowDown") {
              e.preventDefault();
              setActive((a) => Math.min(options.length - 1, a + 1));
            } else if (e.key === "ArrowUp") {
              e.preventDefault();
              setActive((a) => Math.max(-1, a - 1));
            } else if (e.key === "Escape") {
              setOpen(false);
            }
          }}
          placeholder={placeholder}
          aria-label="Search the collection"
          role="combobox"
          aria-expanded={show}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={active >= 0 ? `${listId}-${active}` : undefined}
          className="lettering-sm w-full border-b border-[var(--line)] bg-transparent pb-3 text-[clamp(30px,3.6vw,52px)] italic placeholder:text-[var(--soft)] focus:border-[var(--ink)] focus:outline-none"
        />
      </form>
      {show && (
        <ul id={listId} role="listbox" className="bg-card text-ink absolute inset-x-0 top-[calc(100%+10px)] z-40 max-h-[60vh] overflow-auto rounded-xl py-2 shadow-2xl">
          {options.map((o, i) => (
            <li
              key={o.key}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={i === active}
              onMouseEnter={() => setActive(i)}
              onMouseDown={(e) => {
                e.preventDefault();
                choose(i);
              }}
              className="flex cursor-pointer items-center gap-3 px-4 py-2.5 aria-selected:bg-black/5"
            >
              {o.kind === "artist" ? (
                <>
                  <span className="grid h-9 w-9 place-items-center rounded-full bg-black/5 text-[13px]" aria-hidden>
                    {o.a.name.slice(0, 1)}
                  </span>
                  <span className="flex-1">
                    <span className="font-medium">{o.a.name}</span>
                    <span className="ml-2 text-[14px] text-black/55">{o.a.count} {o.a.count === 1 ? "work" : "works"}</span>
                  </span>
                  <span className="text-[13px] text-black/50">Artist</span>
                </>
              ) : (
                <WorkOption w={o.w} />
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function WorkOption({ w }: { w: Suggestion["works"][number] }) {
  const blur = blurhashURL(w.blurhash);
  return (
    <>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={`/img/${w.id}/200.jpg`} alt="" className="h-9 w-9 rounded-[2px] object-cover" style={{ background: blur ? `url(${blur}) center/cover` : "#ddd" }} />
      <span className="flex-1 truncate">
        <span className="italic">{w.title}</span>
        {w.artist && <span className="text-black/55">, {w.artist}</span>}
      </span>
      <Link href={`/artwork/${w.id}`} className="text-[13px] text-black/50" tabIndex={-1}>
        Work
      </Link>
    </>
  );
}
