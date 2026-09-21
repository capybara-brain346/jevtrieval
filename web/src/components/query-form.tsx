"use client";

import type { FormEvent } from "react";
import { cn } from "../lib/utils";

type QueryFormProps = {
  query: string;
  pending: boolean;
  error: string | null;
  onQueryChange: (query: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

export function QueryForm({ query, pending, error, onQueryChange, onSubmit }: QueryFormProps) {
  return (
    <form onSubmit={onSubmit} className="rounded-xl border border-neutral-200 bg-white p-5 shadow-sm sm:p-6">
      <label htmlFor="query" className="text-sm font-medium text-neutral-950">Research question</label>
      <textarea
        id="query"
        name="query"
        value={query}
        onChange={(event) => onQueryChange(event.target.value)}
        aria-invalid={Boolean(error)}
        aria-describedby={error ? "query-error" : undefined}
        placeholder="Does vitamin B12 deficiency increase homocysteine?"
        rows={4}
        className={cn(
          "mt-3 block w-full resize-y rounded-lg border bg-white px-4 py-3 text-base text-neutral-950 shadow-sm placeholder:text-neutral-400",
          "border-neutral-300 focus:border-neutral-950 focus:outline-none focus:ring-2 focus:ring-neutral-200",
          error && "border-red-400 focus:border-red-600 focus:ring-red-100",
        )}
      />
      <div className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p id="query-error" role={error ? "alert" : undefined} className={cn("min-h-5 text-sm", error ? "text-red-700" : "text-neutral-500")}>{error ?? "Results appear after retrieval and verification finish."}</p>
        <button type="submit" disabled={pending} className="inline-flex min-h-11 items-center justify-center rounded-lg bg-neutral-950 px-5 py-2.5 text-sm font-medium text-white shadow-sm hover:bg-neutral-800 disabled:cursor-not-allowed disabled:bg-neutral-400">
          {pending ? "Searching…" : "Search"}
        </button>
      </div>
    </form>
  );
}
