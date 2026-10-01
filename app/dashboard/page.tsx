"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, ChevronLeft, ChevronRight, ExternalLink, LogOut, Plus, Search, ShieldCheck, UserRound } from "lucide-react";
import ProfileEditor from "@/components/ProfileEditor";
import ChangePassword from "@/components/ChangePassword";
import ProfessorAccountCard from "@/components/ProfessorAccountCard";
import { Header, Footer } from "@/components/SiteShell";
import { emptyResearch, type DirectoryPage, type Professor, type SessionUser } from "@/lib/professors";

const emptyPage: DirectoryPage = { items: [], total: 0, page: 1, pageSize: 20, totalPages: 0 };
const emptyNew = { name: "", slug: "", rank: "استادیار", faculty: "", email: "", username: "", password: "" };
const ranks = ["مربی", "استادیار", "دانشیار", "استاد تمام"];

export default function DashboardPage() {
  const router = useRouter();
  const [user, setUser] = useState<SessionUser | null>(null);
  const [directory, setDirectory] = useState<DirectoryPage>(emptyPage);
  const [faculties, setFaculties] = useState<string[]>([]);
  const [selectedProfile, setSelectedProfile] = useState<Professor | null>(null);
  const [loading, setLoading] = useState(true);
  const [listBusy, setListBusy] = useState(false);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [facultyFilter, setFacultyFilter] = useState("");
  const [page, setPage] = useState(1);
  const [creating, setCreating] = useState(false);
  const [newProfile, setNewProfile] = useState(emptyNew);
  const [createBusy, setCreateBusy] = useState(false);
  const [createError, setCreateError] = useState("");
  const [facultyName, setFacultyName] = useState("");
  const [facultyBusy, setFacultyBusy] = useState(false);
  const [facultyMessage, setFacultyMessage] = useState("");

  const loadDirectory = useCallback(async (q: string, faculty: string, currentPage: number, signal?: AbortSignal) => {
    const params = new URLSearchParams({ q, faculty, page: String(currentPage) });
    const response = await fetch(`/api/professors?${params}`, { cache: "no-store", signal });
    if (!response.ok) throw new Error("دریافت فهرست اساتید انجام نشد");
    return await response.json() as DirectoryPage;
  }, []);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const authResponse = await fetch("/api/auth/me", { credentials: "same-origin", cache: "no-store" });
        if (!authResponse.ok) { router.replace("/login"); return; }
        const account = await authResponse.json() as SessionUser;
        const facultyResponse = await fetch("/api/faculties", { cache: "no-store" });
        if (!facultyResponse.ok) throw new Error();
        const names = await facultyResponse.json() as string[];
        if (account.role === "admin") {
          const list = await loadDirectory("", "", 1);
          if (!alive) return;
          setDirectory(list); setSelectedProfile(list.items[0] ?? null);
        } else {
          const response = await fetch(`/api/professors/${encodeURIComponent(account.professorSlug || "")}`, { cache: "no-store" });
          if (!response.ok) throw new Error();
          const profile = await response.json() as Professor;
          if (!alive) return;
          setSelectedProfile(profile);
        }
        setFaculties(names); setUser(account);
      } catch { if (alive) setError("ارتباط با سرور برقرار نشد. سرویس Go را بررسی کنید."); }
      finally { if (alive) setLoading(false); }
    })();
    return () => { alive = false; };
  }, [router, loadDirectory]);

  useEffect(() => {
    const timer = window.setTimeout(() => { setQuery(search.trim()); setPage(1); }, 400);
    return () => window.clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    if (user?.role !== "admin") return;
    const controller = new AbortController();
    setListBusy(true);
    loadDirectory(query, facultyFilter, page, controller.signal)
      .then((data) => { setDirectory(data); setError(""); })
      .catch(() => { if (!controller.signal.aborted) setError("دریافت فهرست اساتید انجام نشد."); })
      .finally(() => { if (!controller.signal.aborted) setListBusy(false); });
    return () => controller.abort();
  }, [user, query, facultyFilter, page, loadDirectory]);

  async function refreshDirectory() {
    try { setDirectory(await loadDirectory(query, facultyFilter, page)); }
    catch { setError("به‌روزرسانی فهرست انجام نشد."); }
  }

  async function logout() {
    await fetch("/api/auth/logout", { method: "POST", credentials: "same-origin" }).catch(() => {});
    router.replace("/login"); router.refresh();
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setCreateBusy(true); setCreateError("");
    const profile: Professor = { slug: newProfile.slug.trim(), name: newProfile.name.trim(), image: "", rank: newProfile.rank, faculty: newProfile.faculty, email: newProfile.email.trim(), education: [], teaching: [], research: emptyResearch(), theses: [], interests: [], downloads: [] };
    try {
      const response = await fetch("/api/admin/professors", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ profile, username: newProfile.username.trim(), password: newProfile.password }) });
      const result = await response.json();
      if (!response.ok) { setCreateError(result.error || "ایجاد حساب انجام نشد"); return; }
      setSelectedProfile(result as Professor); setCreating(false); setNewProfile(emptyNew); await refreshDirectory();
    } catch { setCreateError("ارتباط با سرور برقرار نشد."); }
    finally { setCreateBusy(false); }
  }

  async function addFaculty(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setFacultyBusy(true); setFacultyMessage("");
    try {
      const response = await fetch("/api/admin/faculties", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name: facultyName.trim() }) });
      const result = await response.json();
      if (!response.ok) { setFacultyMessage(result.error || "ثبت دانشکده انجام نشد."); return; }
      const listResponse = await fetch("/api/faculties", { cache: "no-store" });
      if (listResponse.ok) setFaculties(await listResponse.json() as string[]);
      setNewProfile((current) => ({ ...current, faculty: result.name }));
      setFacultyName(""); setFacultyMessage("دانشکده ثبت شد و اکنون در فرم استاد قابل انتخاب است.");
    } catch { setFacultyMessage("ارتباط با سرور برقرار نشد."); }
    finally { setFacultyBusy(false); }
  }

  return <div className="site-page dashboard-page"><Header /><main className="container dashboard-main">
    <div className="dashboard-top"><div><span className="dashboard-overline">پنل کاربری دانشگاه هرمزگان</span><h1>{user?.role === "admin" ? "مدیریت اساتید" : "پروفایل من"}</h1><p>{user?.role === "admin" ? "مدیریت حساب‌ها، دانشکده‌ها و اطلاعات علمی" : "اطلاعات پروفایل و رزومه خود را به‌روز کنید."}</p></div><div className="dashboard-top-actions"><Link href="/ostad" className="dashboard-public"><ExternalLink size={16} /> مشاهده پرتال</Link><button type="button" onClick={logout}><LogOut size={16} /> خروج از حساب</button></div></div>
    {loading && <div className="dashboard-status">در حال بارگذاری...</div>}{error && <div className="dashboard-status form-error" role="alert">{error}</div>}
    {user && <div className="dashboard-layout"><aside className="dashboard-side"><div className="account-card"><span>{user.role === "admin" ? <ShieldCheck size={20} /> : <UserRound size={20} />}</span><div><strong>{user.username}</strong><small>{user.role === "admin" ? "مدیر سامانه" : "استاد"}</small></div></div>
      {user.role === "admin" && <>
        <div className="dashboard-side-title"><span>اساتید</span><small>{directory.total.toLocaleString("fa-IR")} پروفایل</small></div>
        <label className="dashboard-search"><Search size={17} /><input type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="جست‌وجوی استاد یا گروه..." aria-label="جست‌وجوی اساتید" /></label>
        <select className="dashboard-faculty-filter" value={facultyFilter} onChange={(event) => { setFacultyFilter(event.target.value); setPage(1); }} aria-label="فیلتر دانشکده"><option value="">همه دانشکده‌ها</option>{faculties.map((item) => <option value={item} key={item}>{item}</option>)}</select>
        <div className="dashboard-professors" aria-busy={listBusy}>{directory.items.map((item) => <button key={item.slug} type="button" className={selectedProfile?.slug === item.slug && !creating ? "selected" : ""} onClick={() => { setSelectedProfile(item); setCreating(false); }}><span><strong>{item.name}</strong><small>{item.rank} · {item.faculty}</small></span><ArrowLeft size={15} /></button>)}{!directory.items.length && <p className="dashboard-list-empty">استادی پیدا نشد.</p>}</div>
        {directory.totalPages > 1 && <div className="dashboard-pager"><button type="button" disabled={directory.page <= 1 || listBusy} onClick={() => setPage(directory.page - 1)} aria-label="صفحه قبل"><ChevronRight size={18} /></button><span>{directory.page.toLocaleString("fa-IR")} از {directory.totalPages.toLocaleString("fa-IR")}</span><button type="button" disabled={directory.page >= directory.totalPages || listBusy} onClick={() => setPage(directory.page + 1)} aria-label="صفحه بعد"><ChevronLeft size={18} /></button></div>}
        <button className="dashboard-add" type="button" onClick={() => { setCreating(true); setCreateError(""); }}><Plus size={16} /> افزودن استاد</button>
        <form className="dashboard-faculty-add" onSubmit={addFaculty}><strong>افزودن دانشکده</strong><div><input required minLength={3} value={facultyName} onChange={(event) => setFacultyName(event.target.value)} placeholder="نام دانشکده" aria-label="نام دانشکده جدید" /><button type="submit" disabled={facultyBusy} aria-label="ثبت دانشکده"><Plus size={17} /></button></div>{facultyMessage && <p role="status">{facultyMessage}</p>}</form>
      </>}
      {user.role === "professor" && selectedProfile && <Link href={`/ostad/${selectedProfile.slug}`} className="dashboard-side-link">مشاهده پروفایل عمومی <ArrowLeft size={16} /></Link>}
    </aside><div className="dashboard-content">{creating && user.role === "admin" ? <form onSubmit={create} className="editor-form create-form"><div className="editor-form-head"><div><span className="dashboard-overline">حساب جدید</span><h2>افزودن استاد</h2><p>ابتدا حساب را بسازید؛ سپس سوابق آموزشی و پژوهشی را تکمیل کنید.</p></div></div><div className="editor-grid"><label>نام و نام خانوادگی<input required value={newProfile.name} onChange={(e) => setNewProfile({ ...newProfile, name: e.target.value })} /></label><label>شناسه پروفایل<input required dir="ltr" pattern="[a-z0-9]+(-[a-z0-9]+)*" value={newProfile.slug} onChange={(e) => setNewProfile({ ...newProfile, slug: e.target.value })} placeholder="first-last" /></label><label>مرتبه علمی<select required value={newProfile.rank} onChange={(e) => setNewProfile({ ...newProfile, rank: e.target.value })}>{ranks.map((rank) => <option key={rank}>{rank}</option>)}</select></label><label>دانشکده<select required value={newProfile.faculty} onChange={(e) => setNewProfile({ ...newProfile, faculty: e.target.value })}><option value="">انتخاب دانشکده</option>{faculties.map((item) => <option value={item} key={item}>{item}</option>)}</select></label><label>رایانامه<input type="email" dir="ltr" value={newProfile.email} onChange={(e) => setNewProfile({ ...newProfile, email: e.target.value })} /></label><label>نام کاربری ورود<input required minLength={3} dir="ltr" value={newProfile.username} onChange={(e) => setNewProfile({ ...newProfile, username: e.target.value })} /></label><label>گذرواژه اولیه<input required minLength={12} type="password" dir="ltr" autoComplete="new-password" value={newProfile.password} onChange={(e) => setNewProfile({ ...newProfile, password: e.target.value })} /></label></div>{createError && <p className="form-error" role="alert">{createError}</p>}<div className="editor-actions"><button type="submit" disabled={createBusy}><Plus size={17} /> {createBusy ? "در حال ایجاد..." : "ایجاد حساب استاد"}</button></div></form> : selectedProfile ? <><ProfileEditor key={selectedProfile.slug} professor={selectedProfile} faculties={faculties} onSaved={(updated) => { setSelectedProfile(updated); void refreshDirectory(); }} />{user.role === "admin" && <ProfessorAccountCard key={selectedProfile.slug} slug={selectedProfile.slug} />}</> : <div className="dashboard-status">پروفایلی برای نمایش موجود نیست.</div>}</div></div>}
    {user && <ChangePassword />}
  </main><Footer /></div>;
}
