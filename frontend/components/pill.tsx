// ป้ายสถานะตาม token ใน 05 §5 (สีอยู่ใน app/globals.css)
import type { ReactNode } from "react";

const tones = {
  mint: "bg-(--st-mint-bg) text-(--st-mint-ink)", // ยืนยันแล้ว, เปิดรับจอง
  butter: "bg-(--st-butter-bg) text-(--st-butter-ink)", // เหลือน้อย, คำเตือน, หมดเวลาแก้ไข
  peach: "bg-(--st-peach-bg) text-(--st-peach-ink)", // เจ้าของร้าน, รูปปก
  lav: "bg-(--st-lav-bg) text-(--st-lav-ink)", // หมวดหมู่, ข้อมูล
  pink: "bg-(--st-pink-bg) text-(--st-pink-ink) border border-(--st-pink-border)", // ผิดพลาด, ลบ
  gray: "bg-(--st-gray-bg) text-(--st-gray-ink)", // พักรับจอง, ยกเลิกแล้ว
  glass: "on-image", // ป้ายบนภาพ
} as const;

export type Tone = keyof typeof tones;

export function Pill({ tone, children, className = "" }: { tone: Tone; children: ReactNode; className?: string }) {
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-[11px] py-[3px] text-[13px] font-semibold ${tones[tone]} ${className}`}>
      {children}
    </span>
  );
}
