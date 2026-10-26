export type Language = "go" | "python";
export interface Finding {
  start_line: number;
  end_line: number;
  explanation: string;
  suggested_fix: string;
}
export interface ReviewResult {
  findings: Finding[];
}

export async function requestReview(
  code: string,
  language: Language,
): Promise<ReviewResult> {
  let response: Response;
  try {
    response = await fetch("/lint", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code, language }),
    });
  } catch {
    throw new Error("Could not connect. Check your connection and try again.");
  }
  const data = await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(
      typeof data?.error?.message === "string"
        ? data.error.message
        : "The review could not be completed. Please try again.",
    );
  }
  if (!Array.isArray(data?.findings))
    throw new Error(
      "The server returned an invalid response. Please try again.",
    );
  return data as ReviewResult;
}
