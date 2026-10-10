// หน้ารายการร้าน (อาร์ตบอร์ด 1) — Server Component อ่านข้อมูลจาก Go
// สถานะการเรียงและหน้าอยู่ใน URL (searchParams)
import Link from "next/link";

import { Suspense } from "react";

import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiGet } from "@/lib/api-server";
import type { Category, Page, RestaurantListItem } from "@/lib/types";

import { RestaurantCard } from "./restaurant-card";
import { SearchBox } from "./search-box";
import { SortSelect } from "./sort-select";

type Search = { sort?: string; page?: string; q?: string; category?: string | string[] };

const PAGE_SIZE = 12;

export default async function Home({ searchParams }: { searchParams: Promise<Search> }) {
  const sp = await searchParams;
  const sort = sp.sort === "most_reviewed" ? "most_reviewed" : "highest_rated";
  const page = Math.max(1, Number.parseInt(sp.page ?? "1", 10) || 1);
  const q = (sp.q ?? "").trim().slice(0, 100);
  const selected = ([] as string[]).concat(sp.category ?? []);

  const categories = await apiGet<Category[]>("/api/categories");
  const known = selected.filter((s) => categories.some((c) => c.slug === s)); // slug ที่ไม่มีจริงตัดทิ้ง

  // สร้าง query จากสถานะใน URL (q, category ซ้ำได้ = OR)
  const build = (over: { page?: number; categories?: string[] } = {}) => {
    const u = new URLSearchParams({ sort });
    if (q) u.set("q", q);
    for (const c of over.categories ?? known) u.append("category", c);
    const p = over.page ?? 1;
    if (p > 1) u.set("page", String(p));
    return u;
  };
  const api = build({ page });
  api.set("page", String(page));
  api.set("page_size", String(PAGE_SIZE));
  const data = await apiGet<Page<RestaurantListItem>>(`/api/restaurants?${api}`);
  const totalPages = Math.max(1, Math.ceil(data.total / PAGE_SIZE));

  const pageHref = (p: number) => `/?${build({ page: p })}`;
  const toggleHref = (slug: string) =>
    `/?${build({ categories: known.includes(slug) ? known.filter((s) => s !== slug) : [...known, slug] })}`;
  const filtered = q !== "" || known.length > 0;

  return (
    <main className="mx-auto max-w-(--w-wide) px-4 pt-12 pb-18 sm:px-6">
      <section className="flex flex-col items-center gap-3.5 text-center">
        <Pill tone="peach" className="px-3.5 py-1.5">
          จองล่วงหน้าได้สูงสุด 30 วัน
        </Pill>
        <h1 className="text-4xl leading-tight font-semibold sm:text-[54px]">
          อยากกินไหน นึกถึง <span className="text-(--accent-strong)">กินนี่</span>
        </h1>
        <p className="max-w-(--w-read) text-lg text-(--ink-2)">ที่ที่ทุกคนจะได้กินที่ที่อยากกิน</p>
      </section>

      <div className="glass mt-8 flex flex-col gap-2.5 rounded-3xl p-2.5 sm:flex-row sm:items-center">
        <Suspense fallback={null}>
          <SearchBox initial={q} />
          <SortSelect value={sort} />
        </Suspense>
      </div>

      {/* [B] กรองตามหมวด เลือกได้หลายหมวด (OR) */}
      <nav aria-label="หมวดหมู่" className="mt-5 flex flex-wrap gap-2.5">
        <Chip href={`/?${build({ categories: [] })}`} on={known.length === 0}>
          ทั้งหมด
        </Chip>
        {categories.map((c) => (
          <Chip key={c.id} href={toggleHref(c.slug)} on={known.includes(c.slug)}>
            {c.name_th}
          </Chip>
        ))}
      </nav>

      {data.items.length === 0 ? (
        <section className="glass mx-auto mt-11 flex max-w-xl flex-col items-center gap-3 rounded-(--r-card) p-10 text-center">
          {filtered ? (
            <>
              <h2 className="text-xl font-semibold">ไม่พบร้านที่ตรงกับการค้นหา</h2>
              <p className="text-(--ink-2)">ลองใช้คำค้นอื่น หรือเลือกหมวดอื่น</p>
              <Button asChild variant="glass">
                <Link href={`/?sort=${sort}`}>ล้างการค้นหา</Link>
              </Button>
            </>
          ) : (
            <>
              <h2 className="text-xl font-semibold">ยังไม่มีร้านในระบบ</h2>
              <p className="text-(--ink-2)">เป็นคนแรกที่สร้างร้านของคุณได้เลย</p>
              <Button asChild variant="cta">
                <Link href="/my/restaurants/new">สร้างร้าน</Link>
              </Button>
            </>
          )}
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

function Chip({ href, on, children }: { href: string; on: boolean; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      aria-current={on ? "true" : undefined}
      className={`flex min-h-[42px] items-center rounded-full border px-4.5 ${
        on
          ? "border-(--btn-primary-to) bg-linear-to-b from-(--btn-primary-from) to-(--btn-primary-to) font-semibold shadow-[inset_0_1px_0_#fff,0_6px_16px_rgba(107,91,214,0.2)]"
          : "border-white/95 bg-white/62 font-medium shadow-[inset_0_1px_0_#fff,0_4px_12px_rgba(107,91,214,0.06)] backdrop-blur-[10px]"
      }`}
    >
      {children}
    </Link>
  );
}
