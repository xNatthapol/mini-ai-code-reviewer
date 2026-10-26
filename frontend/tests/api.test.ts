import assert from "node:assert/strict";
import test from "node:test";
import { requestReview } from "../src/api.ts";

test("sends code and language to the relative endpoint and accepts no findings", async (t) => {
  const fetchMock = t.mock.method(
    globalThis,
    "fetch",
    async (url: string, options: RequestInit) => {
      assert.equal(url, "/lint");
      assert.equal(options.method, "POST");
      assert.equal(
        new Headers(options.headers).get("Content-Type"),
        "application/json",
      );
      assert.deepEqual(JSON.parse(options.body as string), {
        code: "x",
        language: "go",
      });
      return Response.json({ findings: [] });
    },
  );
  assert.deepEqual(await requestReview("x", "go"), { findings: [] });
  assert.equal(fetchMock.mock.callCount(), 1);
});

test("shows the API error message on failure", async (t) => {
  t.mock.method(globalThis, "fetch", async () =>
    Response.json(
      {
        error: {
          code: "review_timeout",
          message: "The review timed out. Please try again.",
        },
      },
      { status: 504 },
    ),
  );
  await assert.rejects(requestReview("x", "python"), /review timed out/);
});

test("handles a proxy error that is not JSON", async (t) => {
  t.mock.method(
    globalThis,
    "fetch",
    async () => new Response("Bad gateway", { status: 502 }),
  );
  await assert.rejects(requestReview("x", "go"), /could not be completed/);
});

test("does not turn malformed successful responses into empty findings", async (t) => {
  t.mock.method(globalThis, "fetch", async () =>
    Response.json({ findings: null }),
  );
  await assert.rejects(requestReview("x", "go"), /invalid response/);
});

test("explains connection failure", async (t) => {
  t.mock.method(globalThis, "fetch", async () => {
    throw new TypeError("Failed to fetch");
  });
  await assert.rejects(requestReview("x", "go"), /connect/);
});
