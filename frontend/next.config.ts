import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Raise the body size limit for the /api/upload rewrite proxy to match
  // the backend's 100 MB file size limit (Requirement 5.9).
  experimental: {
    proxyClientMaxBodySize: 110 * 1024 * 1024, // 110 MB (headroom above 100 MB limit)
  },
};

export default nextConfig;
