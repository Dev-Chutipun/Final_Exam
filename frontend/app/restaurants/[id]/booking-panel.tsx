// แผงจองทุกสถานะตามอาร์ตบอร์ด 3: ยังไม่ login, เจ้าของร้าน, ร้านพักรับจอง, จองได้, จองสำเร็จ
import Link from "next/link";
import { Suspense } from "react";

import { BookingForm, type BookingRestaurant } from "@/components/booking/booking-form";
import { LoginLink } from "@/components/login-link";
import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";

export function BookingPanel({
  restaurant,
  loggedIn,
  isOwner,
  initialDate,
}: {
  restaurant: BookingRestaurant;
  loggedIn: boolean;
  isOwner: boolean;
  initialDate: string;
}) {
  if (isOwner) {
    return (
      <Panel title="ร้านของคุณ" badge={<Pill tone="peach">เจ้าของร้าน</Pill>}>
        <p className="text-(--ink-2)">
          เจ้าของร้านจองหรือรีวิวร้านของตัวเองไม่ได้ จัดการที่นั่ง เวลา และดูการจองของลูกค้าได้จากหน้าจัดการร้าน
        </p>
        <Button asChild size="lg">
          <Link href={`/my/restaurants/${restaurant.id}`}>จัดการร้าน</Link>
        </Button>
        <Button asChild variant="glass" size="lg">
          <Link href={`/my/restaurants/${restaurant.id}/bookings`}>ดูการจอง</Link>
        </Button>
      </Panel>
    );
  }

  if (!loggedIn) {
    return (
      <Panel title="จองโต๊ะ">
        <div className="flex flex-col gap-1.5 rounded-[22px] border border-white/95 bg-linear-135 from-(--st-peach-bg)/85 to-(--st-butter-bg)/85 p-5">
          <span className="font-heading text-[17px] font-semibold">ต้องเข้าสู่ระบบก่อน</span>
          <span className="text-[15px] text-(--ink-2)">เข้าสู่ระบบเพื่อจองโต๊ะหรือเขียนรีวิว เสร็จแล้วจะกลับมาที่หน้านี้</span>
        </div>
        <Suspense fallback={null}>
          <LoginLink variant="cta" size="confirm">
            เข้าสู่ระบบ
          </LoginLink>
        </Suspense>
      </Panel>
    );
  }

  return (
    <Panel
      title="จองโต๊ะ"
      badge={restaurant.accepting_bookings ? <Pill tone="mint">เปิดรับจอง</Pill> : <Pill tone="gray">พักรับจอง</Pill>}
    >
      <BookingForm restaurant={restaurant} initialDate={initialDate} />
    </Panel>
  );
}

function Panel({ title, badge, children }: { title: string; badge?: React.ReactNode; children: React.ReactNode }) {
  return (
    <aside aria-label={title} className="glass flex flex-col gap-4.5 rounded-(--r-panel) bg-white/68 p-6 sm:p-7 lg:sticky lg:top-6">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-[26px] font-semibold">{title}</h2>
        {badge}
      </div>
      {children}
    </aside>
  );
}
