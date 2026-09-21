import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function documentAnchorId(docId: string) {
  return `document-${encodeURIComponent(docId)}`;
}

export function formatProbability(probability: number) {
  return probability.toFixed(2);
}
