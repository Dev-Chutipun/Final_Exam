"use client";

// เลือกวันทำการ แล้วเก็บใน URL (?date=YYYY-MM-DD)
import { usePathname, useRouter } from "next/navigation";

export function DatePicker({ value }: { value: string }) {
  const router = useRouter();
  const pathname = usePathname();
  return (
    <label className="flex items-center gap-2 text-sm text-(--ink-2)">
      วันทำการ
      <input
        type="date"
        className="field w-[190px]"
        value={value}
        onChange={(e) => e.target.value && router.push(`${pathname}?date=${e.target.value}`)}
      />
    </label>
  );
}
