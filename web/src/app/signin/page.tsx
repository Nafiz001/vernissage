"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { AuthForm } from "@/components/AuthDialog";
import { SiteHeader } from "@/components/SiteHeader";

function SignIn() {
  const router = useRouter();
  const next = useSearchParams().get("next") ?? "/studio";
  return (
    <div className="room room-light plaster mx-auto mt-10 w-full max-w-[420px] rounded-2xl p-7 shadow-2xl [--wall:var(--color-chalk)]">
      <AuthForm onDone={() => router.push(next.startsWith("/") ? next : "/studio")} />
    </div>
  );
}

export default function SignInPage() {
  return (
    <main className="room room-dark plaster min-h-svh px-5 pb-20 [--wall:var(--color-oxblood)]">
      <SiteHeader />
      <Suspense>
        <SignIn />
      </Suspense>
    </main>
  );
}
