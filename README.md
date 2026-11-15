# GitHub-Issues mit Fabro und Codex bearbeiten

Dieses Repository startet einen Fabro-Server in Podman. Ein gemeinsamer Workflow lässt Codex ein GitHub-Issue bearbeiten, führt feste Projektprüfungen aus und öffnet einen Pull Request. **Du mergst jeden PR selbst.** Der Server startet beim Hochfahren keine Issue-Bearbeitung.

Die Go-Orchestrierung wählt bereite Issues per Polling aus, startet Fabro-Runs und erkennt vorhandene Runs über Repository und Issue wieder. Sie läuft als eigener Prozess im Container und wird nicht beim Serverstart aktiviert. Der [Orchestrierungsentwurf](docs/design/orchestration.md) beschreibt den Vertrag.

Für Kubernetes gibt es ein [Helm-Chart mit getrennten Pods und CI-Image-Build](docs/kubernetes.md). Die CI übernimmt Skills und Agents aus dem privaten Skillshare-Repository. Das Chart startet die Orchestrierung im Preview-Modus.

## Voraussetzungen

- Eine laufende Podman-Machine: `podman machine start`.
- Python 3.13; für lokale Go-Entwicklung Go 1.25.5. Beide Versionen stehen in `.tool-versions` für asdf.
- Lokale Codex-Skills und Plugin-Cache für den Image-Build. Das Build-Skript übernimmt die vorhandenen Skill-Dateien, keine Host-Anmeldungen. Benötigt werden insbesondere `implement`, `tdd`, `code-review` und `pr`.
- Ein GitHub-Bot mit Zugriff auf die Ziel-Repositories und einem PAT: **Contents: write**, **Pull requests: write**, **Issues: write** für Fehlerkommentare und das Lesen von Retry-Reaktionen.
- Ein Codex-/ChatGPT-Login im Container. Zusätzlich benötigt die eingesetzte Fabro-Version einen konfigurierten Modellprovider; dieses Setup verwendet OpenRouter. Codex selbst arbeitet über ACP mit seinem eigenen Login.

## Einrichten und starten

Alle Befehle im Repository-Root ausführen.

### 1. Bot-PAT hinterlegen

Speichere den Token in `.local/secrets/github-pat`. Verwende einen Editor oder Passwortmanager; schreibe den Token nicht als Shell-Befehl in die History.

```bash
mkdir -p .local/secrets
chmod 700 .local/secrets
# Token jetzt in .local/secrets/github-pat speichern.
chmod 600 .local/secrets/github-pat
```

Alternativ kannst du beim Erstellen des Containers eine vorhandene Datei auswählen:

```bash
FABRO_GITHUB_PAT_FILE=/absoluter/pfad/zum/bot-pat ./scripts/start.sh
```

Dafür muss das Image aus dem nächsten Schritt bereits gebaut sein. Die Dateiauswahl wird beim Erstellen des Containers festgelegt, nicht beim erneuten Start eines vorhandenen Containers.

### 2. Image bauen und Fabro starten

```bash
./scripts/build.sh
./scripts/start.sh
```

Die UI läuft unter **<http://localhost:32277>**. Falls sie nach einem Dev-Token fragt:

```bash
./scripts/ui-token.sh
```

Das Skript kopiert den Token unter macOS in die Zwischenablage. Füge ihn in die lokale UI ein. Konfiguriere dort OpenRouter als Modellprovider, falls das noch nicht geschehen ist. Ohne Provider blockiert die eingesetzte Version auch ACP-Workflows; Details und bisherige Tests stehen in [RESULTS.md](RESULTS.md).

### 3. Codex anmelden

```bash
./scripts/login.sh
```

Öffne die ausgegebene Device-Login-Adresse im Browser auf deinem Rechner und bestätige die Anmeldung. Der Browser muss nicht im Container laufen. Der Login bleibt in `.local/data/home/.codex/` erhalten.

Anmeldestatus prüfen:

```bash
podman exec --user fabro fabro-experiment codex login status
```

## Ein Projekt hinzufügen

Lege genau eine Datei pro Repository unter `projects/` an, beispielsweise `projects/mein-projekt.yaml`:

```yaml
repository: owner/repository

instructions: |-
  Lies die Projektanweisungen und wähle einen unabhängig prüfbaren Teilbeitrag.
  Behaupte keine Hardware-Prüfungen, die nicht ausgeführt wurden.

verification:
  - make verify
```

`verification` enthält die tatsächlichen Prüfkommandos des Zielprojekts. Sie laufen im ausgecheckten Projekt innerhalb des Containers. Benötigte Tools müssen im Image vorhanden sein; zusätzliche Pakete gehören in [container/Dockerfile](container/Dockerfile).

Der Workflow bleibt für alle Projekte derselbe: [workflows/implement-issue/workflow.toml](workflows/implement-issue/workflow.toml).

## Orchestrierung starten

Erst die Auswahl ohne Änderungen prüfen:

```bash
./scripts/orchestrate.sh --retry-user YOUR_LOGIN --preview --once
```

Die Ausgabe nennt pro Issue die Aktion und den Grund, etwa `not_ready`, `blocked`, `open_pr`, `retry_required` oder `capacity`. Anschließend dauerhaft pollen:

```bash
./scripts/orchestrate.sh --retry-user YOUR_LOGIN --interval 1m
```

Ein einzelner aktiver Orchestrator koordiniert zunächst **einen aktiven Run insgesamt**. Er wählt das älteste bearbeitbare Issue aus allen `projects/*.yaml`. Ein bereits eingereichter, noch nicht gestarteter Run wird wiederaufgenommen. `--once` gleicht genau einmal ab. Stoppen des Prozesses stoppt keine bereits laufenden Fabro-Runs; der nächste Orchestrator erkennt sie wieder.

Fehlgeschlagene Runs erhalten einen Bot-Kommentar mit Schritt und Fabro-Link. **Nur eine 🚀-Reaktion vom konfigurierten `--retry-user` auf den Kommentar des jüngsten fehlgeschlagenen Runs** erlaubt einen neuen Versuch. Die Zuordnung erfolgt über GitHub-Benutzer-IDs. Alte oder fremde Reaktionen und kopierte Kommentare anderer Autoren lösen keinen Retry aus. Ready-Label, Abhängigkeiten und PR-Prüfung gelten weiterhin. Nach einem gemergten Teil-PR wird ein weiterhin bereites Issue automatisch erneut berücksichtigt.

GitHub und Fabro sind die Zustandsquellen. Der Prozess verwendet lediglich einen temporären exklusiven Prozesslock. `.local/data/issue-jobs/` enthält weiterhin Arbeitskopien und Workflow-Eingaben, aber keine neue Orchestrierungsdatenbank. Der ausdrücklich akzeptierte seltene Create-/Listen-Sichtbarkeitsfall kann einen Doppelstart verursachen; fehlgeschlagene API-Abfragen werden dagegen niemals als leere Listen behandelt.

## Ein Issue bearbeiten

Ein Issue wird standardmäßig in einem Run und einem PR vollständig umgesetzt.
Teil-PRs brauchen einen konkreten Blocker oder eine notwendige getrennte Vorarbeit;
kleine Diffs allein sind kein Aufteilungsgrund. Implementierung und Review prüfen
die Akzeptanzkriterien einschließlich bereits gemergter Arbeit. Ausdrücklich
akzeptierte Lücken und Anforderungen anderer Tickets verhindern den Abschluss nicht.


Zuerst prüfen, ob das Issue bearbeitbar ist — ohne Run oder GitHub-Änderung:

```bash
FORGEKEEPER_RETRY_USER=YOUR_LOGIN ./scripts/run-issue.sh owner/repository 8 --check
```

Erforderlich sind:

- Issue ist offen und trägt `ready-for-agent` oder `ready-for-test`.
- Keine `wayfinder:*`-Labels und keine offenen nativen GitHub-Abhängigkeiten.
- Kein erkennbar zugeordneter offener PR und kein aktiver vorheriger Run.

Bei vorhandenem PR #43 kann Issue #8 deshalb zunächst gesperrt sein. Wähle ein tatsächlich bereites Issue und starte es ohne `--check`:

```bash
./scripts/run-issue.sh OWNER/REPO ISSUE_NUMMER
```

Das Skript meldet die Auswahlentscheidung und bei einem neuen Start die Run-ID. Es läuft vom Host aus, führt die Arbeit aber **im Container** mit dessen Bot-PAT aus; dein Host-`gh`-Login wird nicht verwendet.

Der Ablauf ist:

1. `context`: Issue, Kommentare, Blocker und PRs laden und Startbedingungen prüfen.
2. `implement`: Codex verwendet den Implement-Skill und committet seinen Beitrag.
3. `result` / `verify`: Ergebnis prüfen und feste Projektkommandos ausführen. Bei fehlgeschlagenen Prüfungen sind höchstens zwei Reparaturrunden erlaubt.
4. `review`: Steve und David prüfen denselben getesteten Commit in einem ACP-Step. Das deterministische Gate verlangt beide Berichte. Blockierende Findings gehen als `review.json` und `review.md` zurück an `implement`; nach höchstens zwei Korrekturrunden bleibt der Beitrag ein Draft. Notwendige menschliche Entscheidungen führen ebenfalls zum Draft; ungültige Berichte stoppen den Run. Nach Änderungen laufen Tests und beide Reviews erneut.
5. `publication`: Ein Code-Gate erlaubt einen normalen PR nur mit erfolgreicher Verifikation und Review-Freigabe für denselben Commit. Brauchbare Beiträge mit blockierten Prüfungen, offenen Findings oder `needs_human` werden als Draft veröffentlicht. Ohne Diff stoppt der Run.
6. `pr`: Codex verwendet den PR-Skill, pusht und öffnet den PR mit Git/`gh`. Anschließend verknüpft er ihn mit Fabro.

Ein Teilbeitrag verwendet `Part of #N`; eine vollständige Umsetzung darf `Fixes #N` verwenden. Prüfe das beim Review. Der Run endet nach der PR-Erstellung; GitHub-CI kann dann noch laufen. Der Orchestrator kann anschließend eigene Low-Risk-PRs automatisch mergen, wenn das Projekt `auto_merge: true` setzt.

Auto-Merge wird pro Projekt aktiviert:

```yaml
auto_merge: true
merge_checks:
  - Lint and test
```

Nur PRs mit der numerischen Autor-ID des authentifizierten Bots und einer
Forgekeeper-Run-Zuordnung sind für Folgearbeit oder Auto-Merge zugelassen.
Beide Reviewer müssen den vollständigen PR unabhängig als `risk: low` mit
`risk_reason` bewerten. Fehlende Risikobewertungen in alten Runs, fehlende
Verifikation, Blocker, Drafts, Änderungsanforderungen, laufende Folgearbeit und
abweichende Head- oder Base-SHAs verhindern den Merge. Aktive Runs sperren nur
den zugeordneten PR; ohne PR-Zuordnung sperren sie vorsichtshalber PRs desselben
Issues. Unabhängige Runs verhindern die Merge-Prüfung nicht. Es gibt keine Dateiallowlist.
Die Reviewer bewerten Auswirkungen und Testabdeckung des konkreten Diffs unter der
Bedingung später erfolgreicher Pflicht-CI. Ausstehende GitHub-CI allein erhöht das
Risiko nicht; der Orchestrator prüft sie separat vor dem Merge. Medium/High müssen
ein konkretes Fehlerszenario und die fehlende Evidenz oder Entscheidung benennen.
Bestehende Projektlücken zählen nur, wenn die Änderung davon abhängt.
Für Verhaltensänderungen dokumentiert der Implementierungsschritt einen gezielten
Rot/Grün-Nachweis mit Befehlen und Ergebnissen in `regression.md` im Job-Verzeichnis
oder erklärt die konkrete Einschränkung. Die Reviewer prüfen die Aussagekraft der
Tests und geben relevante Abdeckungslücken als blockierende Findings zurück.
Die Risikobewertung bleibt eine LLM-Entscheidung; die übrigen Bedingungen prüft Code.
Ein Reply-only-Run erzeugt keine neue Merge-Freigabe.

Alle beobachteten Checks/Status müssen erfolgreich sein. Jeder Eintrag in
`merge_checks` muss als erfolgreicher GitHub-Actions-Check auf dem veröffentlichten
Commit existieren; fehlende, übersprungene oder neutrale Checks reichen nicht.
Private Repositories werden direkt mit erwarteter Head-SHA und einem Merge-Commit
gemergt, damit die Implementierungscommits erhalten bleiben. Ohne Branch Protection
können Änderungen an Reviews, Checks oder Base zwischen Prüfung und Merge nicht
atomar ausgeschlossen werden. Ein neuer Head wird vom Merge-Endpunkt abgewiesen.

Öffentliche Repositories nutzen GitHub Auto-Merge mit erwarteter Head-SHA.
Auto-Merge und Merge-Commits müssen im Repository erlaubt sein. Branch Protection
muss aktuelle Branches (`strict`) sowie `forgekeeper/low-risk` und alle
`merge_checks` verlangen. Der Orchestrator setzt den Risikostatus nur für den
geprüften Commit; ein neuer Commit benötigt neue Evidenz und einen neuen Status.
Fehlende Schutzregeln werden nicht automatisch eingerichtet und verhindern die
Freigabe. `--preview` bleibt vollständig lesend. `auto_merge: false` stoppt neue
Freigaben; bereits in GitHub aktiviertes Auto-Merge muss dort deaktiviert werden.

Nach dem Merge eines Teil-PRs kannst du dasselbe weiterhin offene, bereite Issue erneut starten; laufendes Polling übernimmt das automatisch. Es beginnt ein neuer Run auf dem aktuellen Default-Branch.

Prüfkommandos erhalten eine eingeschränkte Umgebung ohne vererbte Token-Variablen. Bekannte Secret-Werte werden vor dem Schreiben in Prüf-Logs maskiert; die Ausgabe eines Kommandos erscheint dort erst nach dessen Ende. Das ersetzt keine separate Umgebung ohne Credential-Dateien.

## Ergebnis und Kontext ansehen

Öffne den Run in der [Fabro-UI](http://localhost:32277):

- **`context` → Output:** vollständiger Issue-Kontext als Markdown-Text.
- **Artifacts → `.fabro-context.md`:** denselben Kontext herunterladen. Die Artefaktansicht rendert Markdown derzeit nicht direkt.
- **`review_gate` → Output / Artifacts → `.fabro-review-N.md`:** beide Review-Berichte pro Runde. Die Antworten des Implementierers liegen im Job-Verzeichnis unter `review-response.md`.
- **`verify` → Output:** Ausgabe bei einer fehlgeschlagenen Prüfung. Das vollständige Prüfprotokoll liegt außerdem im Job-Verzeichnis.
- **Run-Kopf / Run-Liste:** PR-Link, sobald der Agent `fabro pr link` ausgeführt hat. GitHub-Details können ohne separate Fabro-GitHub-Integration fehlen.

Auf dem Host liegen die Job-Dateien unter `.local/data/issue-jobs/JOB_ID/`, insbesondere `context.md`, `result.json`, `verify.log` und nach Veröffentlichung `pr.json`.

Für einen fehlenden PR-Link kannst du die Zuordnung nachtragen:

```bash
podman exec --user fabro fabro-experiment \
  fabro pr link --server http://127.0.0.1:32276 RUN_ID https://github.com/OWNER/REPO/pull/NUMMER
```

## Fehler untersuchen

Bei `needs_human`, fehlenden Voraussetzungen oder erschöpften Reparaturversuchen endet der Run ohne erfolgreichen Abschluss. Lies den fehlgeschlagenen Schritt und die Job-Dateien. **Der laufende Orchestrator veröffentlicht dazu einen festen Fehlerkommentar und wartet auf die Retry-Reaktion.**

Der Go-Starter ersetzt die früheren lokalen Startzuordnungen. Ein offener oder ohne Merge geschlossener PR des letzten Runs blockiert einen neuen Start. Bei einem erfolgreichen Run ohne auffindbaren zugehörigen PR erscheint `pr_not_reconciled`; prüfe dann den Run und die GitHub-Zuordnung. Alte Claim-Dateien werden nicht mehr ausgewertet.

Terminal im Container öffnen:

```bash
podman exec -it --user fabro fabro-experiment bash
```

Workspace-Pfade liefert `sandbox.instance.runtime.working_directory` im Run-State. Rohdaten können Projektinhalte und Ausgaben enthalten; nicht ungeprüft in öffentliche Issues kopieren.

```bash
./scripts/api.py GET /runs/RUN_ID/state
```

Die Issue-Workflows über `run-issue.sh` oder die Orchestrierung starten. Direkte UI-Starts oder Retries umgehen die Startprüfung des Skripts.

## Aktualisieren, Token wechseln und stoppen

Nach Änderungen am Image oder an der PAT-Dateiauswahl den Container bei inaktiven Runs neu erstellen:

```bash
./scripts/build.sh
podman stop fabro-experiment
podman rm fabro-experiment
./scripts/start.sh
```

Der gleiche Ablauf ohne `build.sh` reicht für eine neue PAT-Dateiauswahl. `.local/data` und Logins bleiben erhalten. Änderungen an Skripten, Projekten und Workflows sind über Mounts verfügbar; ändere Skripte nicht während eines Runs, da sie nicht Teil von Fabros unveränderlichem Workflow-Snapshot sind.

Nur stoppen / wieder starten:

```bash
podman stop fabro-experiment
./scripts/start.sh
```

## Lokal entwickeln und prüfen

```bash
make install-tools
make fmt
make lint
make test
make verify
```

`make verify` prüft Go-/Python-/Shell-Formatierung, golangci-lint, Ruff, ShellCheck, YAML, Go-Tests mit Race Detector und Python-Unit-Tests. `make build` baut das lokale Go-Binary. GitHub Actions führt denselben Befehl ohne Bot-PAT, Modellprovider oder laufenden Fabro-Server aus. Tools und Caches liegen unter `.local/` und werden nicht eingecheckt.

Optionale Funktionstests im laufenden Container:

```bash
./scripts/run.sh control  # Command-Workflow mit absichtlich defekter Fixture und Human-Gate
./scripts/run.sh codex    # Codex-ACP-Workflow, benötigt Anmeldung und Modellprovider
```

Diese Demo-Workflows öffnen keine GitHub-PRs. Ihr Human-Gate gehört zur Demo; im Issue-Workflow bleiben PRs ohne Low-Risk-Freigabe beim manuellen Merge.

## Grenzen und weiterführende Dokumente

Der Podman-Container ist die Sandbox; Codex läuft darin ohne innere Sandbox. Server und Agenten teilen Credentials und Zustand. Host-Home und Container-Socket werden nicht eingebunden, aber dies ist keine Isolation zwischen gegenseitig unvertrauenswürdigen Runs. Merge-Verbote und Prüfungen im PR-Schritt sind Agent-Anweisungen, keine harte GitHub-Berechtigungsgrenze.

Sorties Code wurde entfernt. Der alte Kind-Cluster wurde nicht gelöscht; sein Kubeconfig liegt separat unter `.local/sortie-kubeconfig` und wird von Fabro nicht verwendet.

- [Workflow-Vertrag](WORKFLOW-DESIGN.md)
- [Versuchsergebnisse und bekannte Einschränkungen](RESULTS.md)
- [Go-Orchestrierung](docs/design/orchestration.md)
- [Begriffe](GLOSSARY.md)

## Kommentare und Reviews an bestehenden PRs

Forgekeeper pollt alle offenen PRs der konfigurierten Repositories, die über einen
Fabro-Run zugeordnet sind. Neue PR-Kommentare, veröffentlichte Reviews und
Inline-Kommentare des mit `--retry-user` konfigurierten Accounts starten
Folgearbeit ohne Bot-Erwähnung. Eigene Bot-Beiträge und andere Autoren starten
keinen Run; ihre Inhalte dienen nur als nicht vertrauenswürdiger Kontext.
CI-Checks werden als Kontext geladen, lösen aber keine Arbeit aus.

Die Folgearbeit verwendet denselben PR und Branch. Antworten und Rückfragen ohne
Codeänderung überspringen Tests und Reviews und verändern den Draft-Status nicht.
Codeänderungen durchlaufen Verifikation und Steve/David; anschließend bestimmt
das Publication-Gate Draft oder Ready. Der Mensch mergt weiterhin selbst.
Vor der Veröffentlichung prüft der Agent, dass niemand den PR geschlossen,
gemergt oder dessen Head verändert hat. Es gibt keinen Force-Push.

Fabro-Labels `github_pr` und `feedback_cursor` speichern pro Sammlung die höchsten
Kommentar-/Review-IDs des gestarteten Auftrags, auch bei archivierten Runs.
Während ein Run aktiv ist, wird kein zweiter gestartet; neue Beiträge bleiben
für den nächsten Poll übrig. Ein fehlgeschlagener Auftrag wird am PR gemeldet
und nicht automatisch wiederholt. Ein neuer Kommentar autorisiert einen neuen
Versuch, der die bisherige Diskussion ebenfalls erhält. Bereits veröffentlichte
Kommentare nachträglich zu editieren löst keinen neuen Run aus: Korrekturen bitte
als neuen Kommentar schreiben. Vorhandene ältere Beiträge des autorisierten
Accounts werden beim ersten Poll ebenfalls berücksichtigt.

Wie beim Issue-Start gibt es keine serverseitige Create-Idempotenz: Nach einer
unklaren Create-Antwort sucht der nächste Poll den Run anhand seiner Labels.
Die zuvor bewusst akzeptierte Annahme, dass ein persistierter Run dabei sichtbar
ist, bleibt bestehen. Fortschritt liegt in Fabro; keine zusätzliche lokale
Orchestrierungsdatenbank wird angelegt.

## Commit-Historie veröffentlichter PRs

Vor der Veröffentlichung erzeugt der `history`-Step eine separate Git-Referenz.
Leere Fabro-Checkpoint-Commits werden ausgelassen; Implementierungs-Commits behalten
Reihenfolge, vollständige Commit-Nachricht und Autorenmetadaten. Durch die neuen
Eltern-Commits ändern sich ihre SHAs; vorhandene Commit-Signaturen können dabei
nicht übernommen werden. Nichtleere Checkpoints werden als normale Inhalts-Commits
erhalten, damit keine uncommitteten Agent-Änderungen verloren gehen.

Der veröffentlichte Git-Tree muss exakt dem geprüften Tree entsprechen. Der
PR-Agent pusht ausschließlich `publication.json.publish_head`, niemals den
internen Run-HEAD. Fabros Checkpoints bleiben für Wiederaufnahme und Debugging
unverändert. Bei Folgearbeit bleibt der bisherige PR-Head erhalten; nur die neu
hinzugekommenen Commits werden bereinigt. Merge-Commits in diesem neuen Bereich
stoppen die Veröffentlichung, statt sie stillschweigend zu flatten. Bereits
veröffentlichte oder gemergte Historie wird nicht nachträglich geändert.


### Codex-Modelle

Implementierung und Reparatur verwenden Sol/medium, der Review-Step mit Steve
und David Astra/low und die PR-Erstellung Sol/low. Das gilt sowohl für neue
Issues als auch für PR-Folgearbeit. Überschreiben über Helm:

```yaml
fabro:
  codex:
    implement:
      model: gpt-6.1-sol
      model_reasoning_effort: medium
    review:
      model: gpt-6-astra
      model_reasoning_effort: low
    pr:
      model: gpt-6.1-sol
      model_reasoning_effort: low
```

Außerhalb von Helm nehmen `CODEX_IMPLEMENT_CONFIG`, `CODEX_REVIEW_CONFIG` und
`CODEX_PR_CONFIG` dieselben Einstellungen als JSON-Objekt an. Der Step-Launcher
übergibt die ausgewählte Konfiguration als `CODEX_CONFIG` an Codex ACP.
Die Einstellungen gelten für neu gestartete Agent-Sessions. Fabros eigener
Modellprovider ist separat konfiguriert.
