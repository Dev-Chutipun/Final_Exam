"use client";

// เปลี่ยนการเรียงแล้วเก็บใน URL (แชร์ลิงก์และกดย้อนกลับได้) · กลับไปหน้า 1 ทุกครั้ง
import { usePathname, useRouter, useSearchParams } from "next/navigation";

export function SortSelect({ value }: { value: string }) {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();

  return (
    <select
      aria-label="เรียงลำดับ"
      className="field w-full border-white/95 sm:w-[220px]"
      value={value}
      onChange={(e) => {
        const next = new URLSearchParams(params);
        next.set("sort", e.target.value);
        next.delete("page");
        router.push(`${pathname}?${next}`);
      }}
    >
      <option value="highest_rated">เรียงตามคะแนนสูงสุด</option>
      <option value="most_reviewed">เรียงตามรีวิวมากที่สุด</option>
    </select>
  );
}
