import type { NextConfig } from "next";

// API_ORIGIN อ่านจาก frontend/.env.local (ใช้ฝั่งเซิร์ฟเวอร์เท่านั้น)
const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  // ส่งทุกคำขอ /api/* ต่อให้ Go โดยไม่มี proxy.ts (ดู docs/05-frontend.md §1)
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiOrigin}/api/:path*` }];
  },
};

export default nextConfig;
