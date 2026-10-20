---
name: local-doc-search
description: Durchsuche lokal indexierte Dokumente nach Fachfragen; liefere Originalpfade und PDF-Seiten.
---

# Local document search

Find this skill's real file location, following a personal skill-directory link if necessary. The documentation-search project root is three directories above this skill directory (`.agents/skills/local-doc-search`). Run the Go commands from that project root and follow its `AGENTS.md`.

Use `go run . search --query "phrase"` for focused research. Run `go run . sync` first only if the index is missing or the user requests current source content. Search output includes a document ID, version, original path, and PDF page number. Cite the original path and page in the answer. For visual inspection, run `go run . page --id ID --page N` and inspect the returned PNG. DOCX and ODT results have line references only.

The default search prefers `-nokey.pdf` within the same document family and version. Run `search --all` when fallback variants or differences between variants matter. Do not infer that similarly named documents from different versions contain the same content.

Original documents are read-only. Machine-specific source and tool paths live in the untracked `config.json`; generated index files live in `data/`.
