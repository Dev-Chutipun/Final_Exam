import type { NextConfig } from "next";

// API_ORIGIN อ่านจาก frontend/.env.local (ใช้ฝั่งเซิร์ฟเวอร์เท่านั้น)
const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  // ส่งทุกคำขอ /api/* ต่อให้ Go โดยไม่มี proxy.ts (ดู docs/05-frontend.md §1)
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiOrigin}/api/:path*` }];
  },
  // กัน clickjacking (หน้าเว็บถูกฝังใน iframe ของเว็บอื่นแล้วหลอกให้กดปุ่มยืนยัน) และกันเบราว์เซอร์เดาชนิดไฟล์
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Content-Security-Policy", value: "frame-ancestors 'none'" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
        ],
      },
    ];
  },
};

export default nextConfig;
