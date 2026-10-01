import type { Metadata } from "next";
import "./globals.css";
import "./portal-additions.css";
import "./directory-dashboard.css";

export const metadata: Metadata = {
  title: {
    default: "پرتال اساتید | دانشگاه هرمزگان",
    template: "%s | پرتال اساتید دانشگاه هرمزگان",
  },
  description: "اطلاعات اعضای هیئت علمی دانشگاه هرمزگان، سوابق تحصیلی، پژوهش و تدریس",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="fa" dir="rtl">
      <body>{children}</body>
    </html>
  );
}
