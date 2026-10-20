---
name: local-doc-sync
description: Aktualisiere den lokalen Dokumentationsindex auf ausdrueckliche Anfrage und melde Dokumentzahl, Fehler und fehlende Quellen.
---

# Dokumentationsindex aktualisieren

Nutze diesen Skill, wenn der Nutzer den zentralen Dokumentationsindex aktualisieren oder neu einlesen lassen moechte. Normale Dokumentationsfragen gehoeren zum Skill `local-doc-search` und loesen hier keine Synchronisierung aus.

Finde den tatsaechlichen Speicherort dieser `SKILL.md`, auch wenn der persoenliche Skill-Ordner eine Verknuepfung ist. Das Projektverzeichnis liegt drei Verzeichnisse oberhalb dieses Skill-Ordners (`.agents/skills/local-doc-sync`). Lies dort `AGENTS.md` und pruefe, ob `config.json` vorhanden ist. Fuehre im Projektverzeichnis `go run . sync` aus. Veraendere weder die Quellordner noch deren Originaldateien.

Pruefe danach die Ausgabe und `data/manifest.json`: Anzahl der Dokumente, Eintraege mit `status: error` und Warnungen zu fehlenden Quellordnern. Melde diese Zahlen sowie den Zeitpunkt der Aktualisierung. Bei Fehlern nenne die betroffenen Originalpfade und die Ursache. Ein nicht erreichbarer Quellordner bleibt im Manifest erhalten und darf nicht als leer interpretiert werden.

Die Synchronisierung prueft SHA-256 und extrahiert nur geaenderte Dokumente neu. Sie erstellt keinen Zeitplan.
