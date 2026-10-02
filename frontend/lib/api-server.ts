// Server Component อ่านข้อมูลจาก Go ตรงที่ API_ORIGIN โดยส่งต่อ cookie ของผู้ใช้ (05 §2)
import "server-only";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { ApiError, toApiError } from "@/lib/problem";
import type { Me } from "@/lib/types";

function apiOrigin(): string {
  const origin = process.env.API_ORIGIN;
  if (!origin) throw new Error("ไม่ได้ตั้ง API_ORIGIN ใน frontend/.env.local");
  return origin;
}

/** GET ไปที่ Go ส่งต่อ cookie เสมอ และไม่ cache (ข้อมูลการจองต้องสดทุกครั้ง) */
export async function apiGet<T>(path: string): Promise<T> {
  const cookieStore = await cookies();
  const res = await fetch(apiOrigin() + path, {
    headers: { cookie: cookieStore.toString() },
    cache: "no-store",
  });
  if (!res.ok) throw await toApiError(res);
  return res.json() as Promise<T>;
}

/** ผู้ใช้ปัจจุบัน หรือ null ถ้ายังไม่ได้ login */
export async function getMe(): Promise<Me | null> {
  try {
    return await apiGet<Me>("/api/me");
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return null;
    throw err;
  }
}

/**
 * หน้าที่ต้อง login: ถ้ายังไม่ได้ login พาไป login แล้วกลับมาที่ returnTo
 * เป็น UX เท่านั้น สิทธิ์จริงตรวจที่ Go ทุก endpoint
 */
export async function requireMe(returnTo: string): Promise<Me> {
  const me = await getMe();
  if (!me) redirect("/api/auth/login?return_to=" + encodeURIComponent(returnTo));
  return me;
}

/** แปลง 404 จาก Go เป็น null เพื่อให้หน้าเรียก notFound() เอง */
export async function apiGetOrNull<T>(path: string): Promise<T | null> {
  try {
    return await apiGet<T>(path);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return null;
    throw err;
  }
}
