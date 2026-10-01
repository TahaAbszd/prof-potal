"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { LogIn, LayoutDashboard } from "lucide-react";

export default function AuthNav() {
  const [signedIn, setSignedIn] = useState(false);
  useEffect(() => {
    fetch("/api/auth/me", { credentials: "same-origin", cache: "no-store" })
      .then((response) => { if (response.ok) setSignedIn(true); })
      .catch(() => {});
  }, []);
  return <Link href={signedIn ? "/dashboard" : "/login"} className="header-login">
    {signedIn ? <LayoutDashboard size={17} /> : <LogIn size={17} />}
    {signedIn ? "پنل کاربری" : "ورود استاد / مدیر"}
  </Link>;
}
