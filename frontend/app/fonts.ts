// ฟอนต์แบบ local (ไฟล์อยู่ใน app/fonts พร้อมสัญญาอนุญาต OFL) เพื่อให้รันได้แม้ไม่มีอินเทอร์เน็ต
import localFont from "next/font/local";

/** หัวข้อและปุ่ม: Prompt 500/600/700 */
export const prompt = localFont({
  src: [
    { path: "./fonts/Prompt-Medium.ttf", weight: "500", style: "normal" },
    { path: "./fonts/Prompt-SemiBold.ttf", weight: "600", style: "normal" },
    { path: "./fonts/Prompt-Bold.ttf", weight: "700", style: "normal" },
  ],
  variable: "--font-prompt",
  display: "swap",
});

/** เนื้อหา: IBM Plex Sans Thai 400/500/600 */
export const plex = localFont({
  src: [
    { path: "./fonts/IBMPlexSansThai-Regular.ttf", weight: "400", style: "normal" },
    { path: "./fonts/IBMPlexSansThai-Medium.ttf", weight: "500", style: "normal" },
    { path: "./fonts/IBMPlexSansThai-SemiBold.ttf", weight: "600", style: "normal" },
  ],
  variable: "--font-plex",
  display: "swap",
});
