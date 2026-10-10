"use client";

// ลิงก์ในเมนู ต้องเป็น Client Component เพราะใช้ usePathname เพื่อไฮไลต์หน้าปัจจุบัน
import Link from "next/link";
import { usePathname } from "next/navigation";

const links = [
  { href: "/", label: "ร้านอาหาร", login: false },
  { href: "/bookings", label: "การจองของฉัน", login: true },
  { href: "/favorites", label: "ร้านที่ชอบ", login: true },
  { href: "/my/restaurants", label: "ร้านของฉัน", login: true },
];

function isActive(pathname: string, href: string) {
  if (href === "/") return pathname === "/" || pathname.startsWith("/restaurants");
  return pathname.startsWith(href);
}

export function NavLinks({ loggedIn }: { loggedIn: boolean }) {
  const pathname = usePathname();
  return (
    <nav aria-label="เมนูหลัก" className="order-last flex w-full grow gap-1.5 overflow-x-auto sm:order-none sm:w-auto">
      {links
        .filter((l) => loggedIn || !l.login)
        .map((l) => {
          const active = isActive(pathname, l.href);
          return (
            <Link
              key={l.href}
              href={l.href}
              aria-current={active ? "page" : undefined}
              className={
                "rounded-[14px] px-4 py-2.5 font-semibold whitespace-nowrap " +
                (active
                  ? "bg-white/92 text-(--ink) shadow-[inset_0_1px_0_#fff,0_4px_12px_rgba(107,91,214,0.10)]"
                  : "text-(--ink-2) hover:bg-white/50")
              }
            >
              {l.label}
            </Link>
          );
        })}
    </nav>
  );
}
