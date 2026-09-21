import { describe, expect, it } from "vitest";
import { formatProbability } from "./utils";

describe("formatProbability", () => {
  it("shows two decimal places", () => expect(formatProbability(0.756)).toBe("0.76"));
});
