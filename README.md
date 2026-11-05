# Zentrale Dokumentationssuche

Durchsucht verteilte Dokumentation (PDF, DOCX, ODT, MD, TXT) aus mehreren Quellordnern in einem Schritt und nennt zu jedem Treffer Originalpfad, Version und Seite bzw. Zeile. Windows-Kommandozeile in Go. Die Originaldateien verbleiben in den Quellordnern. Der lokale Index liegt unter `data/` und wird vom Programm verwaltet.

## Was das Tool macht

- `sync` liest die konfigurierten Quellordner und extrahiert den Text in einen lokalen Index unter `data/`. PDFs werden seitenweise abgelegt.
- `search` durchsucht diesen Text mit ripgrep. Jeder Treffer nennt ID, Version, Seite bzw. Zeile und den Pfad der Originaldatei.
- `page` rendert eine PDF-Seite als PNG, um einen Treffer im Original zu pruefen (Tabellen, Abbildungen, Layout).
- Zwei Skills (`local-doc-search`, `local-doc-sync`) machen die Suche aus Codex und Claude nutzbar.

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

Das Programm schreibt nur in `data/` und bei `init` eine neue `config.json`. Es verweigert eine Konfiguration, bei der eine Quelle innerhalb des Projektordners oder der Projektordner innerhalb einer Quelle liegt. Dateien mit gleichem Familiennamen und gleicher Version bleiben getrennte Manifest-Eintraege. `-nokey.pdf` hat nur bei dieser Paarung Vorrang. `--all` ist fuer vollstaendige Recherche sinnvoll.

## Repository und Skills

Die beiden Skills liegen als einzige bearbeitbare Fassung unter `.agents/skills/local-doc-search/` und `.agents/skills/local-doc-sync/`. Codex erkennt sie automatisch, wenn es in diesem Repository gestartet wird. Fuer die Nutzung aus anderen Projektordnern erzeugt `scripts/install-skills.ps1` persoenliche Verknuepfungen auf diese Ordner:

```powershell
./scripts/install-skills.ps1
```

Bei der ersten Umstellung von den zuvor einzeln installierten Skills `./scripts/install-skills.ps1 -MigrateExisting` ausfuehren. Vorhandene Skill-Ordner werden dabei unter `data/skill-backups/` gesichert; fremde Skills bleiben unberuehrt. Die Verknuepfungen zeigen immer auf die aktuelle Fassung im Repository. Wenn Codex im Repository und zugleich ueber die persoenlichen Verknuepfungen laeuft, koennen beide Fundorte im Skill-Selektor erscheinen.

Fuer ein neues Checkout `config.example.json` nach `config.json` kopieren und die Quellordner sowie Werkzeugpfade anpassen. Die Beispielkonfiguration enthaelt keine benutzerspezifischen Werkzeugpfade. `config.json`, `data/`, lokale EXE-Dateien und VS-Code-Einstellungen werden nicht versioniert. Der Quellcode und die Repository-Skills stehen unter GPLv3. Lokal eingebundene Originaldokumente und generierte Indexdaten sind nicht Teil des Repositories.
