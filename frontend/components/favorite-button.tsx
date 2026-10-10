"use client";

// ปุ่มหัวใจ "ร้านที่ชอบ" ใช้ทั้งบนการ์ดในหน้ารายการและในหน้าร้าน
// กดแล้วสลับทันทีเพื่อให้รู้สึกตอบสนองไว แล้วค่อยยืนยันกับเซิร์ฟเวอร์ · ล้มเหลวเมื่อไรก็สลับกลับ
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { apiSend } from "@/lib/api-client";
import { problemMessage } from "@/lib/problem";

export function FavoriteButton({
  restaurantId,
  initial,
  refreshOnDone,
  className = "",
}: {
  restaurantId: number;
  initial: boolean;
  /** สั่ง router.refresh() หลังบันทึก สำหรับหน้าที่รายการต้องเปลี่ยนตาม เช่นหน้าร้านที่ชอบ */
  refreshOnDone?: boolean;
  className?: string;
}) {
  const router = useRouter();
  const [on, setOn] = useState(initial);
  const [busy, setBusy] = useState(false);

  async function toggle(e: React.MouseEvent) {
    // การ์ดทั้งใบเป็นลิงก์ไปหน้าร้าน กดหัวใจจึงต้องไม่พาไปไหน
    e.preventDefault();
    e.stopPropagation();
    if (busy) return;
    const next = !on;
    setOn(next);
    setBusy(true);
    try {
      await apiSend(next ? "PUT" : "DELETE", `/api/restaurants/${restaurantId}/favorite`);
      if (refreshOnDone) router.refresh();
    } catch (err) {
      setOn(!next);
      toast.error(problemMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <button
      type="button"
      onClick={toggle}
      aria-pressed={on}
      aria-label={on ? "เอาออกจากร้านที่ชอบ" : "เพิ่มในร้านที่ชอบ"}
      className={`flex size-10 items-center justify-center rounded-full border border-white/95 bg-white/85 shadow-[0_4px_12px_rgba(107,91,214,0.12)] backdrop-blur-[10px] transition hover:bg-white disabled:opacity-60 ${className}`}
      disabled={busy}
    >
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill={on ? "var(--st-pink-ink)" : "none"}
        stroke={on ? "var(--st-pink-ink)" : "var(--ink-2)"}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M12 20.5 4.2 13a4.8 4.8 0 0 1 6.8-6.8l1 1 1-1A4.8 4.8 0 0 1 19.8 13Z" />
      </svg>
    </button>
  );
}
