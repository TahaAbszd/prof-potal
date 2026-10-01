import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${process.env.API_PROXY_TARGET || "http://127.0.0.1:8080"}/api/:path*` }];
  },
};

export default nextConfig;
