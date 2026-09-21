import Link from "next/link";

export function SiteHeader() {
  return (
    <header className="border-b border-neutral-200 bg-white">
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-6 px-4 py-4 sm:px-6">
        <Link href="/" className="flex items-center gap-3 rounded-md">
          <span aria-hidden="true" className="flex size-8 items-center justify-center rounded-md bg-neutral-950 text-sm font-semibold text-white">J</span>
          <span className="font-semibold text-neutral-950">Jev Retrieval</span>
        </Link>
        <nav aria-label="Main navigation">
          <Link href="/how-it-works" className="text-sm font-medium text-neutral-600 hover:text-neutral-950">How it works</Link>
        </nav>
      </div>
    </header>
  );
}
