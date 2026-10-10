import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return (
    <main className="mx-auto max-w-(--w-list) px-4 pt-12 pb-18 sm:px-6" aria-busy="true" aria-label="กำลังโหลด">
      <Skeleton className="h-12 w-60 rounded-2xl bg-white/60" />
      <div className="mt-8 flex flex-col gap-5">
        {Array.from({ length: 2 }, (_, i) => (
          <Skeleton key={i} className="h-52 rounded-(--r-panel) bg-white/60" />
        ))}
      </div>
    </main>
  );
}
