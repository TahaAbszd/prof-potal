"use client";

import { FormEvent, useEffect, useState } from "react";

type AccountStatus = { exists: boolean; username?: string };

export default function ProfessorAccountCard({ slug }: { slug: string }) {
  const [status, setStatus] = useState<AccountStatus | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    fetch(`/api/admin/professors/${encodeURIComponent(slug)}/account`, { credentials: "same-origin", cache: "no-store", signal: controller.signal })
      .then(async (response) => { if (!response.ok) throw new Error(); return await response.json() as AccountStatus; })
      .then(setStatus)
      .catch(() => { if (!controller.signal.aborted) setLoadError(true); });
    return () => controller.abort();
  }, [slug]);

  async function createAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setMessage("");
    try {
      const response = await fetch(`/api/admin/professors/${encodeURIComponent(slug)}/account`, { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username: username.trim(), password }) });
      const result = await response.json();
      if (!response.ok) { setMessage(result.error || "ایجاد حساب انجام نشد."); return; }
      setStatus({ exists: true, username: result.username });
      setPassword("");
      setMessage("حساب ورود استاد ساخته شد.");
    } catch { setMessage("ارتباط با سرور برقرار نشد."); }
    finally { setBusy(false); }
  }

  async function resetPassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setMessage("");
    try {
      const response = await fetch(`/api/admin/professors/${encodeURIComponent(slug)}/password`, { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ password }) });
      if (!response.ok) { const result = await response.json().catch(() => null); setMessage(result?.error || "تغییر گذرواژه انجام نشد."); return; }
      setPassword(""); setMessage("گذرواژه تغییر کرد و نشست‌های قبلی استاد بسته شد.");
    } catch { setMessage("ارتباط با سرور برقرار نشد."); }
    finally { setBusy(false); }
  }

  return <section className="reset-card account-management-card">
    {loadError ? <p role="alert">بررسی حساب ورود انجام نشد. صفحه را دوباره بارگذاری کنید.</p> : status === null ? <p>در حال بررسی حساب ورود...</p> : status.exists ? <>
      <h3>حساب ورود استاد</h3><p>نام کاربری: <strong dir="ltr">{status.username}</strong></p>
      <form className="reset-row" onSubmit={resetPassword}><input required type="password" dir="ltr" minLength={12} maxLength={72} autoComplete="new-password" placeholder="گذرواژه جدید (حداقل ۱۲ نویسه)" value={password} onChange={(event) => setPassword(event.target.value)} /><button type="submit" disabled={busy || password.length < 12}>تغییر گذرواژه</button></form>
    </> : <>
      <h3>ایجاد حساب ورود استاد</h3><p>این پروفایل عمومی است. برای ورود استاد، یک نام کاربری و گذرواژهٔ اختصاصی تعریف کنید.</p>
      <form className="account-create-fields" onSubmit={createAccount}><label>نام کاربری<input required dir="ltr" minLength={3} maxLength={64} value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="off" /></label><label>گذرواژه اولیه<input required type="password" dir="ltr" minLength={12} maxLength={72} value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" /></label><button type="submit" disabled={busy || password.length < 12}>ایجاد حساب</button></form>
    </>}
    {message && <p role="status" className="account-message">{message}</p>}
  </section>;
}
