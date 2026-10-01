import Directory from "@/components/Directory";
import { getFaculties, getProfessorPage } from "@/lib/professors";

export const dynamic = "force-dynamic";

export default async function FacultyDirectory() {
  const [initialPage, faculties] = await Promise.all([getProfessorPage(), getFaculties()]);
  return <Directory initialPage={initialPage} faculties={faculties} />;
}
