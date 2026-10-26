import { useCallback, useRef, useState } from "react";
import type { SubmitEvent } from "react";
import CodeEditor from "./CodeEditor";
import { requestReview } from "./api";
import type { Language, ReviewResult } from "./api";
import {
  affectedCode,
  inputError,
  MAX_BYTES,
  MAX_LINES,
  normalizeCode,
} from "./review";

interface CompletedReview {
  code: string;
  language: Language;
  result: ReviewResult;
}

export default function App() {
  const [code, setCode] = useState("");
  const [language, setLanguage] = useState<Language>("go");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [completed, setCompleted] = useState<CompletedReview | null>(null);
  const inFlight = useRef(false);
  const normalized = normalizeCode(code);
  const lines = normalized.split("\n").length;
  const bytes = new TextEncoder().encode(normalized).length;
  const validationError = inputError(code);

  const changeCode = useCallback((next: string) => {
    setCode(next);
    setCompleted(null);
    setError(null);
  }, []);

  async function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (inFlight.current || validationError) return;
    inFlight.current = true;
    setLoading(true);
    setCompleted(null);
    setError(null);
    const snapshot = { code: normalized, language };
    try {
      const result = await requestReview(snapshot.code, snapshot.language);
      setCompleted({ ...snapshot, result });
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The review failed. Please try again.",
      );
    } finally {
      inFlight.current = false;
      setLoading(false);
    }
  }

  return (
    <main className="app">
      <header className="page-header">
        <div className="brand-mark" aria-hidden="true">
          {"{ }"}
        </div>
        <div>
          <h1>Mini-AI Code Reviewer</h1>
          <p>Find logic errors in a small code snippet.</p>
        </div>
      </header>

      <div className="workspace">
        <form className="input-panel" onSubmit={submit}>
          <div className="panel-heading">
            <h2>Your code</h2>
            <div className="language-control">
              <label htmlFor="language">Language</label>
              <select
                id="language"
                value={language}
                disabled={loading}
                onChange={(event) => {
                  setLanguage(event.target.value as Language);
                  setCompleted(null);
                  setError(null);
                }}
              >
                <option value="go">Go</option>
                <option value="python">Python</option>
              </select>
            </div>
          </div>
          <p id="input-hint" className="input-hint">
            Paste a self-contained snippet. Up to 200 lines and 12 KB.
          </p>
          <CodeEditor
            code={code}
            language={language}
            disabled={loading}
            onChange={changeCode}
          />
          <div className="editor-footer">
            <span
              className={
                lines > MAX_LINES || bytes > MAX_BYTES ? "limit-exceeded" : ""
              }
            >
              {lines} / {MAX_LINES} lines <span aria-hidden="true">·</span>{" "}
              {bytes.toLocaleString()} / {MAX_BYTES.toLocaleString()} bytes
            </span>
          </div>
          <div className="submit-row">
            <p className="input-message" aria-live="polite">
              {validationError ?? "Ready to review."}
            </p>
            <button type="submit" disabled={loading || !!validationError}>
              {loading ? "Reviewing…" : "Review code"}
            </button>
          </div>
        </form>

        <section
          className="results-panel"
          aria-labelledby="results-heading"
          aria-busy={loading}
        >
          <div className="panel-heading">
            <h2 id="results-heading">Review results</h2>
            {completed && (
              <span className="result-count">
                {completed.result.findings.length}{" "}
                {completed.result.findings.length === 1
                  ? "finding"
                  : "findings"}
              </span>
            )}
          </div>
          {loading && (
            <div className="empty-state" role="status">
              <span className="loading-dot" aria-hidden="true" />
              <h3>Reviewing your code…</h3>
              <p>Checking for concrete logic errors.</p>
            </div>
          )}
          {error && (
            <div className="error-state" role="alert">
              <h3>Review couldn’t complete</h3>
              <p>{error}</p>
              <p>Your code is still here. Use “Review code” to retry.</p>
            </div>
          )}
          {!loading && !error && !completed && (
            <div className="empty-state">
              <span className="empty-symbol" aria-hidden="true">
                {"[ ]"}
              </span>
              <h3>A second look at your logic</h3>
              <p>
                Paste your code and select its language. Findings will include
                affected lines, an explanation, and a suggested fix.
              </p>
            </div>
          )}
          {completed && completed.result.findings.length === 0 && (
            <div className="empty-state success-state" role="status">
              <span className="empty-symbol" aria-hidden="true">
                ✓
              </span>
              <h3>No logic errors identified</h3>
              <p>
                This does not guarantee the code is correct. Check its behavior
                with your own tests.
              </p>
            </div>
          )}
          {completed && completed.result.findings.length > 0 && (
            <>
              <p className="results-note" role="status">
                {completed.result.findings.length}{" "}
                {completed.result.findings.length === 1
                  ? "finding"
                  : "findings"}{" "}
                in your {completed.language === "go" ? "Go" : "Python"} snippet.
                Verify each suggestion before using it.
              </p>
              <div className="findings">
                {completed.result.findings.map((finding, index) => (
                  <article className="finding" key={index}>
                    <h3>
                      {finding.start_line === finding.end_line
                        ? `Line ${finding.start_line}`
                        : `Lines ${finding.start_line}–${finding.end_line}`}
                    </h3>
                    <h4>Original code</h4>
                    <pre>
                      <code>
                        {affectedCode(
                          completed.code,
                          finding.start_line,
                          finding.end_line,
                        )}
                      </code>
                    </pre>
                    <p className="explanation">{finding.explanation}</p>
                    <h4>Suggested fix</h4>
                    <pre className="suggested-code">
                      <code>{finding.suggested_fix}</code>
                    </pre>
                  </article>
                ))}
              </div>
            </>
          )}
        </section>
      </div>
      <footer className="page-footer">
        Logic errors only. Suggestions are independent and are not applied
        automatically.
      </footer>
    </main>
  );
}
