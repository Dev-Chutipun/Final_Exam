"use client";

// หน้าต่างแจ้งผลกลางจอหลังทำรายการสำเร็จ (จอง ยกเลิก รีวิว จัดการร้าน)
// วางไว้ชั้นเดียวใน layout แล้วให้ทุกหน้าเรียกผ่าน useSuccessDialog() จึงไม่ต้องมี state ซ้ำทุกหน้า
import { createContext, useContext, useState } from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";

export type SuccessMessage = {
  title: string;
  /** บรรทัดรายละเอียดใต้หัวข้อ เช่น วันเวลาที่จอง */
  detail?: string;
};

// ค่าเริ่มต้นเป็นฟังก์ชันว่าง เผื่อมี component ที่เรียกนอก provider (จะไม่พังแต่ก็ไม่แสดงอะไร)
const SuccessContext = createContext<(m: SuccessMessage) => void>(() => {});

/** คืนฟังก์ชันสำหรับเปิดหน้าต่างแจ้งผลสำเร็จ */
export function useSuccessDialog() {
  return useContext(SuccessContext);
}

export function SuccessDialogProvider({ children }: { children: React.ReactNode }) {
  // setState ของ React มี identity คงที่ ส่งเป็นค่า context ตรง ๆ ได้โดยไม่ทำให้ลูกเรนเดอร์ใหม่
  const [message, setMessage] = useState<SuccessMessage | null>(null);
  return (
    <SuccessContext.Provider value={setMessage}>
      {children}
      <Dialog open={message !== null} onOpenChange={(open) => !open && setMessage(null)}>
        <DialogContent
          showCloseButton={false}
          className="flex flex-col items-center gap-4 rounded-[32px] border border-white/95 bg-white/92 p-8 text-center shadow-[0_24px_60px_rgba(75,62,160,0.22)] backdrop-blur-[26px] sm:max-w-sm"
        >
          <span aria-hidden="true" className="flex size-16 items-center justify-center rounded-full border border-white bg-(--st-mint-bg)">
            <svg width="34" height="34" viewBox="0 0 24 24" fill="none" stroke="var(--st-mint-ink)" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="5 12.5 10 17.5 19 7.5" />
            </svg>
          </span>
          <DialogTitle className="font-heading text-2xl font-semibold">{message?.title}</DialogTitle>
          {/* DialogDescription ต้องมีเสมอเพื่อให้ Radix ผูก aria-describedby ได้ */}
          <DialogDescription className="text-[15px] whitespace-pre-line text-(--ink-2)">{message?.detail ?? "ระบบบันทึกรายการของคุณเรียบร้อยแล้ว"}</DialogDescription>
          {/* ไม่ใช้ DialogFooter ของ shadcn เพราะมีแถบพื้นสีเทาและมุมโค้ง 12px ติดมาในตัว ซึ่งไม่เข้ากับการ์ดมุมโค้ง 32px */}
          <div className="mt-1 w-full">
            <DialogClose asChild>
              <Button variant="cta" size="confirm" className="w-full" autoFocus>
                ตกลง
              </Button>
            </DialogClose>
          </div>
        </DialogContent>
      </Dialog>
    </SuccessContext.Provider>
  );
}
