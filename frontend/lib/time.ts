// เวลาทั้งหมดของหน้าเว็บผ่านไฟล์นี้ (05 §2)
// - แสดงผลด้วย Intl ระบุ timeZone Asia/Bangkok เสมอ ผลจึงเหมือนกันทั้งฝั่งเซิร์ฟเวอร์และเบราว์เซอร์ (ไม่มี hydration warning)
// - ส่ง API เป็นสตริง +07:00 ที่ประกอบเอง ห้ามใช้ toISOString() เพราะแปลงตาม timezone ของเครื่องผู้ใช้

const TZ = "Asia/Bangkok";

const dateFmt = new Intl.DateTimeFormat("th-TH", {
  timeZone: TZ,
  weekday: "short",
  day: "numeric",
  month: "short",
  year: "numeric",
});
const timeFmt = new Intl.DateTimeFormat("th-TH", {
  timeZone: TZ,
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});
const partsFmt = new Intl.DateTimeFormat("en-CA", {
  timeZone: TZ,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});
const tileFmt = {
  weekday: new Intl.DateTimeFormat("th-TH", { timeZone: TZ, weekday: "long" }),
  day: new Intl.DateTimeFormat("th-TH", { timeZone: TZ, day: "numeric" }),
  monthYear: new Intl.DateTimeFormat("th-TH", { timeZone: TZ, month: "short", year: "numeric" }),
};

/** "อา. 11 ต.ค. 2569" (ปี พ.ศ. ตามค่าเริ่มต้นของ locale th-TH) */
export function formatDate(iso: string): string {
  return dateFmt.format(new Date(iso));
}

/** "12:00" */
export function formatTime(iso: string): string {
  return timeFmt.format(new Date(iso));
}

/** "12:00–13:00 น." */
export function formatTimeRange(startIso: string, endIso: string): string {
  return `${formatTime(startIso)}–${formatTime(endIso)} น.`;
}

/** "อา. 11 ต.ค. 2569 เวลา 11:30 น." */
export function formatDateTime(iso: string): string {
  return `${formatDate(iso)} เวลา ${formatTime(iso)} น.`;
}

/** ไทล์วันที่ในหน้าการจองของฉัน */
export function dateTile(iso: string) {
  const d = new Date(iso);
  return { weekday: tileFmt.weekday.format(d), day: tileFmt.day.format(d), monthYear: tileFmt.monthYear.format(d) };
}

/** วันที่ตามเวลา Bangkok เป็น "YYYY-MM-DD" (en-CA เรียงแบบนี้พอดี) */
export function localDate(iso: string | Date): string {
  return partsFmt.format(typeof iso === "string" ? new Date(iso) : iso);
}

/** วันนี้ตามเวลา Bangkok "YYYY-MM-DD" */
export function todayInBangkok(): string {
  return localDate(new Date());
}

/** บวกวันให้สตริง "YYYY-MM-DD" (คำนวณแบบ UTC ล้วน จึงไม่ขึ้นกับ timezone ของเครื่อง) */
export function addDays(date: string, days: number): string {
  const [y, m, d] = date.split("-").map(Number);
  const t = new Date(Date.UTC(y, m - 1, d + days));
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())}`;
}

/** ประกอบเวลาส่ง API: toApiTime("2026-10-11", "12:00") → "2026-10-11T12:00:00+07:00" */
export function toApiTime(date: string, hhmm: string): string {
  return `${date}T${hhmm}:00+07:00`;
}

/** แปลงเวลา ISO เป็นคู่ (วันที่, "HH:MM") ตามเวลา Bangkok สำหรับประกอบกลับด้วย toApiTime */
export function splitApiTime(iso: string): { date: string; hhmm: string } {
  return { date: localDate(iso), hhmm: formatTime(iso) };
}

/** "120 นาที" → "2 ชั่วโมง", "90" → "1 ชั่วโมง 30 นาที" */
export function formatMinutes(min: number): string {
  const h = Math.floor(min / 60);
  const m = min % 60;
  if (h === 0) return `${m} นาที`;
  return m === 0 ? `${h} ชั่วโมง` : `${h} ชั่วโมง ${m} นาที`;
}
