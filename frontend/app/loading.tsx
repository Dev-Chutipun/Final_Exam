import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return (
    <main className="mx-auto max-w-[1232px] px-6 py-12" aria-busy="true" aria-label="กำลังโหลด">
      <Skeleton className="h-12 w-72 rounded-2xl bg-white/60" />
      <div className="mt-8 grid gap-7 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} className="h-72 rounded-(--r-card) bg-white/60" />
        ))}
      </div>
    </main>
  );
}
