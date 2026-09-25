// Shapes of the Go API's JSON.

export type Kind = "painting" | "print" | "drawing";
export type FrameStyle = "gilt" | "oak" | "black" | "silk";

export interface Hang {
  /** Outside of the frame, metres. */
  w: number;
  h: number;
  /** The picture itself, metres. */
  artW: number;
  artH: number;
  frame: FrameStyle;
  border: number;
  mat: number;
  estimated: boolean;
}

export interface Artwork {
  id: number;
  title: string;
  artist: string;
  artistBio: string;
  date: string;
  year?: number;
  medium: string;
  kind: Kind;
  culture?: string;
  department?: string;
  description?: string;
  credit?: string;
  highlight: boolean;
  museum: { key: "cma" | "met"; name: string; url: string };
  image: { aspect: number; blurhash?: string; dominant?: string; ready: boolean };
  palette: { hex: string; w: number }[];
  size?: { wCm: number; hCm: number };
  hang: Hang;
}

export interface ArtworkPage {
  items: Artwork[];
  total: number;
  offset: number;
  next: number | null;
  fuzzy: boolean;
}

export interface ArtworkDetail {
  artwork: Artwork;
  moreByArtist: Artwork[];
  exhibitions: Exhibition[];
  saved: boolean;
}

export interface Owner {
  handle: string;
  name: string;
  hue: number;
}

export interface Preview {
  id: number;
  title: string;
  aspect: number;
  blurhash?: string;
  dominant?: string;
}

export interface Exhibition {
  id: number;
  slug: string;
  title: string;
  statement: string;
  room: string;
  paint: string;
  paintHex: string;
  floor: string;
  light: string;
  status: "draft" | "published";
  coverId: number | null;
  openingAt: string | null;
  publishedAt: string | null;
  posterVersion: number;
  visits: number;
  applause: number;
  createdAt: string;
  updatedAt: string;
  owner: Owner;
  workCount: number;
  preview: Preview[];
  inside: number;
  mine: boolean;
}

export interface Placement {
  artworkId: number;
  wall: number;
  x: number;
  y: number;
  label: string;
}

export interface Problem {
  artworkId: number;
  message: string;
}

export interface ExhibitionDetail {
  exhibition: Exhibition;
  placements: Placement[];
  works: Artwork[];
  problems: Problem[] | null;
  applauded: boolean;
}

export interface Room {
  key: string;
  name: string;
  blurb: string;
  width: number;
  depth: number;
  height: number;
  skylight: boolean;
  door: { width: number; height: number };
  benches: { x: number; z: number; width: number; depth: number }[];
}

export interface Paint {
  key: string;
  name: string;
  hex: string;
}

export interface Rooms {
  rooms: Room[];
  paints: Paint[];
  floors: { key: string; name: string }[];
  lights: { key: string; name: string }[];
  rules: {
    eyeLine: number;
    corner: number;
    minGap: number;
    floorClear: number;
    ceilingClear: number;
    maxWorks: number;
  };
}

export interface User {
  handle: string;
  name: string;
  hue: number;
  email?: string;
}

export interface Stats {
  collection: {
    works: number;
    analyzed: number;
    artists: number;
    paintings: number;
    prints: number;
    drawings: number;
    earliest: number;
    latest: number;
    exhibitions: number;
  };
  indexed: number;
  live: { rooms: number; people: number };
}

export interface GuestbookEntry {
  id: number;
  name: string;
  hue: number;
  message: string;
  createdAt: string;
}
