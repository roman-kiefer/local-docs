---
name: local-doc-update
description: Pruefe auf ausdrueckliche Anfrage, ob eine neue Version des Programms local-docs vorliegt, und installiere sie.
---

# local-docs aktualisieren

Nutze diesen Skill nur, wenn der Nutzer das Programm `local-docs` aktualisieren oder auf eine neue Version pruefen lassen moechte. Normale Suchen oder ein Index-Sync loesen kein Update aus. Das Update ersetzt die ausfuehrbare Datei und darf nie ungefragt erfolgen.

1. `local-docs version` ausfuehren und die installierte Version merken.
2. `local-docs update --check` ausfuehren. Meldet es keine neue Version, dem Nutzer das mitteilen und enden.
3. Gibt es eine neue Version, nenne alte und neue Version und frage, ob installiert werden soll, sofern der Nutzer nicht schon ausdruecklich das Update verlangt hat.
4. Dann `local-docs update` ausfuehren. Das Programm prueft die SHA-256-Summe des Release, ersetzt sich selbst und aktualisiert die bereits installierten Skills.
5. Melde alte und neue Version. Falls der Index betroffen sein koennte, weise auf `local-docs sync` hin, fuehre es aber nur auf Anfrage aus (Skill `local-doc-sync`).

Bei Fehlern (Netzwerk, Pruefsumme) die Ausgabe wiedergeben und nichts manuell ersetzen. Eine Entwicklungsversion (`dev`) wird nicht ersetzt.
