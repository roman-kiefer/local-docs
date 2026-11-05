# Dokumentationssuche

- Originaldokumente in den konfigurierten Quellordnern ausschliesslich lesen. Nie verschieben, umbenennen oder veraendern.
- Vor der Nutzung `local-docs init` ausfuehren (oder `config.example.json` nach `config.json` kopieren) und Quellordner sowie Werkzeugpfade pruefen. Datenverzeichnis: `--root`, `LOCAL_DOCS_HOME`, Arbeitsverzeichnis mit `config.json`, sonst `%LOCALAPPDATA%\local-docs`.
- `local-docs config --path` nennt den Pfad der `config.json`; `local-docs config` oeffnet sie im Editor.
- Suche mit `local-docs search --query "Begriff"` (ohne installiertes Programm `go run . search ...`); die Ausgabe nennt Originalpfad, Version und PDF-Seite.
- Fuer eine PDF-Seite `local-docs page --id <ID> --page <N>` verwenden. Die ID steht in den Suchtreffern.
- `local-docs sync` aktualisiert nur den von diesem Projekt verwalteten Index. Quellordner, die voruebergehend fehlen, bleiben im Manifest erhalten.
- Webseiten koennen als `web_sources` in `config.json` eingetragen werden (rekursiv, nur innerhalb der URL, optional Basic Auth). Treffer nennen die URL und eine Zeile; Zugangsdaten nie ausgeben oder committen.
- Bei gleichen Dokumentfamilien und Versionen werden `-nokey.pdf`-Dateien zuerst angezeigt; andere Varianten bleiben suchbar.

## Repository-Skills

- Die Skill-Quelldateien liegen unter `.agents/skills/`. Aenderungen dort vornehmen; sie werden ins Programm eingebettet und per `local-docs install` bzw. `update` in die persoenlichen Skill-Ordner geschrieben. Releases entstehen durch Tags `v*` (GitHub Action); Updates erfolgen nur auf Anfrage (`local-docs update`, Skill `local-doc-update`).
- `config.json` und `data/` (bzw. `data_dir`) enthalten lokale Pfade bzw. generierte Daten und gehoeren nicht in Git.
