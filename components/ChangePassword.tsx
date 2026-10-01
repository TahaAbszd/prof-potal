"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { LockKeyhole } from "lucide-react";

export default function ChangePassword() {
  const router = useRouter();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setMessage("");
    try {
      const response = await fetch("/api/auth/change-password", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ currentPassword, newPassword }) });
      if (!response.ok) { const result = await response.json(); setMessage(result.error || "تغییر گذرواژه انجام نشد"); return; }
      router.replace("/login"); router.refresh();
    } catch { setMessage("ارتباط با سرور برقرار نشد"); }
    finally { setBusy(false); }
  }

  return <details className="own-password"><summary><LockKeyhole size={17} /> تغییر گذرواژه حساب من</summary><form onSubmit={submit}><label>گذرواژه فعلی<input required type="password" autoComplete="current-password" dir="ltr" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} /></label><label>گذرواژه جدید<input required minLength={12} maxLength={72} type="password" autoComplete="new-password" dir="ltr" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></label>{message && <p className="form-error" role="alert">{message}</p>}<button type="submit" disabled={busy}>{busy ? "در حال ثبت..." : "ثبت گذرواژه جدید"}</button></form><p>پس از تغییر گذرواژه، همه نشست‌ها بسته می‌شوند و باید دوباره وارد شوید.</p></details>;
}
