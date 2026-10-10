// ที่ตั้งร้าน: แกะพิกัดจากลิงก์แผนที่ และสร้าง URL ของแผนที่
// แกะฝั่งเบราว์เซอร์เท่านั้น เซิร์ฟเวอร์รับแค่ตัวเลข จึงไม่ต้องให้เซิร์ฟเวอร์ยิง HTTP ตามลิงก์ของผู้ใช้ (เลี่ยง SSRF)

export type LatLng = { lat: number; lng: number };

const LAT = 90;
const LNG = 180;

/** พิกัดต้องอยู่ในช่วงที่ใช้ได้จริง (กติกาเดียวกับที่ Go ตรวจซ้ำ) */
export function validLatLng(lat: number, lng: number): boolean {
  return Number.isFinite(lat) && Number.isFinite(lng) && Math.abs(lat) <= LAT && Math.abs(lng) <= LNG;
}

// รูปแบบที่พบใน URL ของ Google Maps เรียงจากแม่นยำที่สุดลงมา
// !3d…!4d… คือพิกัดของหมุดจริง ส่วน @…,… คือจุดที่กล้องเล็งอยู่ (ใกล้เคียงแต่ไม่เป๊ะ)
const PATTERNS = [
  /!3d(-?\d+(?:\.\d+)?)!4d(-?\d+(?:\.\d+)?)/,
  /[?&]query=(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)/,
  /[?&]q=(?:loc:)?(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)/,
  /[?&]ll=(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)/,
  /@(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)/,
  /^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$/, // วาง "13.7466, 100.5347" มาตรง ๆ
];

/**
 * แกะพิกัดจากข้อความที่ผู้ใช้วางมา รองรับลิงก์ Google Maps แบบเต็มและพิกัดล้วน
 * ลิงก์ย่อ (maps.app.goo.gl) แกะไม่ได้เพราะพิกัดไม่ได้อยู่ใน URL ต้องเปิดลิงก์ก่อนแล้วค่อยคัดลอกมาใหม่
 */
export function parseLatLng(input: string): LatLng | null {
  const text = input.trim();
  if (text === "") return null;
  for (const re of PATTERNS) {
    const m = re.exec(text);
    if (!m) continue;
    const lat = Number(m[1]);
    const lng = Number(m[2]);
    if (validLatLng(lat, lng)) return { lat, lng };
  }
  return null;
}

/** true เมื่อเป็นลิงก์ย่อของ Google Maps ซึ่งต้องให้ผู้ใช้เปิดก่อนถึงจะได้พิกัด */
export function isShortMapLink(input: string): boolean {
  return /(?:maps\.app\.goo\.gl|goo\.gl\/maps)/i.test(input);
}

/** ลิงก์เปิดใน Google Maps (ไม่ต้องใช้ API key) */
export function googleMapsUrl({ lat, lng }: LatLng): string {
  return `https://www.google.com/maps/search/?api=1&query=${lat},${lng}`;
}

/**
 * URL ของแผนที่ OpenStreetMap สำหรับฝังใน iframe
 * เลือก OSM เพราะไม่ต้องใช้ API key ไม่มีค่าใช้จ่าย และไม่โหลดสคริปต์ติดตามของผู้ให้บริการรายใหญ่
 */
export function osmEmbedUrl({ lat, lng }: LatLng, span = 0.004): string {
  const bbox = [lng - span, lat - span / 2, lng + span, lat + span / 2].map((n) => n.toFixed(6)).join("%2C");
  return `https://www.openstreetmap.org/export/embed.html?bbox=${bbox}&layer=mapnik&marker=${lat}%2C${lng}`;
}
