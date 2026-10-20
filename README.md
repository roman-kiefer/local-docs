# Zentrale Dokumentationssuche

Windows-Kommandozeile in Go. Die Originaldateien verbleiben in den Quellordnern. Der lokale Index liegt unter `data/` und wird vom Programm verwaltet.

## Einrichtung

1. Go 1.23 oder neuer, ripgrep (`rg`) und Poppler (`pdfinfo`, `pdftotext`, `pdftoppm`) installieren. `soffice.com` ist nur fuer alte `.doc`-Dateien erforderlich.
2. `config.example.json` als `config.json` kopieren. Quellordner und Werkzeugpfade an die eigene Maschine anpassen. Weitere Quellordner koennen in `sources` ergaenzt werden. `poppler_bin` und `pdftotext` duerfen leer sein, wenn die Werkzeuge im `PATH` liegen; `pdftotext` kann auch separat konfiguriert werden.
3. Im Projektordner ausfuehren:

```powershell
go run . sync
go run . search --query "Zeiterfassung"
go run . search --query "Zeiterfassung" --all
go run . page --id <ID-aus-Treffer> --page 12
```

`go build -o local-docs.exe .` erstellt eine eigenstaendige EXE. `local-docs.exe` muss im Projektordner gestartet werden oder mit `--root <Pfad-zum-Checkout>` aufgerufen werden.

`sync` erkennt PDF, DOCX und ODT; alte DOC-Dateien nur bei `enable_legacy_doc: true`. Die DOC-Konvertierung erfolgt in einem temporaeren Verzeichnis. Originale werden nur gelesen. Fuer bildbasierte PDFs ist keine OCR enthalten. Gesperrte PDFs und Extraktionsfehler stehen mit Fehlermeldung im Manifest.

`search` durchsucht extrahierten UTF-8-Text mit `rg`. Standardmaessig erscheinen Treffer bevorzugter Varianten zuerst; `--all` zeigt auch alle Fallback-Varianten. Die Ausgabe ist tabellarisch mit ID, Version, Seite/Zeile und Originalpfad. DOCX und ODT haben keine zuverlaessige Seitenzuordnung; dort ist nur eine Textzeile angegeben. `page` erzeugt eine PNG-Datei in `data/views/` und gibt ihren Pfad aus.

`data/manifest.json` enthaelt SHA-256, Originalpfad, Version, Familie, Format, Seitenzahl, Status und relative Indexdateien. Groesse und Aenderungszeit werden nur als schneller Vorfilter verwendet; bei der Synchronisierung wird der SHA-256 der Quelldatei geprueft. Fehlende Quellen werden nicht als geloescht behandelt. Wird eine erreichbare Quelle erneut vollstaendig gescannt, werden nicht mehr vorhandene Eintraege entfernt. Suchindex und Manifest sind abgeleitete Daten; `data/` kann bei Bedarf neu aufgebaut werden.

## Sicherheit und Grenzen

Das Programm schreibt nur in `data/` und bei `init` eine neue `config.json`. Es verweigert eine Konfiguration, bei der eine Quelle innerhalb des Projektordners oder der Projektordner innerhalb einer Quelle liegt. Dateien mit gleichem Familiennamen und gleicher Version bleiben getrennte Manifest-Eintraege. `-nokey.pdf` hat nur bei dieser Paarung Vorrang. `--all` ist fuer vollstaendige Recherche sinnvoll.

## Repository und Skills

Die beiden Skills liegen als einzige bearbeitbare Fassung unter `.agents/skills/local-doc-search/` und `.agents/skills/local-doc-sync/`. Codex erkennt sie automatisch, wenn es in diesem Repository gestartet wird. Fuer die Nutzung aus anderen Projektordnern erzeugt `scripts/install-skills.ps1` persoenliche Verknuepfungen auf diese Ordner:

```powershell
./scripts/install-skills.ps1
```

Bei der ersten Umstellung von den zuvor einzeln installierten Skills `./scripts/install-skills.ps1 -MigrateExisting` ausfuehren. Vorhandene Skill-Ordner werden dabei unter `data/skill-backups/` gesichert; fremde Skills bleiben unberuehrt. Die Verknuepfungen zeigen immer auf die aktuelle Fassung im Repository. Wenn Codex im Repository und zugleich ueber die persoenlichen Verknuepfungen laeuft, koennen beide Fundorte im Skill-Selektor erscheinen.

Fuer ein neues Checkout `config.example.json` nach `config.json` kopieren und die Quellordner sowie Werkzeugpfade anpassen. Die Beispielkonfiguration enthaelt keine benutzerspezifischen Werkzeugpfade. `config.json`, `data/`, lokale EXE-Dateien und VS-Code-Einstellungen werden nicht versioniert. Das Repository selbst wird vom Benutzer angelegt; dieser Ordner wurde nicht mit `git init` initialisiert.
