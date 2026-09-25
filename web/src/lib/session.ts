"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { create } from "zustand";
import { api } from "./api";
import type { User } from "./types";

type Me = { user: User | null; saved?: number[] };

export function useMe() {
  return useQuery({ queryKey: ["me"], queryFn: () => api<Me>("/api/me"), staleTime: 60_000 });
}

export function useSavedSet() {
  const { data } = useMe();
  return new Set(data?.saved ?? []);
}

/** UI state shared across the site. */
export const useUI = create<{
  signIn: { open: boolean; reason?: string };
  askSignIn: (reason?: string) => void;
  closeSignIn: () => void;
  trayOpen: boolean;
  setTray: (open: boolean) => void;
}>((set) => ({
  signIn: { open: false },
  askSignIn: (reason) => set({ signIn: { open: true, reason } }),
  closeSignIn: () => set({ signIn: { open: false } }),
  trayOpen: false,
  setTray: (trayOpen) => set({ trayOpen }),
}));

/** Save a work to your selection, or put it back; asks guests to sign in. */
export function useToggleSaved() {
  const qc = useQueryClient();
  const { data } = useMe();
  const ask = useUI((s) => s.askSignIn);
  const m = useMutation({
    mutationFn: ({ id, on }: { id: number; on: boolean }) =>
      api(`/api/me/saved/${id}`, { method: on ? "PUT" : "DELETE" }),
    onMutate: async ({ id, on }) => {
      await qc.cancelQueries({ queryKey: ["me"] });
      const prev = qc.getQueryData<Me>(["me"]);
      qc.setQueryData<Me>(["me"], (old) =>
        old ? { ...old, saved: on ? [id, ...(old.saved ?? [])] : (old.saved ?? []).filter((s) => s !== id) } : old,
      );
      return { prev };
    },
    onError: (_e, _v, ctx) => ctx?.prev && qc.setQueryData(["me"], ctx.prev),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ["me"] });
      qc.invalidateQueries({ queryKey: ["saved"] });
    },
  });
  return (id: number, on: boolean) => {
    if (!data?.user) {
      ask("Sign in to keep a selection of works and hang them.");
      return;
    }
    m.mutate({ id, on });
  };
}
