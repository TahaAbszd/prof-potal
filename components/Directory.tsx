"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { ArrowLeft, ArrowUpLeft, ChevronLeft, ChevronRight, Search, SlidersHorizontal, UsersRound } from "lucide-react";
import type { DirectoryPage } from "@/lib/professors";
import { Footer, Header } from "./SiteShell";

export default function Directory({ initialPage, faculties }: { initialPage: DirectoryPage; faculties: string[] }) {
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [faculty, setFaculty] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState(initialPage);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const timer = window.setTimeout(() => { setQuery(search.trim()); setPage(1); }, 450);
    return () => window.clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    if (!query && !faculty && page === 1) { setResult(initialPage); setLoading(false); return; }
    const controller = new AbortController();
    const params = new URLSearchParams({ q: query, faculty, page: String(page) });
    setLoading(true); setError("");
    fetch(`/api/professors?${params}`, { signal: controller.signal, cache: "no-store" })
      .then(async (response) => { if (!response.ok) throw new Error(); return await response.json() as DirectoryPage; })
      .then((data) => {
        setResult(data);
        if (query.length >= 2 && data.total === 1) {
          void fetch("/api/search-events", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ query }) }).catch(() => {});
        }
      })
      .catch(() => { if (!controller.signal.aborted) setError("دریافت نتایج انجام نشد. دوباره تلاش کنید."); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [query, faculty, page, initialPage]);

  const first = result.total ? (result.page - 1) * result.pageSize + 1 : 0;
  const last = Math.min(result.page * result.pageSize, result.total);
  return <div className="site-page">
    <Header />
    <main>
      <section className="directory-hero">
        <div className="hero-glow hero-glow-one" /><div className="hero-glow hero-glow-two" />
        <div className="container directory-hero-inner">
          <div className="hero-copy">
            <div className="eyebrow light"><span className="eyebrow-line" /> دانشگاه هرمزگان</div>
            <h1>اساتید دانشگاه هرمزگان</h1>
            <p>جست‌وجوی اعضای هیئت علمی و دسترسی به اطلاعات آموزشی و پژوهشی آن‌ها</p>
            <a href="#faculty-list" className="hero-cta">یافتن استاد <ArrowLeft size={18} /></a>
          </div>
          <div className="hero-type" aria-hidden="true"><span>HU</span><small>HORMOZGAN UNIVERSITY</small></div>
        </div>
      </section>
      <section id="faculty-list" className="directory-section container">
        <div className="section-intro"><div><div className="eyebrow"><span className="eyebrow-line" /> اعضای هیئت علمی</div><h2>فهرست اساتید</h2><p>با نام، گروه آموزشی یا دانشکده جست‌وجو کنید.</p></div><span className="results-badge"><UsersRound size={17} /> {result.total.toLocaleString("fa-IR")} پروفایل</span></div>
        <div className="filter-panel">
          <label className="search-field"><Search size={20} /><input value={search} onChange={(event) => setSearch(event.target.value)} type="search" placeholder="نام استاد، گروه یا رشته..." aria-label="جست‌وجوی اساتید" /></label>
          <label className="select-field"><SlidersHorizontal size={18} /><select value={faculty} onChange={(event) => { setFaculty(event.target.value); setPage(1); }} aria-label="فیلتر دانشکده"><option value="">همه دانشکده‌ها</option>{faculties.map((item) => <option value={item} key={item}>{item}</option>)}</select><ChevronLeft size={16} className="select-chevron" /></label>
        </div>
        <div className="list-heading" aria-live="polite"><span>نتایج جست‌وجو</span><span>{loading ? "در حال دریافت..." : `${first.toLocaleString("fa-IR")} تا ${last.toLocaleString("fa-IR")} از ${result.total.toLocaleString("fa-IR")}`}</span></div>
        {error && <div className="directory-error" role="alert">{error}<button type="button" onClick={() => { setQuery(search.trim()); setPage(1); }}>تلاش دوباره</button></div>}
        {result.items.length ? <div className="faculty-grid">{result.items.map((professor) => <Link href={`/ostad/${professor.slug}`} className="faculty-card" key={professor.slug}>
          <span className="faculty-card-top"><span className="faculty-avatar"><Image src={professor.image || "/images/faculty-placeholder.svg"} alt={`تصویر ${professor.name}`} fill sizes="90px" /></span><span className="card-arrow"><ArrowUpLeft size={19} /></span></span>
          <span className="faculty-card-body"><small>{professor.rank}</small><strong>{professor.name}</strong><span>{professor.faculty}</span>{professor.department && <span className="faculty-department">{professor.department}</span>}</span>
          <span className="faculty-card-footer">مشاهده پروفایل <ArrowLeft size={17} /></span>
        </Link>)}</div> : !loading && <div className="empty-search"><Search size={28} /><strong>استادی با این مشخصات پیدا نشد</strong><span>عبارت جست‌وجو یا دانشکده را تغییر دهید.</span><button type="button" onClick={() => { setSearch(""); setFaculty(""); setPage(1); }}>پاک کردن فیلترها</button></div>}
        {result.totalPages > 1 && <nav className="directory-pagination" aria-label="صفحه‌بندی اساتید"><button type="button" disabled={result.page <= 1 || loading} onClick={() => setPage(result.page - 1)}><ChevronRight size={17} /> قبلی</button><span>صفحه {result.page.toLocaleString("fa-IR")} از {result.totalPages.toLocaleString("fa-IR")}</span><button type="button" disabled={result.page >= result.totalPages || loading} onClick={() => setPage(result.page + 1)}>بعدی <ChevronLeft size={17} /></button></nav>}
      </section>
    </main>
    <Footer />
  </div>;
}
