import type { SearchResponse } from "./types";

const baseUrl = (process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "");

export async function search(query: string): Promise<SearchResponse> {
  const response = await fetch(`${baseUrl}/v1/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query }),
  });
  if (!response.ok) {
    const body = await response.json().catch(() => null) as { detail?: string } | null;
    throw new Error(body?.detail || `Search failed with status ${response.status}.`);
  }
  return response.json() as Promise<SearchResponse>;
}
