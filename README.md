# Zentrale Dokumentationssuche

Durchsucht verteilte Dokumentation (PDF, DOCX, ODT, MD, TXT) aus mehreren Quellordnern in einem Schritt und nennt zu jedem Treffer Originalpfad, Version und Seite bzw. Zeile. Windows-Kommandozeile in Go. Die Originaldateien verbleiben in den Quellordnern. Der lokale Index liegt unter `data/` und wird vom Programm verwaltet.

## Was das Tool macht

- `sync` liest die konfigurierten Quellordner und extrahiert den Text in einen lokalen Index unter `data/`. PDFs werden seitenweise abgelegt.
- `search` durchsucht diesen Text mit ripgrep. Jeder Treffer nennt ID, Version, Seite bzw. Zeile und den Pfad der Originaldatei.
- `page` rendert eine PDF-Seite als PNG, um einen Treffer im Original zu pruefen (Tabellen, Abbildungen, Layout).
- Drei Skills (`local-doc-search`, `local-doc-sync`, `local-doc-update`) machen die Suche aus Codex und Claude nutzbar.

## Vorteile

- **Eine Suche statt vieler Ordner:** mehrere Quellordner und Formate werden gemeinsam durchsucht.
- **Nachvollziehbar:** Originalpfad, Version und PDF-Seite sind Teil jedes Treffers und lassen sich zitieren und gegenpruefen.
- **Versionsbewusst:** Dokumentfamilie und Version werden aus Datei- und Ordnernamen erkannt. `-nokey.pdf` erscheint zuerst, `--all` zeigt alle Varianten.
- **Schnell und inkrementell:** gesucht wird in lokalem Text; `sync` verarbeitet nur Dateien, deren SHA-256 sich geaendert hat.
- **Sicher:** Originale werden nur gelesen. Der Index ist abgeleitet und jederzeit neu aufbaubar. Voruebergehend fehlende Quellen bleiben im Manifest erhalten.
- **Lokal:** keine Cloud, keine Datenweitergabe.

## Nutzen fuer KI-Agenten

- **Aufbereiteter Klartext statt Binaerformat:** PDF, DOCX und ODT lassen sich von Agenten nicht effizient lesen. Der Index stellt sie als durchsuchbaren Text bereit, bei PDFs pro Seite.
- **Gezielter Zugriff:** Ein Agent bekommt nur die Treffer, statt ganze Handbuecher in den Kontext zu laden. Das spart Kontext und Zeit.
- **Belegbare Antworten:** Pfad, Version und Seite bzw. Zeile ermoeglichen Quellenangaben, die Menschen pruefen koennen.
- **Visuelle Gegenpruefung:** `page` liefert die Originalseite als Bild, wenn der Text allein nicht reicht.
- **Verlaessliche Nutzung:** Die tabellarische Ausgabe und die mitgelieferten Skills geben Agenten einen festen Ablauf. Verschiedene Versionen eines Dokuments werden dabei nicht gleichgesetzt.

Grenzen: Fuer bildbasierte PDFs gibt es keine OCR, sie liefern keinen Text. DOCX, ODT, MD, TXT und Webseiten haben nur Zeilen-, keine Seitenreferenzen. Details stehen in den Abschnitten unten.

## Einrichtung

### Mit fertigem Programm (ohne Go)

1. Von den GitHub-Releases das Archiv `local-docs_<os>_<arch>.zip` laden, entpacken und `local-docs install` ausfuehren. Das kopiert das Programm nach `%LOCALAPPDATA%\Programs\local-docs` (Linux/macOS: `~/.local/bin`), traegt es in den User-PATH ein (`--no-path` verhindert das), legt die Skills unter `~/.claude/skills` und `~/.codex/skills` an (vorhandene abweichende Skills bleiben unberuehrt, `--force` sichert und ersetzt sie) und prueft die externen Werkzeuge.
2. ripgrep (`rg`) und Poppler (`pdfinfo`, `pdftotext`, `pdftoppm`) muessen weiterhin installiert sein. `soffice.com` ist nur fuer alte `.doc`-Dateien erforderlich.
3. `config.json` anpassen: `local-docs config` legt sie bei Bedarf an, gibt den Pfad aus und oeffnet sie im Editor (`VISUAL`/`EDITOR`, sonst Standardanwendung des Systems). `local-docs config --path` gibt nur den Pfad aus.

Das Datenverzeichnis (`config.json`, `data/`) wird in dieser Reihenfolge bestimmt: `--root DIR`, Umgebungsvariable `LOCAL_DOCS_HOME`, das Arbeitsverzeichnis, falls dort eine `config.json` liegt (bisheriges Verhalten), sonst `%LOCALAPPDATA%\local-docs` (Linux/macOS: Benutzer-Konfigurationsverzeichnis). Mit `"data_dir"` in `config.json` laesst sich nur der Index (`data/`) an einen anderen Ort legen; relative Pfade gelten ab dem Datenverzeichnis.

```powershell
local-docs sync
local-docs search --query "Zeiterfassung"
local-docs search --query "Zeiterfassung" --all
local-docs page --id <ID-aus-Treffer> --page 12
local-docs version
```

### Aktualisieren

`local-docs update --check` meldet, ob ein neueres Release existiert; `local-docs update` laedt es, prueft die SHA-256-Summe aus `SHA256SUMS`, ersetzt das Programm (die alte Datei bleibt bis zum naechsten Start als `.old`) und aktualisiert die bereits installierten Skills. Nach `sync`, `search` und `page` erscheint hoechstens einmal taeglich ein Hinweis auf stderr, wenn eine neue Version vorliegt; mit `"check_updates": false` in `config.json` abschaltbar. Entwicklungsbuilds (`dev`) pruefen und ersetzen sich nicht. Der Skill `local-doc-update` fuehrt das auf ausdrueckliche Anfrage fuer Agenten aus.

### Aus dem Quellcode

Go 1.23 oder neuer wird nur zum Bauen benoetigt: `go run . search --query "..."` im Projektordner oder `go build -o local-docs.exe .` fuer eine eigenstaendige EXE. Releases entstehen automatisch, wenn ein Tag `v*` gepusht wird (`.github/workflows/release.yml`: Tests, Builds fuer Windows, Linux und macOS, `SHA256SUMS`).

`sync` erkennt PDF, DOCX, ODT sowie Markdown- und Textdateien (MD, TXT; nur Zeilenreferenzen); alte DOC-Dateien nur bei `enable_legacy_doc: true`. Die DOC-Konvertierung erfolgt in einem temporaeren Verzeichnis. Originale werden nur gelesen. Fuer bildbasierte PDFs ist keine OCR enthalten. Gesperrte PDFs und Extraktionsfehler stehen mit Fehlermeldung im Manifest.

### Webseiten als Quelle

Zusaetzlich zu Ordnern kann eine URL in `web_sources` eingetragen werden. `sync` ruft die Seite ab, folgt Links rekursiv und indexiert den Text jeder Seite. Es werden nur Seiten unterhalb des angegebenen URL-Pfads mit gleichem Host besucht; Links nach aussen und Weiterleitungen aus diesem Bereich heraus werden ignoriert.

```json
"web_sources": [
  { "url": "https://kb.example.com/docs/", "username": "", "password": "", "max_pages": 500 }
]
```

`username` und `password` sind optional und werden als HTTP Basic Auth (z. B. `.htaccess`) nur an den konfigurierten Host gesendet. Sie stehen in der lokalen, nicht versionierten `config.json`. `max_pages` (Standard 500) begrenzt den Umfang. Es werden nur HTML- und Textseiten bis 5 MiB gelesen; `robots.txt` wird nicht ausgewertet, daher nur eigene oder freigegebene Seiten eintragen. Treffer zeigen die URL statt eines Dateipfads und nur eine Textzeile; `page` ist fuer Webseiten nicht verfuegbar. Ist eine Webquelle nicht erreichbar oder die Anmeldung schlaegt fehl, bleiben ihre bisherigen Eintraege erhalten; bei erfolgreichem Abruf verschwinden nicht mehr verlinkte Seiten aus dem Index. Der Index wird nicht automatisch aktualisiert, sondern bei jedem `sync`.

`search` durchsucht extrahierten UTF-8-Text mit `rg`. Standardmaessig erscheinen Treffer bevorzugter Varianten zuerst; `--all` zeigt auch alle Fallback-Varianten. Die Ausgabe ist tabellarisch mit ID, Version, Seite/Zeile und Originalpfad. DOCX und ODT haben keine zuverlaessige Seitenzuordnung; dort ist nur eine Textzeile angegeben. `page` erzeugt eine PNG-Datei in `data/views/` und gibt ihren Pfad aus.

`data/manifest.json` enthaelt SHA-256, Originalpfad, Version, Familie, Format, Seitenzahl, Status und relative Indexdateien. Groesse und Aenderungszeit werden nur als schneller Vorfilter verwendet; bei der Synchronisierung wird der SHA-256 der Quelldatei geprueft. Fehlende Quellen werden nicht als geloescht behandelt. Wird eine erreichbare Quelle erneut vollstaendig gescannt, werden nicht mehr vorhandene Eintraege entfernt. Suchindex und Manifest sind abgeleitete Daten; `data/` kann bei Bedarf neu aufgebaut werden.

## Sicherheit und Grenzen

Das Programm schreibt nur in `data/` (bzw. `data_dir`) und bei `init`/`install` eine neue `config.json`. `install` und `update` aendern zusaetzlich die eigene Programmdatei, den User-PATH und die Skill-Ordner. Es verweigert eine Konfiguration, bei der eine Quelle innerhalb des Projektordners oder der Projektordner innerhalb einer Quelle liegt. Dateien mit gleichem Familiennamen und gleicher Version bleiben getrennte Manifest-Eintraege. `-nokey.pdf` hat nur bei dieser Paarung Vorrang. `--all` ist fuer vollstaendige Recherche sinnvoll.

## Repository und Skills

Die Skills liegen als einzige bearbeitbare Fassung unter `.agents/skills/` und werden ins Programm eingebettet. `local-docs install` schreibt genau die zur Programmversion passende Fassung in die persoenlichen Skill-Ordner, `local-docs update` frischt sie auf. Codex erkennt sie zusaetzlich automatisch, wenn es in diesem Repository gestartet wird; dann koennen beide Fundorte im Skill-Selektor erscheinen. Vorgaenger-Verknuepfungen aus `scripts/install-skills.ps1` werden von `install --force` gesichert (unter `skill-backups/` im Datenverzeichnis) und ersetzt.

Fuer ein neues Checkout `local-docs init` (oder `config.example.json` nach `config.json` kopieren) ausfuehren und die Quellordner sowie Werkzeugpfade anpassen. Die Beispielkonfiguration enthaelt keine benutzerspezifischen Werkzeugpfade. `config.json`, `data/`, lokale EXE-Dateien und VS-Code-Einstellungen werden nicht versioniert. Der Quellcode und die Repository-Skills stehen unter GPLv3. Lokal eingebundene Originaldokumente und generierte Indexdaten sind nicht Teil des Repositories.
