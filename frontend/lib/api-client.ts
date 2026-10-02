// Client Component ส่งข้อมูลผ่าน /api/* (same-origin ผ่าน rewrite ของ Next.js) แล้วให้หน้า router.refresh() เอง (05 §2)
import { toApiError } from "@/lib/problem";

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

/**
 * ส่งคำขอไป API
 * - body เป็น FormData → ห้ามตั้ง Content-Type เอง ให้เบราว์เซอร์ใส่ boundary ของ multipart
 * - body อื่น → ส่งเป็น JSON (Go บังคับ Content-Type: application/json)
 * - 401 → พาไป login แล้วกลับมาหน้าเดิม
 * - ไม่สำเร็จ → throw ApiError (ใช้ problemMessage แสดงข้อความ)
 */
export async function apiSend<T = unknown>(method: Method, path: string, body?: unknown): Promise<T | null> {
  const init: RequestInit = { method };
  if (body instanceof FormData) {
    init.body = body;
  } else if (body !== undefined) {
    init.headers = { "Content-Type": "application/json" };
    init.body = JSON.stringify(body);
  }

  const res = await fetch(path, init);
  if (res.status === 401) {
    const here = window.location.pathname + window.location.search;
    // ต้องเปลี่ยนหน้าเต็ม (ไม่ใช่ router.push) เพราะ /api/auth/login คือ Go ที่ redirect ต่อไป Keycloak
    // eslint-disable-next-line @next/next/no-location-assign-relative-destination
    window.location.href = "/api/auth/login?return_to=" + encodeURIComponent(here);
    throw await toApiError(res);
  }
  if (!res.ok) throw await toApiError(res);
  if (res.status === 204) return null;
  return res.json() as Promise<T>;
}
