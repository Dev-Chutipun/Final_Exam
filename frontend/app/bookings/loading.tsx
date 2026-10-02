import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return (
    <main className="mx-auto max-w-[1000px] px-4 pt-12 pb-18 sm:px-6" aria-busy="true" aria-label="กำลังโหลดการจอง">
      <Skeleton className="h-12 w-72 rounded-2xl bg-white/60" />
      <div className="mt-8 flex flex-col gap-4.5">
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} className="h-36 rounded-(--r-card) bg-white/60" />
        ))}
      </div>
    </main>
  );
}
