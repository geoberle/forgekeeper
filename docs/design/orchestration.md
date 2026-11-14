# Go-Orchestrierung — bestätigter Entwurf

## Gesetzte Vorgaben

- Go; Options-Muster nach `~/dev/aro-hcp/tooling/templatize`: raw → validated → completed. Fertige Clients in completed.
- Gut testbar; Go-Tests tabellarisch, Golden Fixtures für geeignete Ausgaben; Logging mit logr.
- Fabro führt den vorgegebenen Workflow aus. Die Orchestrierung wählt GitHub-Issues und startet Runs.
- Gerd mergt jeden PR. Ein offenes Issue kann nach einem gemergten Teilbeitrag erneut ausgewählt werden.
- Pro Repository und Issue höchstens ein aktiver Run und höchstens ein zugeordneter offener PR.

## Bestätigte Entscheidungen

- Auswahl und Start werden nach Go verlagert; die vorhandenen Python-Workflow-Schritte bleiben zunächst bestehen.
- Der Orchestrator gleicht GitHub und Fabro periodisch per Polling ab.
- Initialisierte Clients werden in completed bereitgestellt.
- Fehlgeschlagene Runs werden nicht automatisch erneut gestartet. Ein ausdrücklicher Retry gibt das Issue wieder frei.
- Nach einem manuell gemergten Teil-PR wird ein weiterhin offenes und bereites Issue automatisch erneut berücksichtigt.

- Der Go-Prozess läuft zunächst als eigener Prozess im bestehenden Fabro-Container.
- Zunächst ein aktiver Run insgesamt; ältestes bearbeitbares Issue über alle Repositories zuerst. Spätere Parallelität soll möglich bleiben.
- Vorhandene Runs werden nach Neustarts über Repository und Issue wiedererkannt. Eine atomare Exactly-once-Garantie für die Erstellung wird nicht beansprucht.
- Die Auswahlfunktion verarbeitet eingelesene Daten ohne I/O; die ausführende Schleife verwendet initialisierte Clients.

## Fehler und Retry

- Ein Bot-Kommentar pro fehlgeschlagenem Run, mit fester kurzer Fehlerbeschreibung, betroffenem Schritt und Fabro-Link. Kein LLM-Aufruf zur Zusammenfassung.
- Nur eine 🚀-Reaktion von `geoberle` auf diesen Kommentar autorisiert genau einen neuen Versuch. Alte Reaktionen autorisieren keine späteren Fehlversuche. Normale Startbedingungen gelten weiterhin.
- Credentials dürfen niemals in Kommentaren oder Logs erscheinen. Externe Fehlertexte, HTTP-Bodies, Prozessausgaben und Konfiguration werden nicht ungefiltert ausgegeben. Öffentliche Fehlermeldungen stammen aus festen Kategorien; auch internes Logging darf keine Credentials enthalten.

## Zustand und Wiedererkennung

- GitHub-Issues, PRs, Fehlerkommentare und Fabro-Runs sind die Zustandsquellen. Keine lokale Zustandsdatenbank oder persistente Startabsicht.
- Runs werden über das vorhandene Label `github_issue=owner/repo#nummer` zugeordnet. Mehrere abgeschlossene Runs desselben Issues sind normal; aktive Runs verhindern einen weiteren Start.
- Kein zusätzlicher Startkommentar oder Versuch-ID-Protokoll. Fehlerkommentare und Retry-Freigaben beziehen sich auf den konkreten fehlgeschlagenen Run.
- Vor einer neuen Erstellung vorhandene Runs abgleichen. Nach einer unklaren Antwort beim Create zunächst erneut abgleichen.
- Explizite Entscheidung von Gerd: Das Sichtbarkeitsfenster zwischen Run-Persistierung und Listenprojektion wird ignoriert. Liefert der vollständige Abgleich keinen Run, darf eine ansonsten berechtigte Erstellung erfolgen. Ein fehlgeschlagener oder unvollständiger API-Abruf zählt nicht als leeres Ergebnis.
- Dadurch kann der seltene Fall eines noch unsichtbaren, bereits angenommenen Create-Aufrufs einen doppelten Run verursachen. Es wird dafür keine weitere Zustandsverwaltung eingeführt.

Die Go-Implementierung liegt unter `cmd/orchestrator` und `internal/orchestrator`. Der Python-Starter wurde entfernt; die Python-Workflow-Schritte bleiben. `fabro create` übernimmt die native Paketierung, die initialisierten HTTP-Clients übernehmen GitHub und Fabro-Abgleich. Ein temporärer flock schützt vor zwei aktiven Instanzen; er enthält keinen dauerhaften Zustand. Fehlermeldungen aus SDK, HTTP-Bodies und CLI-Ausgaben werden nicht in Logs oder Kommentare übernommen.

Bedienung: `scripts/orchestrate.sh --preview --once`, danach ausdrücklich `scripts/orchestrate.sh`. Der Serverstart aktiviert kein Polling. `scripts/run-issue.sh` verwendet denselben Go-Pfad mit Repository-/Issue-Filter.
