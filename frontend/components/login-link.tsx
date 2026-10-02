"use client";

// ลิงก์ login พากลับมาหน้าปัจจุบันหลัง login (เปลี่ยนหน้าเต็มเพราะต้องไป Keycloak)
import { usePathname, useSearchParams } from "next/navigation";
import type { ComponentProps } from "react";

import { Button } from "@/components/ui/button";

export function LoginLink({ children = "เข้าสู่ระบบ", ...props }: ComponentProps<typeof Button>) {
  const pathname = usePathname();
  const search = useSearchParams().toString();
  const returnTo = pathname + (search ? "?" + search : "");
  return (
    <Button asChild {...props}>
      <a href={"/api/auth/login?return_to=" + encodeURIComponent(returnTo)}>{children}</a>
    </Button>
  );
}
