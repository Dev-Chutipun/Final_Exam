// หน้าตัวอย่างชั่วคราวสำหรับตรวจ token §5 และ variant ปุ่ม §6 (ลบตอนเก็บงานในขั้น 7)
import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";

export const metadata = { title: "Styleguide" };

const variants = ["default", "cta", "glass", "danger", "danger-strong"] as const;
const tones = [
  ["mint", "ยืนยันแล้ว · เปิดรับจอง"],
  ["butter", "เหลือน้อย · หมดเวลาแก้ไข"],
  ["peach", "เจ้าของร้าน · รูปปก"],
  ["lav", "หมวดหมู่"],
  ["pink", "ผิดพลาด"],
  ["gray", "พักรับจอง · ยกเลิกแล้ว"],
] as const;
const covers = ["peach", "mint", "butter", "latte", "lilac", "sky"];

export default function Styleguide() {
  return (
    <main className="mx-auto flex max-w-[1000px] flex-col gap-8 px-6 py-12">
      <h1 className="text-4xl font-semibold">
        Styleguide <span className="text-(--accent-strong)">พาสเทลกระจก</span>
      </h1>

      <section className="glass flex flex-col gap-4 rounded-(--r-card) p-7">
        <h2 className="text-xl font-semibold">ปุ่ม (§6)</h2>
        <div className="flex flex-wrap gap-3">
          {variants.map((v) => (
            <Button key={v} variant={v}>
              {v}
            </Button>
          ))}
        </div>
        <div className="flex flex-wrap gap-3">
          {variants.map((v) => (
            <Button key={v} variant={v} disabled>
              {v} (disabled)
            </Button>
          ))}
        </div>
        <div className="flex flex-wrap gap-3">
          <Button variant="cta" size="confirm">
            ยืนยันการจอง (58px)
          </Button>
          <Button size="sm" variant="glass">
            ปุ่มเล็กในเมนู
          </Button>
        </div>
      </section>

      <section className="glass flex flex-col gap-4 rounded-(--r-card) p-7">
        <h2 className="text-xl font-semibold">ป้ายสถานะ (§5)</h2>
        <div className="flex flex-wrap gap-2">
          {tones.map(([tone, label]) => (
            <Pill key={tone} tone={tone}>
              {label}
            </Pill>
          ))}
        </div>
      </section>

      <section className="glass flex flex-col gap-4 rounded-(--r-card) p-7">
        <h2 className="text-xl font-semibold">ช่องกรอกและตัวอักษร</h2>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          ช่องปกติ
          <input className="field" defaultValue="ครัวริมคลอง" />
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          ช่องที่กรอกผิด
          <input className="field" aria-invalid="true" defaultValue="0" />
          <span className="text-[13px] font-medium text-(--st-pink-ink)">ต้องเป็นจำนวนเต็มตั้งแต่ 1</span>
        </label>
        <p className="text-(--ink-body)">ข้อความย่อหน้ายาว (--ink-body) ใช้กับรายละเอียดร้านและรีวิว</p>
        <p className="text-(--ink-2)">ตัวอักษรรอง (--ink-2)</p>
      </section>

      <section className="glass flex flex-col gap-4 rounded-(--r-card) p-7">
        <h2 className="text-xl font-semibold">ภาพปก placeholder</h2>
        <div className="grid grid-cols-3 gap-3">
          {covers.map((c) => (
            <div key={c} className="relative h-24 rounded-[20px]" style={{ background: `var(--cover-${c})` }}>
              <span className="on-image absolute bottom-2 left-2 rounded-full px-2 text-xs font-medium">{c}</span>
            </div>
          ))}
        </div>
      </section>
    </main>
  );
}
