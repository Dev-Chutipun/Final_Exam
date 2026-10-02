// ภาพปก placeholder ตามหมวดแรกของร้าน (05 §5 ตารางภาพปก) ใช้เมื่อร้านไม่มีรูป
const coverBySlug: Record<string, string> = {
  thai: "var(--cover-peach)",
  seafood: "var(--cover-peach)",
  chinese: "var(--cover-peach)",
  vietnamese: "var(--cover-mint)",
  buffet: "var(--cover-mint)",
  isan: "var(--cover-butter)",
  italian: "var(--cover-butter)",
  "cafe-drinks": "var(--cover-latte)",
  "dessert-bakery": "var(--cover-latte)",
  "steak-grill": "var(--cover-lilac)",
  korean: "var(--cover-lilac)",
  japanese: "var(--cover-sky)",
};

export function coverGradient(firstCategorySlug?: string): string {
  return coverBySlug[firstCategorySlug ?? ""] ?? "var(--cover-lilac)";
}

export function imageUrl(restaurantId: number, imageId: number): string {
  return `/api/restaurants/${restaurantId}/images/${imageId}`;
}
