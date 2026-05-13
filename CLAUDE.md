# Project conventions for Claude

## Punctuation: no em or en dashes

Never use em (`—`, U+2014) or en (`–`, U+2013) dashes anywhere in this repo. That includes:

- Source code, comments, and docstrings
- Markdown (README, CHANGELOG, docs, examples, plans)
- YAML manifests, CRD descriptions, OLM CSV fields
- Commit messages and PR descriptions
- The GitHub repository description, About panel, and release notes

Use only `.` `,` `;` `:` as punctuation connectors where a dash would otherwise appear. The ASCII hyphen-minus (`-`) is fine for ranges (`1.28-1.32`) and compound identifiers (`hermes-operator`); the rule applies to fancy dashes only.

If a third party tool inserts an em or en dash (release-please changelog, Dependabot title, etc.), rewrite it before merging.
