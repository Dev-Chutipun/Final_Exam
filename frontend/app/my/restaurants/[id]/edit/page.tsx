// แก้ข้อมูลร้าน (อาร์ตบอร์ด 6 โหมดแก้ไข): ชื่อ รายละเอียด หมวด
// ที่นั่ง เวลา นโยบาย และรูป แก้ที่หน้าจัดการร้าน เพราะต้องตรวจกับการจองเดิม
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { apiGet, apiGetOrNull, requireMe } from "@/lib/api-server";
import type { Category, RestaurantDetail } from "@/lib/types";

import { RestaurantForm } from "../../restaurant-form";

export const metadata: Metadata = { title: "แก้ไขข้อมูลร้าน" };

export default async function EditRestaurantPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  await requireMe(`/my/restaurants/${id}/edit`);
  const r = /^\d+$/.test(id) ? await apiGetOrNull<RestaurantDetail>(`/api/restaurants/${id}`) : null;
  // ไม่ใช่เจ้าของ → ทำเหมือนไม่มีหน้านี้ (UX เท่านั้น สิทธิ์จริงตรวจที่ Go)
  if (!r || !r.is_owner) notFound();
  const categories = await apiGet<Category[]>("/api/categories");

  return (
    <main className="mx-auto max-w-[800px] px-4 pt-9 pb-18 sm:px-6">
      <nav aria-label="เส้นทาง" className="flex gap-2 text-sm text-(--ink-2)">
        <Link href="/my/restaurants" className="hover:underline">
          ร้านของฉัน
        </Link>
        <span aria-hidden="true">/</span>
        <Link href={`/my/restaurants/${r.id}`} className="hover:underline">
          {r.name}
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-semibold text-(--ink)">แก้ไขข้อมูล</span>
      </nav>
      <h1 className="mt-3 text-[40px] font-semibold">แก้ไขข้อมูลร้าน</h1>
      <RestaurantForm mode="edit" categories={categories} restaurant={r} />
    </main>
  );
}
