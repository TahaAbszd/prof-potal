"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowRight, Eye, EyeOff, LockKeyhole, ShieldCheck, UserRound } from "lucide-react";
import { Header, Footer } from "@/components/SiteShell";

export default function LoginPage() {
  const router = useRouter();
  const [role, setRole] = useState<"professor" | "admin">("professor");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [visible, setVisible] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(""); setBusy(true);
    try {
      const response = await fetch("/api/auth/login", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ role, username, password }) });
      const result = await response.json();
      if (!response.ok) { setError(result.error || "ورود انجام نشد"); return; }
      router.replace("/dashboard"); router.refresh();
    } catch { setError("ارتباط با سرور برقرار نشد. سرویس Go را بررسی کنید."); }
    finally { setBusy(false); }
  }

  return <div className="site-page auth-page"><Header /><main className="auth-main container">
    <div className="auth-aside"><div className="auth-aside-badge"><ShieldCheck size={22} /> دسترسی امن به پرتال</div><h1>ورود به<br />پرتال اساتید</h1><p>اطلاعات علمی و حرفه‌ای خود را در پنل اختصاصی مدیریت کنید.</p><div className="auth-aside-rule" /><span>دانشگاه هرمزگان</span></div>
    <div className="auth-card"><div className="auth-card-top"><span className="auth-icon"><LockKeyhole size={25} /></span><h2>ورود به حساب کاربری</h2><p>نوع حساب را انتخاب کنید و مشخصات ورود را وارد کنید.</p></div>
      <div className="role-switch" aria-label="نوع حساب"><button type="button" className={role === "professor" ? "selected" : ""} onClick={() => { setRole("professor"); setError(""); }}><UserRound size={18} /> استاد</button><button type="button" className={role === "admin" ? "selected" : ""} onClick={() => { setRole("admin"); setError(""); }}><ShieldCheck size={18} /> مدیر سامانه</button></div>
      <form onSubmit={submit} className="auth-form"><label>نام کاربری<input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" required placeholder="نام کاربری" dir="ltr" /></label><label>گذرواژه<span className="password-wrap"><input value={password} onChange={(event) => setPassword(event.target.value)} type={visible ? "text" : "password"} autoComplete="current-password" required placeholder="گذرواژه" dir="ltr" /><button type="button" onClick={() => setVisible(!visible)} aria-label={visible ? "پنهان کردن گذرواژه" : "نمایش گذرواژه"}>{visible ? <EyeOff size={18} /> : <Eye size={18} />}</button></span></label>{error && <p role="alert" className="form-error">{error}</p>}<button className="auth-submit" disabled={busy} type="submit">{busy ? "در حال بررسی..." : `ورود ${role === "professor" ? "استاد" : "مدیر"}`} <ArrowRight size={18} /></button></form>
      <Link href="/ostad" className="auth-back"><ArrowRight size={16} /> بازگشت به فهرست اساتید</Link>
    </div>
  </main><Footer /></div>;
}
