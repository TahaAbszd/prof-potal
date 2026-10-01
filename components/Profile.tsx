"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import {
  ArrowLeft, ArrowRight, BookMarked, BookOpen, ChevronLeft,
  Download, FileText, FlaskConical, GraduationCap, Heart, Lightbulb,
  Mail, MapPin, Microscope, Presentation, School, UserRound,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { Professor } from "@/lib/professors";
import { Footer, Header } from "./SiteShell";

type SectionId = "education" | "research" | "teaching" | "theses" | "interests" | "downloads";
type ResearchId = "journals" | "conferences" | "books" | "projects" | "patents";

const sections: Array<{ id: SectionId; label: string; icon: LucideIcon; description: string }> = [
  { id: "education", label: "اطلاعات تحصیلی", icon: GraduationCap, description: "سوابق آموزشی و مدارک دانشگاهی" },
  { id: "research", label: "اطلاعات پژوهشی", icon: Microscope, description: "انتشارات و فعالیت‌های علمی" },
  { id: "teaching", label: "عناوین تدریس شده", icon: Presentation, description: "دروس ارائه شده در مقاطع مختلف" },
  { id: "theses", label: "پایان‌نامه‌ها", icon: BookOpen, description: "راهنمایی و مشاوره پژوهش‌های دانشجویی" },
  { id: "interests", label: "علاقه‌مندی‌ها", icon: Heart, description: "زمینه‌های تخصصی و علایق پژوهشی" },
  { id: "downloads", label: "دانلودها", icon: Download, description: "فایل‌ها و منابع آموزشی" },
];

const researchSections: Array<{ id: ResearchId; label: string; icon: LucideIcon }> = [
  { id: "journals", label: "مجلات", icon: FileText },
  { id: "conferences", label: "همایش و کنفرانس‌ها", icon: Presentation },
  { id: "books", label: "کتاب‌ها", icon: BookMarked },
  { id: "projects", label: "طرح‌های پژوهشی", icon: FlaskConical },
  { id: "patents", label: "اختراع‌ها", icon: Lightbulb },
];

function EmptyState({ title, description, icon: Icon }: { title: string; description: string; icon: LucideIcon }) {
  return <div className="empty-state"><span className="empty-state-icon"><Icon size={29} strokeWidth={1.6} /></span><strong>{title}</strong><p>{description}</p></div>;
}

export default function Profile({ professor }: { professor: Professor }) {
  useEffect(() => {
    void fetch(`/api/professors/${encodeURIComponent(professor.slug)}/view`, { method: "POST", credentials: "same-origin" }).catch(() => {});
  }, [professor.slug]);
  const [active, setActive] = useState<SectionId>("education");
  const [research, setResearch] = useState<ResearchId>("journals");
  const current = sections.find((section) => section.id === active)!;
  const ActiveIcon = current.icon;
  const currentResearch = researchSections.find((section) => section.id === research)!;

  return <div className="site-page profile-page">
    <Header />
    <main>
      <div className="profile-breadcrumb container"><Link href="/ostad">فهرست اساتید</Link><ChevronLeft size={14} /><span>{professor.name}</span></div>
      <section className="profile-hero container">
        <div className="profile-identity">
          <div className="profile-photo"><Image src={professor.image || "/images/faculty-placeholder.svg"} alt={`تصویر ${professor.name}`} fill sizes="(max-width: 640px) 105px, 150px" priority /></div>
          <div className="profile-heading"><span className="profile-kicker"><span className="status-dot" /> عضو هیئت علمی دانشگاه هرمزگان</span><h1>{professor.name}</h1><div className="profile-tags"><span>{professor.rank}</span><span>{professor.faculty}</span></div></div>
        </div>
        <div className="profile-hero-actions"><a className="email-button" href={`/api/professors/${professor.slug}/resume.pdf`} download><Download size={18} /> دریافت رزومه PDF</a>{professor.email && <a className="back-link" href={`mailto:${professor.email}`}><Mail size={16} /> ارتباط با استاد</a>}<Link href="/ostad" className="back-link"><ArrowRight size={17} /> بازگشت به فهرست</Link></div>
        <span className="profile-watermark" aria-hidden="true">HU</span>
      </section>

      <div className="profile-layout container">
        <aside className="profile-sidebar">
          <div className="side-card"><h2>درباره استاد</h2><dl>
            <div><dt><School size={17} /> دانشکده</dt><dd>{professor.faculty}</dd></div>
            {professor.department && <div><dt><UsersIcon /> گروه آموزشی</dt><dd>{professor.department}</dd></div>}
            {professor.field && <div><dt><BookOpen size={17} /> رشته تخصصی</dt><dd>{professor.field}</dd></div>}
            {professor.office && <div><dt><MapPin size={17} /> محل استقرار</dt><dd>{professor.office}</dd></div>}
            {professor.email && <div><dt><Mail size={17} /> رایانامه</dt><dd><a href={`mailto:${professor.email}`} dir="ltr">{professor.email}</a></dd></div>}
          </dl></div>
          <div className="side-note"><GraduationCap size={22} /><span>دانشگاه هرمزگان<br /><small>دانش، پژوهش و توسعه</small></span></div>
        </aside>

        <div className="profile-main">
          <nav className="profile-tabs" aria-label="بخش‌های پروفایل استاد">
            {sections.map(({ id, label, icon: Icon }) => <button type="button" key={id} className={`profile-tab ${active === id ? "active" : ""}`} onClick={() => setActive(id)} aria-pressed={active === id}><Icon size={18} strokeWidth={1.8} /><span>{label}</span></button>)}
          </nav>
          <section className="content-card" aria-live="polite">
            <div className="content-heading"><span className="content-heading-icon"><ActiveIcon size={24} strokeWidth={1.7} /></span><div><span className="content-overline">پروفایل علمی</span><h2>{current.label}</h2><p>{current.description}</p></div></div>

            {active === "education" && (professor.education.length ? <div className="degree-list">{professor.education.map((item, index) => <article className="degree-card" key={`${item.degree}-${index}`}><div className="degree-header"><span className="degree-icon"><GraduationCap size={22} /></span><div><small>مقطع تحصیلی</small><h3>{item.degree}</h3></div><span className="degree-number">{String(index + 1).padStart(2, "0")}</span></div><dl className="detail-grid"><div><dt>نام دانشگاه</dt><dd>{item.university}</dd></div>{item.country && <div><dt>کشور</dt><dd>{item.country}</dd></div>}{item.date && <div><dt>تاریخ</dt><dd>{item.date}</dd></div>}{item.thesis && <div className="wide"><dt>موضوع پایان‌نامه</dt><dd>{item.thesis}</dd></div>}</dl></article>)}</div> : <EmptyState icon={GraduationCap} title="اطلاعات تحصیلی ثبت نشده است" description="سوابق تحصیلی این استاد هنوز ثبت نشده است." />)}

            {active === "research" && <div className="research-area"><div className="research-tabs" role="tablist" aria-label="دسته‌بندی فعالیت‌های پژوهشی">{researchSections.map(({ id, label, icon: Icon }) => <button type="button" role="tab" aria-selected={research === id} className={research === id ? "selected" : ""} key={id} onClick={() => setResearch(id)}><Icon size={17} />{label}</button>)}</div><div role="tabpanel" className="research-panel"><div className="research-panel-title"><span>۰{researchSections.findIndex((item) => item.id === research) + 1}</span><h3>{currentResearch.label}</h3></div>{professor.research[research]?.length ? <ol className="profile-entry-list">{professor.research[research].map((item, index) => <li key={`${index}-${item}`}>{item}</li>)}</ol> : <EmptyState icon={currentResearch.icon} title={`موردی در بخش ${currentResearch.label} ثبت نشده است`} description="اطلاعات این بخش هنوز ثبت نشده است." />}</div></div>}

            {active === "teaching" && (professor.teaching.length ? <div className="teaching-list">{professor.teaching.map((group, index) => <article className="teaching-card" key={`${group.degree}-${index}`}><div className="teaching-head"><span><Presentation size={21} /></span><div><small>مقطع</small><h3>{group.degree}</h3></div></div><div className="subject-list">{group.subjects.map((subject) => <span key={subject}><span className="subject-dot" />{subject}</span>)}</div></article>)}</div> : <EmptyState icon={Presentation} title="عنوان درسی ثبت نشده است" description="فهرست دروس این استاد هنوز ثبت نشده است." />)}

            {active === "theses" && (professor.theses?.length ? <div className="profile-entry-list">{professor.theses.map((item, index) => <article key={`${item.title}-${index}`} className="profile-entry"><strong>{item.title}</strong><small>{[item.role, item.degree, item.year].filter(Boolean).join(" · ")}</small></article>)}</div> : <EmptyState icon={BookOpen} title="پایان‌نامه‌ای ثبت نشده است" description="اطلاعات پایان‌نامه‌های این استاد هنوز ثبت نشده است." />)}
            {active === "interests" && (professor.interests?.length ? <div className="interest-chips">{professor.interests.map((item) => <span key={item}>{item}</span>)}</div> : <EmptyState icon={Heart} title="علاقه‌مندی پژوهشی ثبت نشده است" description="زمینه‌های مورد علاقه این استاد هنوز ثبت نشده است." />)}
            {active === "downloads" && (professor.downloads?.length ? <div className="profile-entry-list">{professor.downloads.map((item, index) => <a className="download-entry" key={`${item.title}-${index}`} href={item.url} target="_blank" rel="noopener noreferrer"><FileText size={20} /><span>{item.title}</span><Download size={17} /></a>)}</div> : <EmptyState icon={Download} title="فایلی برای دانلود موجود نیست" description="منبعی برای دانلود در این پروفایل ثبت نشده است." />)}
          </section>
          <Link href="/ostad" className="bottom-back"><ArrowRight size={17} /> بازگشت به فهرست اساتید <ArrowLeft size={16} /></Link>
        </div>
      </div>
    </main>
    <Footer />
  </div>;
}

function UsersIcon() { return <UserRound size={17} />; }
