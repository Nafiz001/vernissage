"use client";

import { useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion } from "motion/react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { api, ApiError } from "@/lib/api";
import { useUI } from "@/lib/session";

const DEMO = { email: "demo@vernissage.local", password: "open-the-doors" };

export function AuthDialog() {
  const { open, reason } = useUI((s) => s.signIn);
  const close = useUI((s) => s.closeSignIn);
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && close();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, close]);
  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="fixed inset-0 z-[100] grid place-items-center bg-black/55 p-4 backdrop-blur-[2px]"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onMouseDown={(e) => e.target === e.currentTarget && close()}
        >
          <motion.div
            role="dialog"
            aria-modal="true"
            aria-labelledby="auth-title"
            className="room room-light plaster w-full max-w-[420px] rounded-2xl p-7 shadow-2xl [--wall:var(--color-chalk)]"
            initial={{ y: 24, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            exit={{ y: 12, opacity: 0 }}
            transition={{ duration: 0.35, ease: [0.65, 0, 0.35, 1] }}
          >
            <AuthForm reason={reason} onDone={close} />
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

export function AuthForm({ reason, onDone }: { reason?: string; onDone?: () => void }) {
  const [mode, setMode] = useState<"in" | "join">("in");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  const form = useRef<HTMLFormElement>(null);
  const qc = useQueryClient();

  async function submit(body: Record<string, string>) {
    setBusy(true);
    setError(null);
    setFields({});
    try {
      await api(mode === "in" ? "/api/auth/login" : "/api/auth/signup", { method: "POST", json: body });
      await qc.invalidateQueries();
      onDone?.();
    } catch (e) {
      if (e instanceof ApiError) {
        setError(e.message);
        setFields(e.fields ?? {});
      }
    } finally {
      setBusy(false);
    }
  }

  function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
    submit(data);
  }

  function useDemo() {
    setMode("in");
    const f = form.current;
    if (f) {
      (f.elements.namedItem("email") as HTMLInputElement).value = DEMO.email;
      (f.elements.namedItem("password") as HTMLInputElement).value = DEMO.password;
    }
    submit(DEMO);
  }

  return (
    <div>
      <h2 id="auth-title" className="lettering text-[40px]">
        {mode === "in" ? "Welcome back" : "Become a curator"}
      </h2>
      <p className="mt-2 text-[15px] text-[var(--soft)]">
        {reason ?? (mode === "in" ? "Sign in to hang exhibitions and keep a selection." : "An account lets you save works and open exhibitions of your own.")}
      </p>
      <form ref={form} onSubmit={onSubmit} className="mt-6 space-y-3" noValidate>
        {mode === "join" && (
          <Field name="name" label="Your name" autoComplete="name" error={fields.name} />
        )}
        <Field name="email" label="Email" type="email" autoComplete="email" error={fields.email} />
        <Field
          name="password"
          label="Password"
          type="password"
          autoComplete={mode === "in" ? "current-password" : "new-password"}
          error={fields.password}
          hint={mode === "join" ? "At least 8 characters." : undefined}
        />
        {error && !Object.keys(fields).length && (
          <p role="alert" className="text-alarm text-[14px]">
            {error}
          </p>
        )}
        <button className="btn btn-solid mt-2 w-full" disabled={busy}>
          {busy ? "One moment…" : mode === "in" ? "Sign in" : "Create account"}
        </button>
      </form>
      <div className="mt-5 flex flex-col gap-2 border-t border-[var(--line)] pt-5 text-[14px]">
        <button className="btn btn-line w-full" onClick={useDemo} disabled={busy}>
          Try the demo curator
        </button>
        <button className="text-center text-[var(--soft)] underline underline-offset-4 hover:text-[var(--ink)]" onClick={() => setMode(mode === "in" ? "join" : "in")}>
          {mode === "in" ? "New here? Create an account" : "Already have an account? Sign in"}
        </button>
      </div>
    </div>
  );
}

function Field({ name, label, error, hint, ...rest }: { name: string; label: string; error?: string; hint?: string } & React.InputHTMLAttributes<HTMLInputElement>) {
  const id = `f-${name}`;
  return (
    <label htmlFor={id} className="block">
      <span className="mb-1 block text-[14px] font-medium">{label}</span>
      <input id={id} name={name} className="field" aria-invalid={!!error} aria-describedby={error || hint ? `${id}-note` : undefined} {...rest} />
      {(error || hint) && (
        <span id={`${id}-note`} className={`mt-1 block text-[13px] ${error ? "text-alarm" : "text-[var(--soft)]"}`}>
          {error ?? hint}
        </span>
      )}
    </label>
  );
}
