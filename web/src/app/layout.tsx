import type { Metadata, Viewport } from "next";
import { Archivo, Bodoni_Moda } from "next/font/google";
import { AuthDialog } from "@/components/AuthDialog";
import { Providers } from "@/components/Providers";
import { SelectionTray } from "@/components/SelectionTray";
import "./globals.css";

const bodoni = Bodoni_Moda({
  subsets: ["latin"],
  style: ["normal", "italic"],
  axes: ["opsz"],
  variable: "--font-bodoni",
  display: "swap",
});

const archivo = Archivo({
  subsets: ["latin"],
  style: ["normal", "italic"],
  axes: ["wdth"],
  variable: "--font-archivo",
  display: "swap",
});

export const metadata: Metadata = {
  metadataBase: new URL(process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3790"),
  title: { default: "Vernissage", template: "%s | Vernissage" },
  description:
    "Hang your own exhibition of public-domain masterpieces from The Met and the Cleveland Museum of Art, then walk through it with friends.",
};

export const viewport: Viewport = {
  themeColor: "#5b1d1f",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${bodoni.variable} ${archivo.variable}`}>
      <body>
        <Providers>
          {children}
          <SelectionTray />
          <AuthDialog />
        </Providers>
      </body>
    </html>
  );
}
