# Decision interview discipline

## Provenance

Adapted from the `grill-me` wrapper and its `grilling` primitive in [`mattpocock/skills`](https://github.com/mattpocock/skills/tree/e9fcdf95b402d360f90f1db8d776d5dd450f9234/skills/productivity), at that exact revision: walk a design tree in dependency order, finish an opened decision branch before starting an independent one, ask one question at a time and wait, provide a recommended answer, investigate all available environment facts instead of asking for them, and do not act on changed understanding until the user explicitly confirms the shared understanding.

The link is pinned deliberately. Slipway follows that revision's rule that questions are asked one at a time because asking several at once is bewildering. Later upstream revisions ask a whole computed frontier per round instead; Slipway does not follow that change, because a Run's Clarify Action carries exactly one decision and a partial answer to a batch is not a legal Outcome. Read a difference from current upstream as a deliberate divergence, not as drift.

Slipway keeps two additions from later upstream revisions that do not conflict with one decision per response: a fixed question shape that puts the recommendation on its own line, and the rule that investigating an environment fact never blocks the decisions that do not depend on it. Slipway adds the immediate wrap-up rule and keeps this interview stateless.

The discipline itself lives in the generated capability text, so the capability and this reference cannot drift apart.

## Attribution

MIT License

Copyright (c) 2026 Matt Pocock

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
