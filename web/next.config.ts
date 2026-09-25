import type { NextConfig } from "next";

// The Go server. REST calls, pictures and posters go through Next so the
// browser sees one origin; the live rooms' WebSocket connects directly.
// In production a reverse proxy (see Caddyfile) does the same routing.
const api = process.env.VERNISSAGE_API ?? "http://127.0.0.1:8790";

const nextConfig: NextConfig = {
  // The Docker image runs Next's self-contained server.
  output: process.env.NEXT_OUTPUT === "standalone" ? "standalone" : undefined,
  devIndicators: false,
  agentRules: false,
  async rewrites() {
    return ["api", "img", "dzi", "poster"].map((prefix) => ({
      source: `/${prefix}/:path*`,
      destination: `${api}/${prefix}/:path*`,
    }));
  },
};

export default nextConfig;
