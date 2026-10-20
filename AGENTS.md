# Dokumentationssuche

- Originaldokumente in den konfigurierten Quellordnern ausschliesslich lesen. Nie verschieben, umbenennen oder veraendern.
- Vor der Nutzung `config.example.json` nach `config.json` kopieren und Quellordner sowie Werkzeugpfade pruefen.
- Suche mit `go run . search --query "Begriff"`; die Ausgabe nennt Originalpfad, Version und PDF-Seite.
- Fuer eine PDF-Seite `go run . page --id <ID> --page <N>` verwenden. Die ID steht in den Suchtreffern.
- `go run . sync` aktualisiert nur den von diesem Projekt verwalteten Index. Quellordner, die voruebergehend fehlen, bleiben im Manifest erhalten.
- Bei gleichen Dokumentfamilien und Versionen werden `-nokey.pdf`-Dateien zuerst angezeigt; andere Varianten bleiben suchbar.

## Repository-Skills

- Die Skill-Quelldateien liegen unter `.agents/skills/`. Aenderungen dort vornehmen; persoenliche Skill-Verknuepfungen sind nur Installationen.
- `config.json` und `data/` enthalten lokale Pfade bzw. generierte Daten und gehoeren nicht in Git.
