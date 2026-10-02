"use client";

// [B] ช่องค้นหาชื่อร้าน: หน่วง 300 ms ก่อนเปลี่ยน URL (05 §3) สถานะอยู่ใน URL จึงแชร์และย้อนกลับได้
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { SearchIcon } from "@/components/icons";

export function SearchBox({ initial }: { initial: string }) {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [q, setQ] = useState(initial);
  const [lastInitial, setLastInitial] = useState(initial);
  const first = useRef(true);

  // URL เปลี่ยนจากที่อื่น (เช่นกด "ล้างการค้นหา" หรือย้อนกลับ) → ซิงก์ช่องค้นหา
  // ระหว่างพิมพ์ initial จะเท่ากับข้อความที่พิมพ์อยู่แล้ว จึงไม่ทับสิ่งที่กำลังพิมพ์
  if (initial !== lastInitial) {
    setLastInitial(initial);
    if (initial !== q.trim()) setQ(initial);
  }

  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    const t = setTimeout(() => {
      const next = new URLSearchParams(params);
      if (q.trim()) next.set("q", q.trim());
      else next.delete("q");
      next.delete("page");
      router.replace(`${pathname}?${next}`);
    }, 300);
    return () => clearTimeout(t);
    // ทำงานเมื่อข้อความเปลี่ยนเท่านั้น
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q]);

  return (
    <div className="relative grow">
      <span className="absolute top-[15px] left-[15px]">
        <SearchIcon />
      </span>
      <input
        type="search"
        aria-label="ค้นหาร้านจากชื่อ"
        placeholder="ค้นหาจากชื่อร้าน"
        maxLength={100}
        value={q}
        onChange={(e) => setQ(e.target.value)}
        className="field border-white/95 pl-11"
      />
    </div>
  );
}
