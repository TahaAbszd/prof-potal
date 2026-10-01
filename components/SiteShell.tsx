import Image from "next/image";
import Link from "next/link";
import { ArrowUpLeft, GraduationCap } from "lucide-react";
import AuthNav from "./AuthNav";

export function Header() {
  return (
    <header className="site-header">
      <div className="header-inner container">
        <Link href="/ostad" className="brand" aria-label="پرتال اساتید دانشگاه هرمزگان">
          <span className="brand-mark"><Image src="/images/university-logo.png" alt="نشان دانشگاه هرمزگان" width={42} height={50} /></span>
          <span className="brand-copy"><strong>دانشگاه هرمزگان</strong><small>پرتال اعضای هیئت علمی</small></span>
        </Link>
        <nav className="header-nav" aria-label="ناوبری اصلی">
          <Link href="/ostad" className="header-link">فهرست اساتید</Link>
          <AuthNav />
          <a href="https://hormozgan.ac.ir" className="header-external" target="_blank" rel="noopener noreferrer">
            وب‌سایت دانشگاه <ArrowUpLeft size={16} strokeWidth={1.8} />
          </a>
        </nav>
      </div>
    </header>
  );
}

export function Footer() {
  return (
    <footer className="site-footer">
      <div className="container footer-inner">
        <div className="footer-brand"><span className="footer-icon"><GraduationCap size={23} /></span><span><strong>دانشگاه هرمزگان</strong><small>پرتال اطلاعات اعضای هیئت علمی</small></span></div>
        <span>بندرعباس، دانشگاه هرمزگان</span>
        <span>© دانشگاه هرمزگان</span>
      </div>
    </footer>
  );
}
