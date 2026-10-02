// สร้างร้านใหม่ (อาร์ตบอร์ด 6) — หน้านี้แค่ตรวจ login และโหลดหมวด ตัวฟอร์มเป็น Client Component
import type { Metadata } from "next";
import Link from "next/link";

import { apiGet, requireMe } from "@/lib/api-server";
import type { Category } from "@/lib/types";

import { RestaurantForm } from "../restaurant-form";

export const metadata: Metadata = { title: "สร้างร้านใหม่" };

export default async function NewRestaurantPage() {
  await requireMe("/my/restaurants/new");
  const categories = await apiGet<Category[]>("/api/categories");

  return (
    <main className="mx-auto max-w-[800px] px-4 pt-9 pb-18 sm:px-6">
      <nav aria-label="เส้นทาง" className="flex gap-2 text-sm text-(--ink-2)">
        <Link href="/my/restaurants" className="hover:underline">
          ร้านของฉัน
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-semibold text-(--ink)">สร้างร้านใหม่</span>
      </nav>
      <h1 className="mt-3 text-[40px] font-semibold">สร้างร้านใหม่</h1>
      <p className="mt-1.5 text-(--ink-2)">กรอก 5 ส่วนด้านล่าง ระบบตรวจซ้ำทุกช่องที่เซิร์ฟเวอร์ก่อนบันทึก</p>
      <RestaurantForm mode="create" categories={categories} />
    </main>
  );
}
