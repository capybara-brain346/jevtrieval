import type { Metadata } from "next";
import Link from "next/link";
import { SiteHeader } from "../../components/site-header";

export const metadata: Metadata = {
  title: "How retrieval works | Jev Retrieval",
  description: "How Jev Retrieval finds, verifies, and synthesizes scientific evidence.",
};

const stages = [
  ["01", "Decompose", "Turn one research query into 2–6 focused evidence questions."],
  ["02", "Embed", "Convert the original query into a semantic vector."],
  ["03", "Retrieve", "Find 25 candidate documents in the Qdrant index."],
  ["04", "Verify", "Reject irrelevant documents, then select up to five."],
  ["05", "Synthesize", "Write one answer from only the selected document contents."],
] as const;

export default function HowItWorksPage() {
  return (
    <main className="min-h-dvh bg-neutral-50">
      <SiteHeader />

      <div className="mx-auto max-w-5xl px-4 py-12 sm:px-6 sm:py-16">
        <section className="mx-auto max-w-3xl text-center">
          <p className="text-sm font-medium text-neutral-500">How our retrieval works</p>
          <h1 className="mt-3 text-4xl font-semibold text-neutral-950 text-balance sm:text-5xl">From a question to evidence you can inspect.</h1>
          <p className="mx-auto mt-5 max-w-2xl text-lg leading-8 text-neutral-600 text-pretty">Semantic search finds 25 candidates. Jev rejects documents below 0.5 direct relevance, then selects up to five using missing evidence coverage.</p>
        </section>

        <section aria-labelledby="pipeline-heading" className="mt-14 rounded-xl border border-neutral-200 bg-white p-5 shadow-sm sm:p-8">
          <div className="flex items-end justify-between gap-4">
            <div>
              <p className="text-sm font-medium text-neutral-500">The pipeline</p>
              <h2 id="pipeline-heading" className="mt-1 text-2xl font-semibold text-neutral-950 text-balance">One request, five stages</h2>
            </div>
            <span className="hidden font-mono text-xs tabular-nums text-neutral-400 sm:block">POST /v1/search</span>
          </div>

          <ol className="mt-8 grid gap-3 md:grid-cols-5">
            {stages.map(([number, title, description]) => (
              <li key={number} className="relative rounded-lg border border-neutral-200 bg-neutral-50 p-4">
                <span className="font-mono text-xs tabular-nums text-neutral-400">{number}</span>
                <h3 className="mt-6 font-semibold text-neutral-950">{title}</h3>
                <p className="mt-2 text-sm leading-6 text-neutral-600 text-pretty">{description}</p>
              </li>
            ))}
          </ol>

          <div aria-hidden="true" className="mt-5 hidden items-center gap-2 px-6 text-neutral-300 md:flex">
            <span className="size-2 rounded-full bg-neutral-950" />
            <span className="h-px flex-1 bg-neutral-300" />
            <span className="size-2 rounded-full bg-neutral-950" />
            <span className="h-px flex-1 bg-neutral-300" />
            <span className="size-2 rounded-full bg-neutral-950" />
            <span className="h-px flex-1 bg-neutral-300" />
            <span className="size-2 rounded-full bg-neutral-950" />
            <span className="h-px flex-1 bg-neutral-300" />
            <span className="size-2 rounded-full bg-neutral-950" />
          </div>
        </section>

        <section aria-labelledby="layers-heading" className="mt-16">
          <div className="max-w-2xl">
            <p className="text-sm font-medium text-neutral-500">Two layers of relevance</p>
            <h2 id="layers-heading" className="mt-2 text-3xl font-semibold text-neutral-950 text-balance">Fast retrieval first. Deliberate verification second.</h2>
            <p className="mt-4 leading-7 text-neutral-600 text-pretty">Vector similarity is useful for finding candidates, but similarity alone does not mean a document answers the question. The second pass makes that distinction visible.</p>
          </div>

          <div className="mt-8 grid gap-5 md:grid-cols-2">
            <article className="rounded-xl border border-neutral-200 bg-white p-6 shadow-sm">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <span className="font-mono text-xs tabular-nums text-neutral-400">LAYER 01</span>
                  <h3 className="mt-1 text-xl font-semibold text-neutral-950">Semantic retrieval</h3>
                </div>
                <span className="rounded-md border border-neutral-200 bg-neutral-50 px-2 py-1 font-mono text-xs text-neutral-500">Qdrant</span>
              </div>

              <svg role="img" aria-label="A query vector finding nearby document vectors" viewBox="0 0 420 250" className="mt-6 h-auto w-full text-neutral-300">
                <circle cx="210" cy="125" r="92" fill="none" stroke="currentColor" strokeDasharray="4 7" />
                <circle cx="210" cy="125" r="54" fill="#fafafa" stroke="currentColor" />
                <line x1="210" y1="125" x2="276" y2="79" stroke="#a3a3a3" strokeWidth="1.5" />
                <line x1="210" y1="125" x2="284" y2="143" stroke="#a3a3a3" strokeWidth="1.5" />
                <line x1="210" y1="125" x2="151" y2="80" stroke="#a3a3a3" strokeWidth="1.5" />
                <circle cx="210" cy="125" r="17" fill="#171717" />
                <circle cx="276" cy="79" r="12" fill="#404040" />
                <circle cx="284" cy="143" r="10" fill="#737373" />
                <circle cx="151" cy="80" r="9" fill="#a3a3a3" />
                <circle cx="116" cy="169" r="8" fill="#e5e5e5" stroke="#a3a3a3" />
                <circle cx="333" cy="187" r="8" fill="#e5e5e5" stroke="#a3a3a3" />
                <circle cx="82" cy="73" r="7" fill="#e5e5e5" stroke="#a3a3a3" />
                <text x="210" y="159" textAnchor="middle" fill="#525252" fontSize="12">query</text>
                <text x="300" y="75" fill="#525252" fontSize="12">closest documents</text>
              </svg>

              <p className="mt-2 text-sm leading-6 text-neutral-600 text-pretty">The query and indexed documents share one vector space. Cosine similarity returns the closest candidates in ranked order.</p>
            </article>

            <article className="rounded-xl border border-neutral-200 bg-white p-6 shadow-sm">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <span className="font-mono text-xs tabular-nums text-neutral-400">LAYER 02</span>
                  <h3 className="mt-1 text-xl font-semibold text-neutral-950">Evidence verification</h3>
                </div>
                <span className="rounded-md border border-neutral-200 bg-neutral-50 px-2 py-1 font-mono text-xs text-neutral-500">Jev</span>
              </div>

              <div className="mt-8 overflow-hidden rounded-lg border border-neutral-200">
                <table className="w-full border-collapse text-sm">
                  <thead className="bg-neutral-50 text-neutral-500">
                    <tr>
                      <th scope="col" className="px-3 py-2.5 text-left font-medium">Document</th>
                      <th scope="col" className="px-3 py-2.5 text-right font-medium">Q1</th>
                      <th scope="col" className="px-3 py-2.5 text-right font-medium">Q2</th>
                      <th scope="col" className="px-3 py-2.5 text-right font-medium">Q3</th>
                    </tr>
                  </thead>
                  <tbody className="font-mono tabular-nums text-neutral-700">
                    <tr className="border-t border-neutral-200"><th scope="row" className="px-3 py-3 text-left font-sans font-medium text-neutral-950">Doc 01</th><td className="px-3 py-3 text-right font-semibold text-neutral-950">.94</td><td className="px-3 py-3 text-right">.81</td><td className="px-3 py-3 text-right">.72</td></tr>
                    <tr className="border-t border-neutral-200"><th scope="row" className="px-3 py-3 text-left font-sans font-medium text-neutral-950">Doc 02</th><td className="px-3 py-3 text-right">.76</td><td className="px-3 py-3 text-right">.43</td><td className="px-3 py-3 text-right">.18</td></tr>
                    <tr className="border-t border-neutral-200"><th scope="row" className="px-3 py-3 text-left font-sans font-medium text-neutral-950">Doc 03</th><td className="px-3 py-3 text-right">.22</td><td className="px-3 py-3 text-right">.09</td><td className="px-3 py-3 text-right">.67</td></tr>
                  </tbody>
                </table>
              </div>

              <div className="mt-5 flex items-center gap-3 text-xs text-neutral-500">
                <span className="h-px flex-1 bg-neutral-200" />
                <span>one probability per question × document</span>
                <span className="h-px flex-1 bg-neutral-200" />
              </div>
              <p className="mt-5 text-sm leading-6 text-neutral-600 text-pretty">Jev also scores direct relevance to the original query. Selection uses 70% direct relevance and 30% marginal question coverage.</p>
            </article>
          </div>
        </section>

        <section aria-labelledby="question-heading" className="mt-16 grid gap-8 rounded-xl border border-neutral-200 bg-white p-6 shadow-sm md:grid-cols-2 md:p-8">
          <div className="self-center">
            <p className="text-sm font-medium text-neutral-500">Question decomposition</p>
            <h2 id="question-heading" className="mt-2 text-3xl font-semibold text-neutral-950 text-balance">A broad query becomes testable checks.</h2>
            <p className="mt-4 leading-7 text-neutral-600 text-pretty">The model creates 2–6 distinct yes-or-no questions. These questions define what useful evidence should contain and make the verification step easier to inspect.</p>
          </div>

          <div className="rounded-lg border border-neutral-200 bg-neutral-50 p-4 sm:p-5">
            <div className="rounded-lg border border-neutral-200 bg-white p-4 shadow-sm">
              <p className="text-xs font-medium text-neutral-400">RESEARCH QUERY</p>
              <p className="mt-2 text-sm font-medium leading-6 text-neutral-950 text-pretty">Does vitamin B12 deficiency increase homocysteine?</p>
            </div>
            <div aria-hidden="true" className="mx-auto h-5 w-px bg-neutral-300" />
            <div className="space-y-2">
              <div className="rounded-lg border border-neutral-200 bg-white px-4 py-3 text-sm text-neutral-700 shadow-sm"><span className="mr-3 font-mono text-xs text-neutral-400">Q1</span>Does the document discuss B12 deficiency?</div>
              <div className="rounded-lg border border-neutral-200 bg-white px-4 py-3 text-sm text-neutral-700 shadow-sm"><span className="mr-3 font-mono text-xs text-neutral-400">Q2</span>Does it measure homocysteine levels?</div>
              <div className="rounded-lg border border-neutral-200 bg-white px-4 py-3 text-sm text-neutral-700 shadow-sm"><span className="mr-3 font-mono text-xs text-neutral-400">Q3</span>Does it report an association?</div>
            </div>
          </div>
        </section>

        <section aria-labelledby="answer-heading" className="mt-16">
          <div className="grid gap-5 md:grid-cols-3">
            <div className="md:col-span-1">
              <p className="text-sm font-medium text-neutral-500">Grounded synthesis</p>
              <h2 id="answer-heading" className="mt-2 text-3xl font-semibold text-neutral-950 text-balance">The answer uses only selected content.</h2>
              <p className="mt-4 leading-7 text-neutral-600 text-pretty">The final model sees only the user query and selected document contents. It receives no verification questions, document IDs, titles, ranks, or scores.</p>
            </div>

            <div className="rounded-xl border border-neutral-200 bg-white p-6 shadow-sm md:col-span-2">
              <div className="flex items-center justify-between border-b border-neutral-200 pb-4">
                <span className="text-sm font-medium text-neutral-500">Final answer</span>
                <span className="flex items-center gap-2 text-xs text-neutral-500"><span className="size-2 rounded-full bg-emerald-600" />Grounded</span>
              </div>
              <p className="mt-5 leading-7 text-neutral-700 text-pretty">The evidence indicates that vitamin B12 deficiency is associated with elevated homocysteine concentrations, particularly when folate status is also considered. The strength of the relationship varies across populations.</p>
            </div>
          </div>
        </section>

        <section className="mt-16 rounded-xl bg-neutral-950 px-6 py-10 text-center text-white sm:px-10">
          <h2 className="text-3xl font-semibold text-balance">See the pipeline on your own question.</h2>
          <p className="mx-auto mt-3 max-w-xl leading-7 text-neutral-400 text-pretty">Every result exposes the retrieved documents, vector ranks, and Jev probabilities behind the answer.</p>
          <Link href="/" className="mt-6 inline-flex min-h-11 items-center justify-center rounded-lg bg-white px-5 py-2.5 text-sm font-medium text-neutral-950 hover:bg-neutral-200">Try a search</Link>
        </section>
      </div>
    </main>
  );
}
