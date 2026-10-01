"use client";

import { FormEvent, useState } from "react";
import { Check, Download, Plus, Save, Trash2 } from "lucide-react";
import type { Education, Professor, Research, Resource, Teaching, Thesis } from "@/lib/professors";

const researchFields: Array<{ key: keyof Research; label: string }> = [
  { key: "journals", label: "مجلات" }, { key: "conferences", label: "همایش و کنفرانس‌ها" },
  { key: "books", label: "کتاب‌ها" }, { key: "projects", label: "طرح‌های پژوهشی" }, { key: "patents", label: "اختراع‌ها" },
];

function lines(value: string) { return value.split("\n"); }
function cleaned(items: string[]) { return items.map((item) => item.trim()).filter(Boolean); }

export default function ProfileEditor({ professor, faculties, onSaved }: { professor: Professor; faculties: string[]; onSaved: (item: Professor) => void }) {
  const [draft, setDraft] = useState<Professor>(() => structuredClone(professor));
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  function basic(key: keyof Professor, value: string) { setDraft((current) => ({ ...current, [key]: value })); }
  function updateEducation(index: number, key: keyof Education, value: string) { setDraft((current) => ({ ...current, education: current.education.map((item, i) => i === index ? { ...item, [key]: value } : item) })); }
  function updateTeaching(index: number, key: keyof Teaching, value: string | string[]) { setDraft((current) => ({ ...current, teaching: current.teaching.map((item, i) => i === index ? { ...item, [key]: value } : item) })); }
  function updateThesis(index: number, key: keyof Thesis, value: string) { setDraft((current) => ({ ...current, theses: current.theses.map((item, i) => i === index ? { ...item, [key]: value } : item) })); }
  function updateDownload(index: number, key: keyof Resource, value: string) { setDraft((current) => ({ ...current, downloads: current.downloads.map((item, i) => i === index ? { ...item, [key]: value } : item) })); }
  function updateResearch(key: keyof Research, value: string) { setDraft((current) => ({ ...current, research: { ...current.research, [key]: lines(value) } })); }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setError(""); setMessage("");
    const payload: Professor = {
      ...draft,
      education: draft.education.filter((item) => item.degree.trim() || item.university.trim()),
      teaching: draft.teaching.map((item) => ({ ...item, subjects: cleaned(item.subjects) })).filter((item) => item.degree.trim() || item.subjects.length),
      research: Object.fromEntries(researchFields.map(({ key }) => [key, cleaned(draft.research[key])])) as unknown as Research,
      theses: draft.theses.filter((item) => item.title.trim()),
      interests: cleaned(draft.interests),
      downloads: draft.downloads.filter((item) => item.title.trim() || item.url.trim()),
    };
    try {
      const response = await fetch(`/api/professors/${encodeURIComponent(draft.slug)}`, { method: "PUT", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
      const result = await response.json();
      if (!response.ok) { setError(result.error || "ذخیره انجام نشد"); return; }
      setDraft(result); onSaved(result); setMessage("تغییرات پروفایل ذخیره شد.");
    } catch { setError("ارتباط با سرور برقرار نشد."); }
    finally { setBusy(false); }
  }

  return <form onSubmit={save} className="editor-form">
    <div className="editor-form-head"><div><span className="dashboard-overline">ویرایش پروفایل</span><h2>{draft.name}</h2><p>تغییرات پس از ذخیره در پروفایل عمومی و رزومهٔ PDF نمایش داده می‌شود.</p></div><a href={`/api/professors/${draft.slug}/resume.pdf`} className="editor-pdf"><Download size={16} /> دریافت رزومه</a></div>
    <section className="editor-section"><h3>اطلاعات اصلی</h3><div className="editor-grid">
      <label>نام و نام خانوادگی<input value={draft.name} onChange={(e) => basic("name", e.target.value)} required /></label>
      <label>مرتبه علمی<select value={draft.rank} onChange={(e) => basic("rank", e.target.value)} required>{!["مربی", "استادیار", "دانشیار", "استاد تمام"].includes(draft.rank) && <option value={draft.rank}>{draft.rank}</option>}{["مربی", "استادیار", "دانشیار", "استاد تمام"].map((rank) => <option value={rank} key={rank}>{rank}</option>)}</select></label>
      <label>دانشکده<select value={draft.faculty} onChange={(e) => basic("faculty", e.target.value)} required>{!faculties.includes(draft.faculty) && <option value={draft.faculty}>{draft.faculty}</option>}{faculties.map((item) => <option value={item} key={item}>{item}</option>)}</select></label>
      <label>گروه آموزشی<input value={draft.department || ""} onChange={(e) => basic("department", e.target.value)} /></label>
      <label>رشته تخصصی<input value={draft.field || ""} onChange={(e) => basic("field", e.target.value)} /></label>
      <label>محل استقرار<input value={draft.office || ""} onChange={(e) => basic("office", e.target.value)} /></label>
      <label>رایانامه<input type="email" dir="ltr" value={draft.email || ""} onChange={(e) => basic("email", e.target.value)} /></label>
    </div></section>

    <section className="editor-section"><div className="editor-section-head"><h3>اطلاعات تحصیلی</h3><button type="button" onClick={() => setDraft((current) => ({ ...current, education: [...current.education, { degree: "", university: "" }] }))}><Plus size={16} /> افزودن مدرک</button></div>
      {draft.education.map((item, index) => <div className="editor-repeat" key={index}><div className="editor-repeat-head"><strong>مدرک {index + 1}</strong><button type="button" aria-label="حذف مدرک" onClick={() => setDraft((current) => ({ ...current, education: current.education.filter((_, i) => i !== index) }))}><Trash2 size={16} /></button></div><div className="editor-grid"><label>مقطع<input value={item.degree} onChange={(e) => updateEducation(index, "degree", e.target.value)} /></label><label>دانشگاه<input value={item.university} onChange={(e) => updateEducation(index, "university", e.target.value)} /></label><label>کشور<input value={item.country || ""} onChange={(e) => updateEducation(index, "country", e.target.value)} /></label><label>تاریخ<input value={item.date || ""} onChange={(e) => updateEducation(index, "date", e.target.value)} /></label><label className="full">موضوع پایان‌نامه<input value={item.thesis || ""} onChange={(e) => updateEducation(index, "thesis", e.target.value)} /></label></div></div>)}
      {!draft.education.length && <p className="editor-empty">مدرکی ثبت نشده است.</p>}
    </section>

    <section className="editor-section"><h3>اطلاعات پژوهشی</h3><p className="editor-hint">هر عنوان را در یک خط بنویسید.</p><div className="editor-grid">{researchFields.map(({ key, label }) => <label className="full" key={key}>{label}<textarea rows={3} value={draft.research[key].join("\n")} onChange={(e) => updateResearch(key, e.target.value)} placeholder={`عنوان‌های ${label}`} /></label>)}</div></section>

    <section className="editor-section"><div className="editor-section-head"><h3>عناوین تدریس شده</h3><button type="button" onClick={() => setDraft((current) => ({ ...current, teaching: [...current.teaching, { degree: "", subjects: [] }] }))}><Plus size={16} /> افزودن مقطع</button></div>{draft.teaching.map((item, index) => <div className="editor-repeat" key={index}><div className="editor-repeat-head"><strong>مقطع {index + 1}</strong><button type="button" aria-label="حذف مقطع" onClick={() => setDraft((current) => ({ ...current, teaching: current.teaching.filter((_, i) => i !== index) }))}><Trash2 size={16} /></button></div><div className="editor-grid"><label>مقطع<input value={item.degree} onChange={(e) => updateTeaching(index, "degree", e.target.value)} /></label><label className="full">نام درس‌ها، هر کدام در یک خط<textarea rows={3} value={item.subjects.join("\n")} onChange={(e) => updateTeaching(index, "subjects", lines(e.target.value))} /></label></div></div>)}{!draft.teaching.length && <p className="editor-empty">درسی ثبت نشده است.</p>}</section>

    <section className="editor-section"><div className="editor-section-head"><h3>پایان‌نامه‌ها</h3><button type="button" onClick={() => setDraft((current) => ({ ...current, theses: [...current.theses, { title: "" }] }))}><Plus size={16} /> افزودن پایان‌نامه</button></div>{draft.theses.map((item, index) => <div className="editor-repeat" key={index}><div className="editor-repeat-head"><strong>پایان‌نامه {index + 1}</strong><button type="button" aria-label="حذف پایان‌نامه" onClick={() => setDraft((current) => ({ ...current, theses: current.theses.filter((_, i) => i !== index) }))}><Trash2 size={16} /></button></div><div className="editor-grid"><label className="full">عنوان<input value={item.title} onChange={(e) => updateThesis(index, "title", e.target.value)} /></label><label>نقش (راهنما / مشاور)<input value={item.role || ""} onChange={(e) => updateThesis(index, "role", e.target.value)} /></label><label>مقطع<input value={item.degree || ""} onChange={(e) => updateThesis(index, "degree", e.target.value)} /></label><label>سال<input value={item.year || ""} onChange={(e) => updateThesis(index, "year", e.target.value)} /></label></div></div>)}{!draft.theses.length && <p className="editor-empty">پایان‌نامه‌ای ثبت نشده است.</p>}</section>

    <section className="editor-section"><h3>علاقه‌مندی‌ها</h3><label>هر زمینه در یک خط<textarea rows={4} value={draft.interests.join("\n")} onChange={(e) => setDraft((current) => ({ ...current, interests: lines(e.target.value) }))} /></label></section>

    <section className="editor-section"><div className="editor-section-head"><h3>دانلودها</h3><button type="button" onClick={() => setDraft((current) => ({ ...current, downloads: [...current.downloads, { title: "", url: "" }] }))}><Plus size={16} /> افزودن لینک</button></div>{draft.downloads.map((item, index) => <div className="editor-repeat" key={index}><div className="editor-repeat-head"><strong>فایل {index + 1}</strong><button type="button" aria-label="حذف لینک" onClick={() => setDraft((current) => ({ ...current, downloads: current.downloads.filter((_, i) => i !== index) }))}><Trash2 size={16} /></button></div><div className="editor-grid"><label>عنوان<input value={item.title} onChange={(e) => updateDownload(index, "title", e.target.value)} /></label><label>نشانی فایل<input type="url" dir="ltr" value={item.url} onChange={(e) => updateDownload(index, "url", e.target.value)} placeholder="https://..." /></label></div></div>)}{!draft.downloads.length && <p className="editor-empty">لینکی ثبت نشده است.</p>}</section>
    <div className="editor-actions">{error && <span className="form-error" role="alert">{error}</span>}{message && <span className="form-success"><Check size={17} /> {message}</span>}<button type="submit" disabled={busy}><Save size={18} /> {busy ? "در حال ذخیره..." : "ذخیره تغییرات"}</button></div>
  </form>;
}
