import type { Metadata } from "next";
import { Suspense } from "react";
import { CollectionView } from "./CollectionView";

export const metadata: Metadata = {
  title: "Collection",
  description: "Paintings, prints and drawings from The Met and the Cleveland Museum of Art, searchable by artist, subject, date and colour.",
};

export default function CollectionPage() {
  return (
    <Suspense>
      <CollectionView />
    </Suspense>
  );
}
