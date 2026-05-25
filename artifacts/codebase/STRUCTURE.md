# Structure

- Directory layout: cmd/, docs/, internal/, skills_ref/
- Entry points: CHANGELOG.md, CLAUDE.md, README.md, go.mod, main.go
- Generated versus handwritten boundaries: internal/tmpl contains generated prompt/skill surfaces; cmd/ and internal/ contain handwritten Go runtime code.
- Ownership hints: Tests are colocated as *_test.go files under cmd/ and internal/.
- Notes: Go *_test.go files are present.
