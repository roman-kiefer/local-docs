---
name: local-doc-search
description: Durchsuche lokal indexierte Dokumente nach Fachfragen; liefere Originalpfade und PDF-Seiten.
---

# Local document search

Das Werkzeug ist das Programm `local-docs` (im PATH nach `local-docs install`). Ist es nicht installiert, im Repository `go run .` an Stelle von `local-docs` verwenden und dessen `AGENTS.md` beachten. Konfiguration und Index liegen im Datenverzeichnis des Programms (Standard: `%LOCALAPPDATA%\local-docs`, ueberschreibbar mit `--root` oder `LOCAL_DOCS_HOME`).

Use `local-docs search --query "phrase"` for focused research. Run `local-docs sync` first only if the index is missing or the user requests current source content. Search output includes a document ID, version, original path, and PDF page number. Cite the original path and page in the answer. For visual inspection, run `local-docs page --id ID --page N` and inspect the returned PNG. DOCX, ODT, Markdown, text and web page results have line references only; for web pages the path column is the page URL (cite the URL) and `page` is unavailable.

The default search prefers `-nokey.pdf` within the same document family and version. Run `search --all` when fallback variants or differences between variants matter. Do not infer that similarly named documents from different versions contain the same content.

Original documents are read-only. Machine-specific source and tool paths live in the untracked `config.json`; generated index files live in the data directory.

Erscheint auf stderr der Hinweis auf eine neue Programmversion, nenne ihn dem Nutzer kurz; aktualisiere nicht selbst (dafuer gibt es den Skill `local-doc-update`).
