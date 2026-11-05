---
name: local-doc-sync
description: Aktualisiere den lokalen Dokumentationsindex auf ausdrueckliche Anfrage und melde Dokumentzahl, Fehler und fehlende Quellen.
---

# Dokumentationsindex aktualisieren

Nutze diesen Skill, wenn der Nutzer den zentralen Dokumentationsindex aktualisieren oder neu einlesen lassen moechte. Normale Dokumentationsfragen gehoeren zum Skill `local-doc-search` und loesen hier keine Synchronisierung aus.

Das Werkzeug ist das Programm `local-docs` (ohne Installation im Repository `go run .`, siehe dessen `AGENTS.md`). Pruefe, ob `config.json` im Datenverzeichnis vorhanden ist (Standard `%LOCALAPPDATA%\local-docs`, ueberschreibbar mit `--root` oder `LOCAL_DOCS_HOME`); sonst `local-docs init` und den Nutzer die Quellen eintragen lassen. Fuehre `local-docs sync` aus. Veraendere weder die Quellordner noch deren Originaldateien.

Pruefe danach die Ausgabe und `manifest.json` im Datenverzeichnis (die Ausgabe nennt den Pfad): Anzahl der Dokumente, Eintraege mit `status: error` und Warnungen zu fehlenden Quellordnern. Melde diese Zahlen sowie den Zeitpunkt der Aktualisierung. Bei Fehlern nenne die betroffenen Originalpfade und die Ursache. Das gilt auch fuer `web_sources` (URLs): Warnungen wie "web source unavailable" (z. B. HTTP 401 wegen fehlender Zugangsdaten) melden, aber niemals Passwoerter aus `config.json` ausgeben. Ein nicht erreichbarer Quellordner bleibt im Manifest erhalten und darf nicht als leer interpretiert werden.

Die Synchronisierung prueft SHA-256 und extrahiert nur geaenderte Dokumente neu. Sie erstellt keinen Zeitplan.
