import assert from "node:assert/strict";
import test from "node:test";
import { normalizeCode, inputError, affectedCode } from "../src/review.ts";

test("normalization preserves indentation, blank lines, and trailing newline", () => {
  assert.equal(normalizeCode("  x\r\n\r\ny\r\n"), "  x\n\ny\n");
});

test("input limits match backend line and UTF-8 byte limits", () => {
  assert.ok(inputError(" \t\n"));
  assert.equal(inputError("x\n".repeat(199)), null);
  assert.ok(inputError("x\n".repeat(200)));
  assert.equal(inputError("é".repeat(6144)), null);
  assert.ok(inputError("é".repeat(6145)));
  assert.equal(inputError("x".repeat(12287) + "\r\n"), null);
});

test("affected code uses inclusive 1-based lines, including blank lines", () => {
  assert.equal(affectedCode("first\n\n  third\nlast", 2, 3), "\n  third");
  assert.equal(affectedCode("first\n", 2, 2), "");
});
