"use client";

// หน้าทดสอบ spike (docs/02-auth.md §8) จะถูกแทนด้วยหน้าจริงในส่วนที่ 5
import { useEffect, useState } from "react";

export default function SpikePage() {
  const [me, setMe] = useState<unknown>(null);
  const [echo, setEcho] = useState<unknown>(null);
  const [upload, setUpload] = useState<unknown>(null);
  const [files, setFiles] = useState<FileList | null>(null);

  useEffect(() => {
    fetch("/api/spike/me").then((r) => r.json()).then(setMe);
  }, []);

  async function sendEcho() {
    const r = await fetch("/api/spike/echo", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ hello: "spike" }),
    });
    setEcho({ status: r.status, body: await r.json() });
  }

  async function sendUpload() {
    if (!files) return;
    const fd = new FormData();
    for (const f of Array.from(files)) fd.append("images", f);
    // ไม่ตั้ง Content-Type เอง ให้เบราว์เซอร์ใส่ boundary ให้
    const r = await fetch("/api/spike/upload", { method: "POST", body: fd });
    setUpload({ status: r.status, body: await r.text() });
  }

  return (
    <main>
      <h1>DevPool spike</h1>

      <h2>1. ผู้ใช้ปัจจุบัน (GET /api/spike/me)</h2>
      <pre>{JSON.stringify(me, null, 2)}</pre>
      <p>
        <a href="/api/auth/login">เข้าสู่ระบบ</a>
      </p>
      <form method="POST" action="/api/auth/logout">
        <button type="submit">ออกจากระบบ</button>
      </form>

      <h2>2. POST JSON ผ่าน rewrite (CrossOriginProtection)</h2>
      <button onClick={sendEcho}>ส่ง POST /api/spike/echo</button>
      <pre>{JSON.stringify(echo, null, 2)}</pre>

      <h2>3. อัปโหลด multipart</h2>
      <input type="file" multiple onChange={(e) => setFiles(e.target.files)} />
      <button onClick={sendUpload}>อัปโหลด</button>
      <pre>{JSON.stringify(upload, null, 2)}</pre>
    </main>
  );
}
