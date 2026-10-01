import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Profile from "@/components/Profile";
import { getProfessor } from "@/lib/professors";

type Props = { params: Promise<{ slug: string }> };

export const dynamic = "force-dynamic";

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const professor = await getProfessor((await params).slug);
  return { title: professor?.name ?? "استاد یافت نشد" };
}

export default async function ProfessorPage({ params }: Props) {
  const professor = await getProfessor((await params).slug);
  if (!professor) notFound();
  return <Profile professor={professor} />;
}
