// Next.js 16 ไม่มีคำสั่ง next lint แล้ว ใช้ ESLint ตรงด้วย config ของ Next
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const config = [
  ...nextVitals,
  ...nextTs,
  { ignores: [".next/**", "node_modules/**", "next-env.d.ts"] },
];

export default config;
