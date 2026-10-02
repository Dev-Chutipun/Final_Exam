package booking

import (
	"math/rand/v2"
	"testing"
	"time"
)

// 01 §10.3 Differential test
//
// สุ่มสถานการณ์บนตาราง 48 ช่อง (ช่องละ 30 นาทีใน 1 วัน) แล้วเทียบ MaxLoad กับ oracle
// oracle ไล่ทุกช่องใน [s, e) รวมขนาดของช่วงที่ครอบจุดเริ่มช่อง แล้วเอาค่ามากสุด
// oracle ถูกแน่นอนเพราะทุกเวลาลงช่อง แต่ช้า ส่วน MaxLoad เร็วกว่าแต่ต้องพิสูจน์ว่าถูก
const (
	diffSeed      = 20261011
	diffScenarios = 10_000
	diffSlots     = 48
)

// slotSpan คือช่วงในหน่วยช่อง [Start, End) ใช้พิมพ์สถานการณ์ที่พังให้อ่านง่าย
type slotSpan struct{ Start, End, Size int }

func TestMaxLoadDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(diffSeed, diffSeed))
	day := time.Date(2026, 10, 11, 0, 0, 0, 0, Bangkok)
	slot := func(k int) time.Time { return day.Add(time.Duration(k) * SlotSize) }

	for i := range diffScenarios {
		// existing 0–15 ช่วง แต่ละช่วงยาว 1–6 ช่อง ขนาด 1–10
		spans := make([]slotSpan, rng.IntN(16))
		existing := make([]Interval, len(spans))
		for j := range spans {
			length := 1 + rng.IntN(6)
			start := rng.IntN(diffSlots - length + 1)
			spans[j] = slotSpan{start, start + length, 1 + rng.IntN(10)}
			existing[j] = Interval{Start: slot(start), End: slot(start + length), Size: spans[j].Size}
		}
		// คำขอแบบสุ่ม [s, e) ภายในวัน
		s := rng.IntN(diffSlots)
		e := s + 1 + rng.IntN(diffSlots-s)

		gotLoad, gotAt := MaxLoad(existing, slot(s), slot(e))
		wantLoad, wantAtSlot := oracleMaxLoad(spans, s, e)

		if gotLoad != wantLoad || !gotAt.Equal(slot(wantAtSlot)) {
			t.Fatalf("seed %d scenario %d: request slots [%d, %d) existing %+v\nMaxLoad = %d at %s, oracle = %d at slot %d (%s)",
				diffSeed, i, s, e, spans,
				gotLoad, gotAt.In(Bangkok).Format("15:04"),
				wantLoad, wantAtSlot, slot(wantAtSlot).Format("15:04"))
		}

		// PeakAll คือ MaxLoad ของทั้งวัน ตรวจกับ oracle ตัวเดียวกันได้
		if gotPeak, _ := PeakAll(existing); gotPeak != oraclePeak(spans) {
			t.Fatalf("seed %d scenario %d: existing %+v\nPeakAll = %d, oracle = %d",
				diffSeed, i, spans, gotPeak, oraclePeak(spans))
		}
	}
}

// oracleMaxLoad คืนโหลดสูงสุดในช่อง [s, e) และช่องแรกที่เกิดโหลดนั้น
func oracleMaxLoad(spans []slotSpan, s, e int) (load, atSlot int) {
	load, atSlot = -1, s
	for k := s; k < e; k++ {
		sum := 0
		for _, sp := range spans {
			if sp.Start <= k && k < sp.End {
				sum += sp.Size
			}
		}
		if sum > load {
			load, atSlot = sum, k
		}
	}
	return load, atSlot
}

func oraclePeak(spans []slotSpan) int {
	peak, _ := oracleMaxLoad(spans, 0, diffSlots)
	return peak
}
