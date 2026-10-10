import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return (
    <main className="mx-auto max-w-(--w-wide) px-4 pt-5 pb-18 sm:px-6" aria-busy="true" aria-label="กำลังโหลดร้าน">
      <Skeleton className="mt-8 h-[380px] rounded-[32px] bg-white/60" />
      <div className="mt-7 grid gap-7 lg:grid-cols-[minmax(0,1fr)_400px]">
        <Skeleton className="h-96 rounded-(--r-card) bg-white/60" />
        <Skeleton className="h-[520px] rounded-(--r-panel) bg-white/60" />
      </div>
    </main>
  );
}
