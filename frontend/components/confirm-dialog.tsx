"use client";

// หน้าต่างยืนยันก่อนทำสิ่งที่ย้อนกลับไม่ได้ (อาร์ตบอร์ด 8) ปิดด้วย Esc ได้ (Radix AlertDialog)
import type { ReactNode } from "react";

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";

export function ConfirmDialog({
  trigger,
  title,
  children,
  confirmLabel,
  cancelLabel,
  busy,
  onConfirm,
  open,
  onOpenChange,
}: {
  trigger?: ReactNode;
  title: string;
  children: ReactNode;
  confirmLabel: string;
  cancelLabel: string;
  busy?: boolean;
  onConfirm: () => void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      {trigger && <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>}
      <AlertDialogContent className="flex flex-col gap-4 rounded-[32px] border border-white/95 bg-white/90 p-7 shadow-[0_24px_60px_rgba(75,62,160,0.22)] backdrop-blur-[26px] sm:max-w-md">
        <AlertDialogTitle className="font-heading text-2xl font-semibold">{title}</AlertDialogTitle>
        <AlertDialogDescription asChild>
          <div className="flex flex-col gap-3 text-[15px] text-(--ink-2)">{children}</div>
        </AlertDialogDescription>
        {/* ไม่ใช้ AlertDialogFooter ของ shadcn เพราะมีแถบพื้นสีเทาและมุมโค้ง 12px ติดมาในตัว ซึ่งไม่เข้ากับการ์ดมุมโค้ง 32px */}
        <div className="mt-1 flex flex-row justify-end gap-2.5">
          <AlertDialogCancel asChild>
            <Button variant="glass" disabled={busy}>
              {cancelLabel}
            </Button>
          </AlertDialogCancel>
          {/* ไม่ใช้ AlertDialogAction เพราะต้องรอผลจากเซิร์ฟเวอร์ก่อนปิด */}
          <Button variant="danger-strong" disabled={busy} onClick={onConfirm}>
            {busy ? "กำลังดำเนินการ…" : confirmLabel}
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
