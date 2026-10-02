import { coverGradient, imageUrl } from "@/lib/cover";

/**
 * รูปปกของร้าน: มีรูป → <img> ธรรมดาชี้ /api/restaurants/{id}/images/{imageId}
 * ไม่มีรูป → พื้นไล่สีตามหมวดแรก · กล่องกำหนดความสูงไว้ กันหน้าเลื่อนตอนรูปโหลด
 */
export function CoverImage({
  restaurantId,
  imageId,
  categorySlug,
  alt,
  className = "",
  children,
}: {
  restaurantId: number;
  imageId: number | null;
  categorySlug?: string;
  alt: string;
  className?: string;
  children?: React.ReactNode;
}) {
  return (
    <div
      className={`relative overflow-hidden shadow-[inset_0_1px_0_rgba(255,255,255,0.7)] ${className}`}
      style={{ background: coverGradient(categorySlug) }}
    >
      {imageId !== null && (
        // ใช้ <img> ธรรมดาตาม 05 §2: รูปมาจาก Go ผ่าน rewrite และ cache แบบ immutable อยู่แล้ว
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={imageUrl(restaurantId, imageId)}
          alt={alt}
          loading="lazy"
          className="absolute inset-0 size-full object-cover"
        />
      )}
      {children}
    </div>
  );
}
