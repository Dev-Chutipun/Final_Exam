export const metadata = { title: "DevPool spike" };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="th">
      <body style={{ fontFamily: "sans-serif", maxWidth: 720, margin: "24px auto", padding: "0 16px" }}>
        {children}
      </body>
    </html>
  );
}
