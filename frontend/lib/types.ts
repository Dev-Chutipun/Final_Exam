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
  accepting_bookings: boolean;
  avg_rating: number | null;
  review_count: number;
  is_owner: boolean;
  my_review: { id: number; rating: number; comment: string } | null;
};

export type Review = {
  id: number;
  rating: number;
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

export type DailyBookings = {
  restaurant_id: number;
  date: string;
  capacity: number;
  peak: number;
  peak_at: string | null;
  items: {
    id: number;
    start_at: string;
    end_at: string;
    party_size: number;
    status: "active" | "cancelled";
    customer_display_name: string;
  }[];
};

export type SettingsConflict = { booking_id: number; start_at: string; end_at: string; reason: "capacity" | "opening_hours" };
