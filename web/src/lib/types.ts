export type Probability = {
  question: string;
  probability: number | null;
  error: string | null;
};

export type SearchDocument = {
  doc_id: string;
  title: string;
  text: string;
  rank: number;
  vector_score: number;
  query_probability: number | null;
  query_error: string | null;
  probabilities: Probability[];
};

export type SearchResponse = {
  query: string;
  questions: string[];
  documents: SearchDocument[];
  answer: string;
};
