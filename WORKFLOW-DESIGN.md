# Issue → geprüfter PR

Stand: 13. November 2026. Der zentrale Workflow und der manuelle Starter sind implementiert; Issue #8 wurde erfolgreich bis PR #43 ausgeführt. Die Go-Orchestrierung kann ausdrücklich per `scripts/orchestrate.sh` gestartet werden; der Serverstart aktiviert keine Ticketauswahl.

## Vertrag

Das Issue-Label autorisiert die Bearbeitung: `ready-for-test` oder `ready-for-agent`. Jeder PR wird von Gerd selbst gemergt. Ein Run bearbeitet einen begrenzten Beitrag, prüft ihn, erstellt einen PR und endet. Er wartet nicht auf den Merge und hält keine eigene Inkrement-Lebensdauer bis zum Merge offen.

Ein späterer Auswahlzyklus kann dasselbe weiterhin offene Issue erneut auswählen. Ohne laufendes Polling startet Gerd diesen unabhängigen Run erneut über `scripts/run-issue.sh OWNER/REPO ISSUE`. Der aktuelle Hauptbranch und Issue-/PR-Kontext liefern den bisherigen Fortschritt; eine besondere Fortsetzungsbeziehung zwischen Runs ist nicht nötig.

Pro `(repository, issue)` darf höchstens ein aktiver Run und höchstens ein zugeordneter offener PR existieren. Beide können während der Veröffentlichung gleichzeitig existieren. Andere Issues dürfen unabhängig bearbeitet werden; die derzeitige Fabro-Serverkonfiguration begrenzt die gesamte Ausführung zusätzlich auf einen Run.

## Implementierter Graph

Definition: `workflows/implement-issue/workflow.toml` und `workflow.fabro`; Prompts liegen daneben. Projekte konfigurieren Repository, Anweisungen und feste Prüfkommandos in `projects/*.yaml`.

1. **Startprüfung:** offenes Issue, `ready-for-test` oder `ready-for-agent`, keine `wayfinder:*`-Labels, keine offenen nativen GitHub-Blocker, kein aktiver vorheriger Run und kein erkennbarer offener PR. Ein temporärer Prozesslock erlaubt eine aktive Orchestrator-Instanz; Runs werden über GitHub-Issue-Labels in Fabro wiedererkannt.
2. **Kontext:** frischer Clone des Default-Branches; Issue, Kommentare, Blocker und zugehörige PRs als `context.md` bereitstellen. Den vollständigen Markdown-Text im Step-Output anzeigen und als `.fabro-context.md`-Artefakt sammeln; die Workspace-Kopie bleibt lokal Git-ignoriert. Im Run Startbedingungen erneut prüfen.
3. **Implementieren:** Codex über ACP verwendet `implement`, einschließlich TDD und Code-Review. Er wählt selbst einen sinnvollen begrenzten Beitrag. Keine zusätzliche Planungsinstanz und keine verpflichtenden Subissues. Bei fehlenden Entscheidungen oder Validierungsumgebungen meldet er `needs_human`.
4. **Prüfen:** feste Projektkommandos ausführen, Exitcode und Logs erfassen. Bei Fehlern höchstens zwei Reparaturrunden, insgesamt drei Prüfversuche. Danach endet der Run fehlgeschlagen. Ein leerer Beitrag oder `needs_human` endet ohne PR.
5. **PR erstellen:** Codex verwendet den `pr`-Skill, prüft gemäß Prompt den getesteten Git-Tree und die aktuellen GitHub-Startbedingungen, pusht mit Git und öffnet den PR mit `gh`. Er speichert die PR-Zuordnung und verknüpft den PR über `fabro pr link` mit der Run-Ansicht. Kein eigener Publish-Helper, keine Codeänderungen und kein automatischer Merge. GitHub-CI darf nach Run-Ende noch laufen.

## Teilfortschritt und Gesamtabschluss

Der Skill entscheidet wie im bisherigen Arbeitsablauf: Teilfortschritt referenziert das Issue etwa mit `Part of #15`; vollständige Umsetzung darf `Fixes #15` verwenden. Gerd prüft diese Aussage beim PR-Review. Es gibt keine zusätzliche Abschlussfreigabe nach dem Merge und kein pauschales Verbot von Closing-Keywords.

Für eine spätere Auswahl zählt GitHubs tatsächlicher Issue-Zustand. Ein geschlossenes Issue wird nicht erneut ausgewählt. #16 und #17 bleiben blockiert, solange ihr nativer Blocker #15 offen ist. Das Schließen von #15 beseitigt diese Blockade; die Nachfolger benötigen weiterhin ihr eigenes Freigabelabel.

## Zustand und Grenzen des Experiments

`/storage/issue-jobs/` enthält pro Versuch Kontext, Konfiguration, Prüflogs und PR-Daten. Die Orchestrierung liest GitHub und Fabro als Zustandsquellen und benötigt keine lokalen Startdatensätze. Ein ohne Merge geschlossener PR des letzten Runs wird nicht automatisch ersetzt. Der akzeptierte Create-/Listen-Sichtbarkeitsfall wird nicht durch zusätzliche Startabsichten abgesichert.

Die Ein-Run-Regel gilt für Starts durch unser Skript. Direkte UI-Starts oder Wiederaufnahmen müssen damit koordiniert werden. Externe PRs werden anhand ihrer Issue-Verweise in Titel/Beschreibung erkannt; völlig unverknüpfte PRs sind nicht zuverlässig zuzuordnen. PR-Links in Fabro sowie der Branch aus dem Job-Input ermöglichen zusätzliche Zuordnung, auch ohne lokale PR-Metadaten.

Fabro kann zwischen Stages Checkpoint-Commits anlegen. Der PR-Prompt verlangt deshalb den Vergleich des Git-Trees statt der Commit-SHA. Diese Prüfung liegt beim Agenten und wird nicht mehr von einem festen Publish-Schritt erzwungen.

Agenten, Server und Zustand teilen den äußeren Container. Prompts untersagen Merge und Änderungen am Workflow-Zustand, sind aber keine technische Zugriffssperre gegen einen feindlichen Agenten. Helpers sind read-only vom Host eingebunden und noch kein Bestandteil von Fabros unveränderlichem Workflow-Paket. OpenShell und Kubernetes bleiben separate spätere Arbeit.

## Skills und Validierung

Das Image bündelt alle ermittelten persönlichen, Codex-System- und aktuellen gecachten Plugin-Skills, ausgenommen explizit deaktivierte Plugins: derzeit 172. Plugin-Namen werden zur Kollisionsvermeidung präfixiert; Ressourcen werden mitkopiert. MCP-/App-Anmeldungen und zusätzliche externe Laufzeitwerkzeuge werden dadurch nicht automatisch eingerichtet.

Geprüft: Graphvalidierung, native Run-Erstellung ohne Start, korrekte Inputs und deaktivierte native Veröffentlichung in der zugelassenen Run-Konfiguration, sowie Tests für Startbedingungen, konkurrierende Starts, unsichere Startantworten, Prüffehler und Reparaturgrenze. Issue #8 erzeugte PR #43 noch mit dem früheren Publish-Helper; der vereinfachte ACP-PR-Schritt wurde noch nicht an einem neuen Issue ausgeführt. Bedienung und verbleibende Einrichtung stehen in `README.md`.
