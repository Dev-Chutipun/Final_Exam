// แปลง application/problem+json ของ Go เป็น error ที่หน้าเว็บใช้ได้ (00-overview.md §5)

export type FieldError = { field: string; message: string };

export type Problem = {
  type: string; // เช่น "/problems/seat-capacity-exceeded"
  title: string;
  status: number;
  detail?: string;
  errors?: FieldError[];
  [extra: string]: unknown; // available, full_from, conflicts, upcoming_count, ...
};

export class ApiError extends Error {
  constructor(public problem: Problem) {
    super(problem.detail ?? problem.title);
  }
  get type() {
    return this.problem.type.replace("/problems/", "");
  }
  get status() {
    return this.problem.status;
  }
  /** error ของแต่ละช่อง (จาก errors[].field) สำหรับแสดงใต้ช่องกรอก */
  get fieldErrors(): Record<string, string> {
    const out: Record<string, string> = {};
    for (const e of this.problem.errors ?? []) out[e.field] ??= e.message;
    return out;
  }
}

/** ข้อความไทยสำรองกรณีเซิร์ฟเวอร์ไม่ส่ง detail */
const fallback: Record<string, string> = {
  validation: "ข้อมูลไม่ถูกต้อง",
  unauthenticated: "ต้องเข้าสู่ระบบก่อน",
  "own-restaurant-booking": "จองร้านของตัวเองไม่ได้",
  "own-restaurant-review": "รีวิวร้านของตัวเองไม่ได้",
  "not-restaurant-owner": "จัดการได้เฉพาะร้านของตัวเอง",
  "cross-origin-request": "คำขอถูกปฏิเสธ",
  "not-found": "ไม่พบข้อมูล",
  "image-too-large": "ไฟล์ใหญ่เกินกำหนด",
  "unsupported-image-type": "รองรับเฉพาะรูป JPEG, PNG หรือ WebP",
  "unsupported-media-type": "รูปแบบข้อมูลไม่ถูกต้อง",
  "booking-in-past": "เวลาที่เลือกผ่านไปแล้ว",
  "lead-time-not-met": "จองล่วงหน้าไม่พอ",
  "too-far-in-advance": "จองล่วงหน้าได้ไม่เกิน 30 วัน",
  "outside-opening-hours": "อยู่นอกเวลาเปิดร้าน",
  "party-exceeds-capacity": "จำนวนคนเกินที่นั่งทั้งร้าน",
  "cancel-deadline-passed": "เลยเวลาที่แก้ไขหรือยกเลิกได้",
  "invalid-restaurant-policy": "นโยบายร้านไม่ถูกต้อง",
  "overlapping-own-booking": "คุณมีการจองช่วงเวลานี้อยู่แล้ว",
  "seat-capacity-exceeded": "ที่นั่งไม่พอในช่วงที่เลือก",
  "booking-not-active": "การจองนี้ถูกยกเลิกแล้ว",
  "already-cancelled": "การจองนี้ถูกยกเลิกไปแล้ว",
  "already-reviewed": "คุณรีวิวร้านนี้แล้ว",
  "settings-conflict-with-bookings": "ค่าใหม่ชนกับการจองที่มีอยู่",
  "restaurant-not-accepting-bookings": "ร้านพักรับจองชั่วคราว",
  "restaurant-name-taken": "คุณมีร้านชื่อนี้อยู่แล้ว",
  "restaurant-has-upcoming-bookings": "ร้านยังมีการจองที่จะมาถึง",
  "image-limit-reached": "ร้านมีรูปครบ 3 รูปแล้ว",
  "last-image": "ร้านต้องมีรูปอย่างน้อย 1 รูป",
  "internal-error": "เกิดข้อผิดพลาดในระบบ ลองใหม่อีกครั้ง",
};

/** ข้อความที่ควรแสดงให้ผู้ใช้เห็น: detail ของเซิร์ฟเวอร์ก่อน แล้วค่อยใช้ข้อความสำรอง */
export function problemMessage(err: unknown): string {
  if (err instanceof ApiError) {
    return err.problem.detail ?? fallback[err.type] ?? err.problem.title;
  }
  return "เชื่อมต่อเซิร์ฟเวอร์ไม่ได้ ลองใหม่อีกครั้ง";
}

/** อ่าน body ของคำตอบที่ไม่สำเร็จเป็น ApiError */
export async function toApiError(res: Response): Promise<ApiError> {
  let p: Problem;
  try {
    p = await res.json();
  } catch {
    p = { type: "/problems/internal-error", title: res.statusText, status: res.status };
  }
  if (!p || typeof p.type !== "string") {
    p = { type: "/problems/internal-error", title: res.statusText, status: res.status };
  }
  return new ApiError(p);
}
