import type { NextConfig } from "next";

const isExport = process.env.NODE_ENV === "production";

const nextConfig: NextConfig = {
  output: isExport ? "export" : undefined,
  ...(!isExport && {
    async rewrites() {
      return [
        {
          source: "/api/:path*",
          destination: `${process.env.API_BACKEND_URL || "http://localhost:8080"}/api/:path*`,
        },
      ];
    },
  }),
};

export default nextConfig;
