// ชนิดข้อมูลของ API (ตรงกับ JSON ที่ Go ตอบ)

export type Me = { id: number; display_name: string };

export type Category = { id: number; slug: string; name_th: string };

export type Page<T> = { items: T[]; page: number; page_size: number; total: number };

export type RestaurantListItem = {
  id: number;
  name: string;
  capacity: number;
  open_time: string;
  close_time: string;
  avg_rating: number | null;
  review_count: number;
  accepting_bookings: boolean;
  categories: Category[];
  cover_image_id: number | null;
  is_mine: boolean;
  is_favorite: boolean;
};

/** รายการใน GET /api/me/favorites */
export type FavoriteRestaurant = {
  id: number;
  name: string;
  cover_image_id: number | null;
  open_time: string;
  close_time: string;
  accepting_bookings: boolean;
  avg_rating: number | null;
  review_count: number;
  favorited_at: string;
};

export type RestaurantDetail = {
  id: number;
  name: string;
  description: string;
  categories: Category[];
  image_ids: number[];
  capacity: number;
  open_time: string;
  close_time: string;
  cancel_window_minutes: number;
  min_lead_minutes: number;
  address: string | null;
  latitude: number | null;
  longitude: number | null;
  accepting_bookings: boolean;
  avg_rating: number | null;
  review_count: number;
  is_owner: boolean;
  is_favorite: boolean;
  aspect_averages: ReviewAspects | null;
  my_review: { id: number; rating: number; aspects: ReviewAspects; comment: string } | null;
};

/** คะแนน 4 ด้านที่ผู้รีวิวให้ · คะแนนรวมคือค่าเฉลี่ยของสี่ค่านี้ ฐานข้อมูลคำนวณให้ */
export type ReviewAspects = { food: number; ambience: number; service: number; value: number };

export const ASPECTS = [
  { key: "food", label: "อาหาร" },
  { key: "ambience", label: "บรรยากาศ" },
  { key: "service", label: "บริการ" },
  { key: "value", label: "ความคุ้มค่า" },
] as const satisfies readonly { key: keyof ReviewAspects; label: string }[];

export type Review = {
  id: number;
  rating: number;
  aspects: ReviewAspects;
  comment: string;
  display_name: string;
  created_at: string;
  edited: boolean;
  is_mine: boolean;
};

export type Slot = { start: string; end: string; remaining: number; limited: boolean; bookable: boolean };

export type Availability = {
  restaurant_id: number;
  date: string;
  capacity: number;
  accepting_bookings: boolean;
  slots: Slot[];
};

export type Booking = {
  id: number;
  restaurant_id: number;
  party_size: number;
  start_at: string;
  end_at: string;
  status: "active" | "cancelled";
  cancel_deadline: string;
};

export type MyBooking = Booking & {
  restaurant_name: string;
  is_past: boolean;
  can_modify: boolean;
  restaurant_deleted: boolean;
};

export type MyBookings = Page<MyBooking> & { counts: { upcoming: number; past: number; cancelled: number } };

export type MyRestaurant = {
  id: number;
  name: string;
  cover_image_id: number | null;
  capacity: number;
  open_time: string;
  close_time: string;
  accepting_bookings: boolean;
  avg_rating: number | null;
  review_count: number;
  upcoming_count: number;
};

export type RestaurantBooking = {
  id: number;
  start_at: string;
  end_at: string;
  party_size: number;
  status: "active" | "cancelled";
  customer_display_name: string;
};

/** มุมมองรายวันของเจ้าของร้าน (ส่ง date) — มี peak เพราะวัดได้ในวันทำการเดียว */
export type DailyBookings = {
  restaurant_id: number;
  date: string;
  capacity: number;
  peak: number;
  peak_at: string | null;
  items: RestaurantBooking[];
};

/** รายการการจองทุกวันของร้าน แบ่งหน้า (ส่ง tab) */
export type RestaurantBookings = Page<RestaurantBooking> & {
  restaurant_id: number;
  capacity: number;
  counts: { upcoming: number; past: number; cancelled: number };
};

export type SettingsConflict = { booking_id: number; start_at: string; end_at: string; reason: "capacity" | "opening_hours" };
