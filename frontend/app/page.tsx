// หน้ารายการร้าน (อาร์ตบอร์ด 1) — Server Component อ่านข้อมูลจาก Go
// สถานะการเรียงและหน้าอยู่ใน URL (searchParams)
import Link from "next/link";

import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiGet } from "@/lib/api-server";
import type { Page, RestaurantListItem } from "@/lib/types";

import { RestaurantCard } from "./restaurant-card";
import { SortSelect } from "./sort-select";

type Search = { sort?: string; page?: string };

const PAGE_SIZE = 12;

export default async function Home({ searchParams }: { searchParams: Promise<Search> }) {
  const sp = await searchParams;
  const sort = sp.sort === "most_reviewed" ? "most_reviewed" : "highest_rated";
  const page = Math.max(1, Number.parseInt(sp.page ?? "1", 10) || 1);

  const query = new URLSearchParams({ sort, page: String(page), page_size: String(PAGE_SIZE) });
  const data = await apiGet<Page<RestaurantListItem>>(`/api/restaurants?${query}`);
  const totalPages = Math.max(1, Math.ceil(data.total / PAGE_SIZE));

  const pageHref = (p: number) => {
    const q = new URLSearchParams({ sort });
    if (p > 1) q.set("page", String(p));
    return `/?${q}`;
  };

  return (
    <main className="mx-auto max-w-[1232px] px-4 pt-12 pb-18 sm:px-6">
      <section className="flex flex-col items-center gap-3.5 text-center">
        <Pill tone="peach" className="px-3.5 py-1.5">
          จองล่วงหน้าได้สูงสุด 30 วัน
        </Pill>
        <h1 className="text-4xl leading-tight font-semibold sm:text-[54px]">
          วันนี้อยากกิน<span className="text-(--accent-strong)">อะไรดี?</span>
        </h1>
        <p className="max-w-[620px] text-lg text-(--ink-2)">เลือกร้านที่ใช่ ดูที่ว่างรายช่วงเวลา แล้วจองโต๊ะได้ในไม่กี่ขั้นตอน</p>
      </section>

      <div className="glass mx-auto mt-8 flex max-w-[880px] flex-col gap-2.5 rounded-3xl p-2.5 sm:flex-row sm:items-center sm:justify-end">
        <SortSelect value={sort} />
      </div>

      {data.items.length === 0 ? (
        <section className="glass mx-auto mt-11 flex max-w-xl flex-col items-center gap-3 rounded-(--r-card) p-10 text-center">
          <h2 className="text-xl font-semibold">ยังไม่มีร้านในระบบ</h2>
          <p className="text-(--ink-2)">เป็นคนแรกที่สร้างร้านของคุณได้เลย</p>
          <Button asChild variant="cta">
            <Link href="/my/restaurants/new">สร้างร้าน</Link>
          </Button>
        </section>
      ) : (
        <div className="mt-11 grid gap-7 sm:grid-cols-2 lg:grid-cols-3">
          {data.items.map((r) => (
            <RestaurantCard key={r.id} r={r} />
          ))}
        </div>
      )}

      {totalPages > 1 && (
        <nav aria-label="การแบ่งหน้า" className="mt-12 flex items-center justify-center gap-4">
          {page > 1 ? (
            <Button asChild variant="glass">
              <Link href={pageHref(page - 1)}>ก่อนหน้า</Link>
            </Button>
          ) : (
            <Button variant="glass" disabled>
              ก่อนหน้า
            </Button>
          )}
          <span className="glass font-heading rounded-2xl px-4.5 py-2.5 font-medium">
            หน้า {page} จาก {totalPages}
          </span>
          {page < totalPages ? (
            <Button asChild variant="glass">
              <Link href={pageHref(page + 1)}>ถัดไป</Link>
            </Button>
          ) : (
            <Button variant="glass" disabled>
              ถัดไป
            </Button>
          )}
        </nav>
      )}
    </main>
  );
}
