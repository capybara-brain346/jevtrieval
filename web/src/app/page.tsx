"use client";

import { useState, type FormEvent } from "react";
import ReactMarkdown from "react-markdown";
import { QueryForm } from "../components/query-form";
import { SiteHeader } from "../components/site-header";
import { search } from "../lib/api";
import type { SearchResponse } from "../lib/types";
import { formatProbability } from "../lib/utils";

export default function HomePage() {
  const [query, setQuery] = useState("");
  const [result, setResult] = useState<SearchResponse | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const value = query.trim();
    if (!value) {
      setError("Enter a research question.");
      return;
    }
    setPending(true);
    setError(null);
    setResult(null);
    try {
      setResult(await search(value));
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Search failed.");
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="min-h-dvh bg-neutral-50">
      <SiteHeader />

      <div className="mx-auto flex max-w-5xl flex-col gap-6 px-4 py-8 sm:px-6 sm:py-12">
        <div className="max-w-2xl">
          <h1 className="text-3xl font-semibold text-neutral-950 text-balance sm:text-4xl">Research answers, backed by scored evidence.</h1>
          <p className="mt-3 leading-7 text-neutral-600 text-pretty">Ask a question to retrieve relevant documents, verify their probabilities, and synthesize a final answer.</p>
        </div>

        <QueryForm query={query} pending={pending} error={error} onQueryChange={(value) => { setQuery(value); setError(null); }} onSubmit={submit} />

        {pending ? (
          <section aria-live="polite" aria-busy="true" className="rounded-xl border border-neutral-200 bg-white p-6 shadow-sm">
            <p className="font-medium text-neutral-900">Retrieving and scoring documents…</p>
            <div className="mt-4 space-y-3" aria-hidden="true">
              <div className="h-20 rounded-lg bg-neutral-100" />
              <div className="h-20 rounded-lg bg-neutral-100" />
            </div>
          </section>
        ) : null}

        {result ? (
          <>
            <section aria-labelledby="answer-heading" className="rounded-xl border border-neutral-200 bg-white p-5 shadow-sm sm:p-7">
              <p className="text-sm font-medium text-neutral-500">Final answer</p>
              <h2 id="answer-heading" className="mt-1 text-2xl font-semibold text-neutral-950 text-balance">Answer</h2>
              <div className="markdown mt-5 text-neutral-700 text-pretty">
                <ReactMarkdown>{result.answer}</ReactMarkdown>
              </div>
            </section>

            <section aria-labelledby="documents-heading" className="rounded-xl border border-neutral-200 bg-white p-5 shadow-sm sm:p-7">
              <div className="flex items-end justify-between gap-4 border-b border-neutral-200 pb-5">
                <div>
                  <p className="text-sm font-medium text-neutral-500">Jev verification</p>
                  <h2 id="documents-heading" className="mt-1 text-xl font-semibold text-neutral-950 text-balance">Evidence and scores</h2>
                </div>
                <p className="text-sm tabular-nums text-neutral-500">{result.documents.length} documents</p>
              </div>
              {result.documents.length ? (
                <ol className="divide-y divide-neutral-200">
                  {result.documents.map((document) => (
                    <li key={document.doc_id} className="py-5 last:pb-0">
                      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-6">
                        <div>
                          <p className="font-mono text-xs tabular-nums text-neutral-500">#{document.rank} · {document.doc_id}</p>
                          <h3 className="mt-1 font-medium text-neutral-950 text-pretty">{document.title}</h3>
                        </div>
                        <p className="shrink-0 rounded-md border border-neutral-200 bg-neutral-50 px-2.5 py-1 font-mono text-xs tabular-nums text-neutral-700">Vector {formatProbability(document.vector_score)}</p>
                      </div>
                      <ul className="mt-4 grid gap-2">
                        <li className="flex flex-col justify-between gap-1 rounded-md bg-neutral-50 px-3 py-2.5 sm:flex-row sm:gap-4">
                          <span className="text-sm font-medium text-neutral-900">Direct query relevance</span>
                          {document.query_error ? <span className="shrink-0 text-sm font-medium text-red-700">Failed</span> : <span className="shrink-0 font-mono text-sm font-semibold tabular-nums text-neutral-950">{formatProbability(document.query_probability!)}</span>}
                        </li>
                        {document.probabilities.map((item) => (
                          <li key={item.question} className="flex flex-col justify-between gap-1 rounded-md bg-neutral-50 px-3 py-2.5 sm:flex-row sm:gap-4">
                            <span className="text-sm text-neutral-700 text-pretty">{item.question}</span>
                            {item.error ? <span className="shrink-0 text-sm font-medium text-red-700">Failed</span> : <span className="shrink-0 font-mono text-sm font-semibold tabular-nums text-neutral-950">{formatProbability(item.probability!)}</span>}
                          </li>
                        ))}
                      </ul>
                      <details className="mt-3">
                        <summary className="cursor-pointer text-sm font-medium text-neutral-600">View document text</summary>
                        <p className="mt-3 whitespace-pre-wrap border-l-2 border-neutral-200 pl-4 text-sm leading-6 text-neutral-600 text-pretty">{document.text}</p>
                      </details>
                    </li>
                  ))}
                </ol>
              ) : <p className="mt-5 text-sm text-neutral-600">No documents matched. Try a broader question.</p>}
            </section>
          </>
        ) : null}
      </div>
    </main>
  );
}
