import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Jev Retrieval",
  description: "Retrieve documents and inspect their Jev probabilities.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
