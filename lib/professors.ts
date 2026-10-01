import seed from "@/data/professors.json";

export type Education = { degree: string; university: string; country?: string; date?: string; thesis?: string };
export type Teaching = { degree: string; subjects: string[] };
export type Research = { journals: string[]; conferences: string[]; books: string[]; projects: string[]; patents: string[] };
export type Thesis = { title: string; role?: string; degree?: string; year?: string };
export type Resource = { title: string; url: string };
export type Professor = {
  slug: string;
  name: string;
  image: string;
  rank: string;
  faculty: string;
  department?: string;
  field?: string;
  office?: string;
  email?: string;
  education: Education[];
  teaching: Teaching[];
  research: Research;
  theses: Thesis[];
  interests: string[];
  downloads: Resource[];
  views?: number;
  searches?: number;
};

export type DirectoryPage = { items: Professor[]; total: number; page: number; pageSize: number; totalPages: number };

export type SessionUser = { username: string; role: "admin" | "professor"; professorSlug?: string };

const fallback = seed as Professor[];
const internalAPI = process.env.API_INTERNAL_URL || "http://127.0.0.1:8080";

export async function getProfessorPage(query = "", faculty = "", page = 1): Promise<DirectoryPage> {
  try {
    const params = new URLSearchParams({ q: query, faculty, page: String(page) });
    const response = await fetch(`${internalAPI}/api/professors?${params}`, { cache: "no-store" });
    if (response.ok) return await response.json() as DirectoryPage;
  } catch { /* The seed keeps the public portal usable before the API starts. */ }
  const matching = fallback.filter((professor) =>
    (!faculty || professor.faculty === faculty) &&
    (!query || `${professor.name} ${professor.department ?? ""} ${professor.field ?? ""}`.toLocaleLowerCase("fa").includes(query.toLocaleLowerCase("fa")))
  );
  const ordered = [...matching].sort((a, b) => {
    const ranks: Record<string, number> = { "مربی": 1, "استادیار": 2, "دانشیار": 3, "استاد تمام": 4 };
    return (ranks[b.rank] ?? 0) - (ranks[a.rank] ?? 0) || a.name.localeCompare(b.name, "fa", { sensitivity: "base" });
  });
  const totalPages = Math.ceil(ordered.length / 20);
  const currentPage = Math.max(1, Math.min(page, totalPages || 1));
  return { items: ordered.slice((currentPage - 1) * 20, currentPage * 20), total: ordered.length, page: currentPage, pageSize: 20, totalPages };
}

export async function getFaculties(): Promise<string[]> {
  try {
    const response = await fetch(`${internalAPI}/api/faculties`, { cache: "no-store" });
    if (response.ok) return await response.json() as string[];
  } catch { /* Fallback below. */ }
  return [...new Set(fallback.map((professor) => professor.faculty))].sort((a, b) => a.localeCompare(b, "fa"));
}

export async function getProfessor(slug: string): Promise<Professor | undefined> {
  try {
    const response = await fetch(`${internalAPI}/api/professors/${encodeURIComponent(slug)}`, { cache: "no-store" });
    if (response.ok) return await response.json() as Professor;
    if (response.status === 404) return undefined;
  } catch { /* See fallback below. */ }
  return fallback.find((professor) => professor.slug === slug);
}

export const emptyResearch = (): Research => ({ journals: [], conferences: [], books: [], projects: [], patents: [] });
