# Evaluation

## Approach

The [14 evaluation cases](cases.md) contain seven Go and seven Python snippets. Each language has three single-bug, two correct, one ambiguous, and one multi-bug case.

- **Development set:** Six cases used to evaluate and refine the prompt.
- **Test set:** Eight separate cases, not used for prompt tuning, evaluated only after prompt v2 was finalized.

The Development set has three single-bug, two correct, and one ambiguous case. The Test set has the same mix plus two multi-bug cases with three independent defects each. Expected behavior, failing inputs, affected lines, and acceptable fixes were defined before model runs. Expectations were not changed to match responses, and only code and language were sent to the model.

Responses were checked manually for bug detection, useful line references, correct fixes, and unsupported findings. Proposed fixes were also checked locally on selected inputs. Backend validation and error-handling tests were checked separately.

The v2 evaluations used Groq with `openai/gpt-oss-120b`. The two multi-bug cases were manually rerun after their comments/docstrings were simplified, with the same reported outcomes.

## Development results and prompt change

Prompt v1 identified all three intended bugs, but `python-dev-01` produced an extra finding about empty-input handling. Its docstring explicitly requires a nonempty list, so that finding was unsupported. The actual bug was floor division: `[1, 2]` returned `1` instead of `1.5`.

Prompt v2 added:

> Respect explicit preconditions and input constraints stated in the snippet. Do not report failures that require violating those stated preconditions.

All six Development cases were rerun. The three intended bugs were still identified, the empty-input false positive disappeared, and the other five case outcomes were unchanged. No regression was observed against the manually recorded v1 outcomes.

## Test set results

The finalized v2 prompt was used for the original six Test cases and then the two additional multi-bug cases. All eight matched expectations, including the manual confirmation reruns of the simplified multi-bug snippets. The v2 prompt remained unchanged.

The reviewer found the premature return in `go-test-01`, the counter reset in `python-test-01`, and the incorrect OR operator in `python-test-02`. Both correct snippets and the ambiguous threshold case returned no findings.

For example, `Contains([]int{3, 8}, 8)` returned false because it stopped after the first nonmatch. The suggested fix searched the whole slice before returning false and passed checks for first/later matches, an absent target, and an empty slice.

The two additional cases tested multiple independent findings in one snippet, with expectations defined before either was run. The manual UI results are summarized below; line references match the current snippets in `cases.md`.

| Case           | Findings | Defects identified and affected lines                                                        |
| -------------- | -------: | -------------------------------------------------------------------------------------------- |
| go-test-04     |        3 | Quantity ignored (16–18), discount added (20–22), shipping charged at the threshold (23–25). |
| python-test-04 |        3 | Jobs due exactly now excluded (6), ascending priority order (8), limit exceeded (9).         |

All six suggested fixes matched the expected fixes, with no unsupported findings or request failures. These results were reported from manual UI checks; the earlier three Test-case fixes were also executed locally on selected inputs.

Multiple-finding display behavior had already been checked manually during development using fixture responses. The formal Development set focused on prompt refinement; these new multi-bug Test cases confirmed multiple independent findings with the finalized prompt.

## Metrics

Results are shown separately because only the Development set was used to improve the prompt. The metrics summarize the final outcome for each case; confirmation reruns do not increase the case or finding counts.

| Measure                                       | Development v2 | Test set v2 |
| --------------------------------------------- | -------------: | ----------: |
| Expected bugs identified                      |            3/3 |         9/9 |
| Findings with useful affected lines           |            3/3 |         9/9 |
| Suggested fixes matching expected fixes       |            3/3 |         9/9 |
| Correct/ambiguous cases returning no findings |            3/3 |         3/3 |
| Unsupported findings / total findings         |            0/3 |         0/9 |
| Cases with request failures / evaluated cases |            0/6 |         0/8 |

Development v1 identified 3/3 expected bugs, with 1 unsupported finding among 4 total findings. Its three correct/ambiguous cases also returned no findings.

## Limitations

- Small, manually selected snippets and one provider/model; no broad accuracy claim.
- Manual assessment and selected fix checks do not prove correctness for every input.
- Development results benefited from tuning. No findings does not guarantee correct code.
