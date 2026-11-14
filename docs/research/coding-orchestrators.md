# Coding-Orchestratoren: neue Primärquellenrecherche

Stand: **13. November 2026**, Tool-Uhr bei Recherchebeginn **12:32:20 UTC**. Nur öffentliche Quellen gelesen; keine Installation, privaten Repositories oder Sortie-Konfiguration verändert.

## Ergebnis und Korrektur der vorherigen Suche

Sandcastle identifiziert die richtige Kategorie: Coding-spezifischer ausführbarer Kontrollfluss um Agentenprozesse und isolierte Arbeitsbereiche. Die frühere Auswahl allgemeiner Workflow-Engines beantwortete eine andere Frage. Argo, Temporal, Prefect, LangGraph und CI-Engines füllen hier keine Plätze.

**Keine zehn nachweislich etablierte, breit von Menschen gepflegte Volltreffer gefunden.** Es gibt jedoch deutlich mehr ernsthafte Coding-Orchestratoren als die frühere Recherche erkennen ließ. Für eine konkrete Evaluation stehen **TAKT, Codev und Issue-Orchestrator** vorne; bei letzterem ist die Maintainer-Konzentration besonders ausgeprägt. Sandcastle bleibt der klarste kleine Codex/Podman-Baukasten. Die Rangfolge bewertet funktionale Passung, nicht Popularität oder gesicherte Produktionsreife.

Harte Workflow-Anforderung: mehrere ausführbare, extern gesteuerte Coding-Schritte in Code/Config. Davon getrennt wird geprüft, ob Tests wirklich durch einen Prozess-Exitstatus sperren. Ein Agentenereignis wie „Tests grün“ ist kein solcher Nachweis. Ebenso sind Tool-Freigaben keine Pipeline-Freigaben, Task-Kanten keine nativen GitHub-Issue-Blocker und Session-Resume keine dauerhafte Pipeline-Wiederaufnahme.

## Zehn Coding-Kandidaten, ausdrücklich keine zehn etablierte Empfehlungen

Fit ist eine qualitative Einschätzung von 1–5; Wartungsvertrauen wird separat ausgewiesen. „Offen“ bedeutet nicht „unmöglich“, sondern in dieser Recherche nicht ausreichend nachgewiesen.

| Rang | Kandidat | Fit | Workflow-Code | Test-Gate | Wartungsurteil |
|---|---|---:|---|---|---|
| 1 | TAKT | 4,5 | Ja: YAML-Schritte/Regeln und Runtime | Command-Gate mit Exitcode vorhanden | Mehrere Monate, externe Beiträge, aber stark konzentriert |
| 2 | Codev / Porch | 4,5 | Ja: JSON-Protokoll und Phasenmaschine | Separate Check-Kommandos | Mehrere Monate; zuletzt zwei dominante Konten |
| 3 | Issue-Orchestrator | 4,5 | Ja: Coding/Review/Rework-Steuerung | Commitgebundene Validierung | Early beta, nahezu ein Hauptautor |
| 4 | Sandcastle | 4 | Ja: TypeScript, separate Agentenaufrufe | Explizit selbst einfügen | Aktivitätspause, Hauptmaintainer |
| 5 | On-Par Software Factory | 4 | Ja: PLAN/BUILD/CHECK/SHIP | Checker/CI, konkrete Konfiguration prüfen | Erst seit Juli, praktisch ein Hauptautor |
| 6 | Ralph Orchestrator | 3,5 | Ja: YAML-Hats/Ereignis-Runtime | Ereignis allein reicht nicht; Hook-Gate gesondert prüfen | Über ein Jahr, externe PRs; konzentriert |
| 7 | Gas Town | 3 | Ja: Formulas/Molecules | Agentenschritte nicht pauschal Exit-Gates | Mehrere Monate; Betrieb und Merge-Modell komplex |
| 8 | iradulovic/software-factory | 3,5 | Ja: C#-Pipeline | Validierungsstufen, End-to-End-Garantie offen | Sehr jung, September/Oktober, ein Hauptautor |
| 9 | Tencent LoopForge | 3 | Ja: Nodes/Edges und Scheduler | Nicht abschließend geprüft | Seit Juli; Lizenz separat klären |
| 10 | Orchestrator.inc | 2,5 | Fester gewünschter Coding-DAG nicht belegt | CI-Reaktionen sind kein Beleg für diesen DAG | Kontinuierliche Aktivität, mehrere Konten; funktional bedingt |

### 1. TAKT

[builtins/en/workflows/default.yaml](https://github.com/nrslib/takt/blob/72916b6d7b50a8efe49f3841945458092d0c8672/builtins/en/workflows/default.yaml) und [src/core/workflow/quality-gates/commandGateRunner.ts](https://github.com/nrslib/takt/blob/72916b6d7b50a8efe49f3841945458092d0c8672/src/core/workflow/quality-gates/commandGateRunner.ts) zeigen tatsächliche Workflow-Aufrufe und deterministische Command-Gates: Erfolg verlangt Exitcode 0, keinen Timeout und keinen Ausführungsfehler. Das ist wesentlich stärker als ein Prompt mit Testanweisung. Die Standarddefinition delegiert an weitere versionierte Development-Workflows; nicht jede modellbewertete Regel ist dadurch deterministisch.

[README.md](https://github.com/nrslib/takt/blob/72916b6d7b50a8efe49f3841945458092d0c8672/README.md) dokumentiert Codex/Claude-Provider, Task-Queue, Worktrees und Skills/Facets. Achtung: SDK und CLI nicht gleichsetzen; ein Codex-Provider allein beweist keinen geprüften ChatGPT-Abonnement-Login. Manager-Freigaben und persistierte Eingaben existieren, aber die README warnt ausdrücklich, dass historische Approval-Antworten keine Ausführungsberechtigung definieren. Keine pauschale Zusage für menschliche Freigabe jeder Pipelinekante. Podman/Kubernetes und native GitHub-Blocker hier offen. Die vorhandenen Skills müssen in die Facets/Schritte eingebunden werden; kein automatischer Import aller persönlichen Skills behauptet.

### 2. Codev / Porch

[codev-skeleton/protocols/spir/protocol.json](https://github.com/cluesmith/codev/blob/4291dabb0eb7100b07059ecd67f907875bd31ec5/codev-skeleton/protocols/spir/protocol.json); [packages/codev/src/commands/porch/checks.ts](https://github.com/cluesmith/codev/blob/4291dabb0eb7100b07059ecd67f907875bd31ec5/packages/codev/src/commands/porch/checks.ts); [packages/codev/src/commands/porch/state.ts](https://github.com/cluesmith/codev/blob/4291dabb0eb7100b07059ecd67f907875bd31ec5/packages/codev/src/commands/porch/state.ts).

SPIR beschreibt Specify→Plan→Implement→Review, getrennte Modellreviews, begrenzte Wiederholungen, explizite Spec-/Plan-Approval-Gates und eigene Build-/Test-Kommandos. Der Check-Runner startet Prozesse und behandelt Timeouts einschließlich Prozessgruppe. Das ist ein echter Workflow-Pass. Zustandsdateien sind mehr als bloßer Chatverlauf; Crash-/Side-Effect-Wiederaufnahme dennoch nicht praktisch getestet. Das Protokoll führt bereits Commits und Pushes aus: vor Verwendung den gesamten ausgewählten Protokollpfad einschließlich PR/Merge-Verhalten lesen. Native GitHub-Issue-Blocker, Podman/Kubernetes und Abonnement-Refresh nicht bestätigt. Die CLI-/Modellintegration ist nicht automatisch eine Sandbox-Integration.

### 3. Issue-Orchestrator

[src/issue_orchestrator/control/workflows/review_workflow.py](https://github.com/issue-orchestrator/issue-orchestrator/blob/d4a79824ccd806089cf3f70ec632d39b8ee255ea/src/issue_orchestrator/control/workflows/review_workflow.py); [src/issue_orchestrator/control/validation.py](https://github.com/issue-orchestrator/issue-orchestrator/blob/d4a79824ccd806089cf3f70ec632d39b8ee255ea/src/issue_orchestrator/control/validation.py); [docs/user/tutorial.md](https://github.com/issue-orchestrator/issue-orchestrator/blob/d4a79824ccd806089cf3f70ec632d39b8ee255ea/docs/user/tutorial.md); [README.md](https://github.com/issue-orchestrator/issue-orchestrator/blob/d4a79824ccd806089cf3f70ec632d39b8ee255ea/README.md).

Coding-, Reviewer-, Rework- und Tech-Lead-Sessions werden außerhalb des Agenten gesteuert. ValidationRunner führt Kommandos aus; Records werden nach Suite und HEAD-SHA gespeichert. GitHub-Labels und beobachteter Worktree-Zustand bilden Wiederanlaufgrundlagen. Das Projekt bezeichnet sich selbst als Early beta. Menschlicher Merge ist ausdrücklich vorgesehen; der Tech-Lead-Agent ist dabei nicht mit menschlicher Approval gleichzusetzen.

Dependencies werden unter anderem als maschinenlesbare `Depends-on: #42` oder symbolische IDs im Issue-Text ausgedrückt. Das ist ein echter Parser-/Scheduler-Vertrag, aber **kein Nachweis für GitHubs native blocked-by API**. Eigener GitHub-Token ist dokumentiert. Codex/Claude-Anmeldung im konkreten isolierten Container, Podman und Kubernetes bleiben offen.

### 4. Sandcastle als Referenz

[src/templates/sequential-reviewer/main.mts](https://github.com/mattpocock/sandcastle/blob/e5b2d4e44e27f7cf8a122520e4d81999c91ef6cd/src/templates/sequential-reviewer/main.mts); [docs/guide/run.md](https://github.com/mattpocock/sandcastle/blob/e5b2d4e44e27f7cf8a122520e4d81999c91ef6cd/docs/guide/run.md); [docs/guide/sandbox-providers.md](https://github.com/mattpocock/sandcastle/blob/e5b2d4e44e27f7cf8a122520e4d81999c91ef6cd/docs/guide/sandbox-providers.md); [docs/guide/agent-providers.md](https://github.com/mattpocock/sandcastle/blob/e5b2d4e44e27f7cf8a122520e4d81999c91ef6cd/docs/guide/agent-providers.md).

Getrennte Implementierungs-/Review-Aufrufe sind echter TypeScript-Kontrollfluss. Die Vorlage stoppt bei fehlenden Commits; das beweist weder erledigte Issues noch erfolgreiche Tests. Die Run-Dokumentation zeigt `sandbox.exec` und Exitcode-Prüfung: eigener Testschritt ist möglich, muss aber tatsächlich eingefügt werden, auch nach einem verändernden Review. Podman ist dokumentiert; Kubernetes nicht bestätigt. Claude-OAuth und Codex-Provider nicht mit vollständig geprüftem Codex-ChatGPT-Refresh in Podman gleichsetzen. Native dauerhafte Pipeline-Approvals, GitHub-Abhängigkeitsscheduler und durable Pipeline-Resume bleiben DIY/offen. Skills können als Dateien/Prompt-Kontext bereitgestellt werden.

### 5. On-Par

[README.md](https://github.com/on-par/software-factory/blob/d267b568670ba8f0d166e0390bba84721be1dcad/README.md); [packages/core/src/phases/ship-approval.ts](https://github.com/on-par/software-factory/blob/d267b568670ba8f0d166e0390bba84721be1dcad/packages/core/src/phases/ship-approval.ts); [docs/agents.md](https://github.com/on-par/software-factory/blob/d267b568670ba8f0d166e0390bba84721be1dcad/docs/agents.md).

Die lokale Factory hat PLAN→BUILD→CHECK→SHIP, getrennte Worktrees, Claude/Codex, angemeldete CLIs, GitHub-Token und manuelles Mergen als Standard. Das ist besonders nah am gewünschten Produkt. CI unbekannt oder fehlgeschlagen wird laut dokumentiertem Vertrag nicht als bestanden behandelt. **Der Actions-Wrapper aktiviert dagegen Auto-Merge**; nicht ungeprüft übernehmen. OS-Sandbox/firejail und lesbare CLI-Auth-Verzeichnisse sind dokumentiert, kein Beweis für Podman/Kubernetes. Queue-/Merge-Wait ist nicht automatisch native GitHub-Blocker-Unterstützung. Erst vier Monate Historie; hohe Commitzahl ersetzt keine breite Community.

### 6. Ralph Orchestrator

Korrekter untersuchter Ursprung: **mikeyobrien/ralph-orchestrator**, nicht die vielen gleichnamigen Forks. 
[presets/review.yml](https://github.com/mikeyobrien/ralph-orchestrator/blob/1f8784b0900ef12436cf1b82160dc8efaef1116c/presets/review.yml); [crates/ralph-core/src/hooks/engine.rs](https://github.com/mikeyobrien/ralph-orchestrator/blob/1f8784b0900ef12436cf1b82160dc8efaef1116c/crates/ralph-core/src/hooks/engine.rs); [crates/ralph-adapters/src/cli_backend.rs](https://github.com/mikeyobrien/ralph-orchestrator/blob/1f8784b0900ef12436cf1b82160dc8efaef1116c/crates/ralph-adapters/src/cli_backend.rs).

YAML definiert Hats, Trigger, publizierte Events, Iterations-/Zeitgrenzen; Rust führt die Schleife und CLI-Backends aus. Workflow-Pass, aber `required_events` und Completion-Promise sind Agentenmeldungen, keine unabhängigen Testergebnisse. Lifecycle-Hooks sind vorhanden; deren konkreten Blocking-Vertrag vor einer Testgarantie verifizieren. Nicht aus „Backpressure“ auf automatisch erzwungene Tests schließen. Tasks/Events nicht mit GitHub-Issue-Abhängigkeiten gleichsetzen; native Human-Approval und crashfeste ganze Pipeline hier offen.

### 7–10. Weitere echte Kandidaten mit Vorbehalten

- **Gas Town:** [internal/formula/formulas/mol-polecat-work.formula.toml](https://github.com/gastownhall/gastown/blob/94cc82289d2ead275a62e1db4ef1d4ada4a1f98e/internal/formula/formulas/mol-polecat-work.formula.toml) beschreibt Coding-Schritte als Formula. Molecule-/Beads-Abhängigkeiten sind ein eigenes Taskmodell. Nicht jede Anweisung in einer Formula wird als unabhängiger Prozess erzwungen. Refinery/Merge-Automation passt nicht ungeprüft zum ausschließlich manuellen Merge. Hoher Integrations- und Betriebsumfang.
- **iradulovic/software-factory:** [src/Factory.Orchestrator/Pipeline.cs](https://github.com/iradulovic/software-factory/blob/b731b22a765ba3964f2657f05828c420d4e33ede/src/Factory.Orchestrator/Pipeline.cs) besitzt explizite Outcomes Succeeded/Failed/NeedsHuman/WaitingForQuota, Step-Interface, Review-Kontext und Persistenz über TaskStore. Echte Coding-Pipeline, aber wenige Wochen substantieller beobachteter Historie, kein Release: Forschungs-/Evaluationskandidat, keine etablierte Empfehlung.
- **Tencent LoopForge:** [internal/engine/workflow_graph.go](https://github.com/Tencent/LoopForge/blob/8a8fcd2383834e24848c9d3adea264dbb5072317/internal/engine/workflow_graph.go) schedult nur pending Nodes, deren Vorgänger succeeded sind, und erhält einen Workflow-Snapshot pro Run. Das ist echte Graphsteuerung. Approval-/Recovery-Dateien existieren, aber deren vollständige Garantien wurden nicht geprüft. GitHub meldet Lizenz NOASSERTION: vor Aufnahme als OSS-Volltreffer Lizenztext separat juristisch/fachlich prüfen.
- **Orchestrator.inc**, früher ComposioHQ/agent-orchestrator: [README.md](https://github.com/OrchestratorInc/agent-orchestrator/blob/c5ef1ae0231c77528261c99d42e7be98e2ebda32/README.md) beschreibt inzwischen Desktop-Workspace, Worker, Worktrees und Rückrouting von CI-/Review-Feedback. Die frühere Produktbeschreibung darf nicht auf den aktuellen Code übertragen werden. Kein ausreichender Nachweis des gewünschten frei definierten Implement→Test→Review-DAG; deshalb nur bedingter Platz 10, kein Hard-Requirement-Pass.

## Offene Integrationspunkte für alle Kandidaten

Keine dieser statischen Prüfungen bestätigt den vollständigen Betrieb mit Gerds privaten Repositories. Ein Bot-PAT muss sowohl Lesen/Schreiben als auch Token-Abschirmung gegenüber Agenten korrekt abdecken. Worktrees sind keine Sicherheits-Sandbox. Podman-Kompatibilität ist nur bei Sandcastle hier konkret dokumentiert; Containerfähigkeit bedeutet nicht nativer Kubernetes-Scheduler.

Die Skills `implement`, `tdd`, `code-review`, `wayfinder`, `grilling` brauchen eine explizite Zuordnung zu Schritten. Skill-Dateien können Verhalten steuern, aber weder menschliche Freigaben noch Test-Exitcodes ersetzen. Für `grilling` muss ein echter wartender Menschenschritt verfügbar oder implementiert sein. Keine native Integration dieser konkreten persönlichen Skills behauptet.

## Wartung: reproduzierbare Primärdaten

GitHub-API neu abgefragt: Metadaten, Latest-Release, Tags, letzte 100 Default-Branch-Commits, letzte 50 geschlossene PRs; für die zehn Kandidaten zusätzlich April/Juli/Oktober jeweils 1.–28., maximal 100 Commits. Zahlen sind Stichproben, keine vollständigen Monatsstatistiken. Commit-Meldungen wurden auf Implementierungsaktivität geprüft; keine Garantie menschlicher Autorschaft. Konten wie claude, Copilot, all-hands-bot, rookopenclaw oder *[bot] werden **nicht** als nachgewiesene unabhängige Menschen gezählt. Auch author_association=CONTRIBUTOR beweist keine Unabhängigkeit: patrob ist etwa der dominante On-Par-Autor.

Alle folgenden Default-Branch-Funktionsbelege sind auf den tatsächlich gelesenen Clone-Commit gepinnt. API-Daten und Clone können während aktiver Entwicklung geringfügig auseinanderliegen. Kein Release wurde ausgeführt; **Main-Funktionen nicht automatisch als Bestandteil von Latest-Release verstehen**. Besonders TAKT: Tags trotz fehlender Latest-Release-Antwort; On-Par: Tags weiter als Latest.

| Projekt | Erstellt / archiviert | Latest Release / Datum; neuester Tag | April / Juli / Oktober Commits | Häufigste Konten letzte 100 |
|---|---|---|---|---|
| [nrslib/takt](https://api.github.com/repos/nrslib/takt) | 2026-01-25 / False | keine Latest-Antwort / —; v0.75.0 | 100 / 100 / 100 | nrslib: 97, zonuexe: 1, KurisuWataru: 1 |

| [cluesmith/codev](https://api.github.com/repos/cluesmith/codev) | 2025-09-19 / False | v3.3.7 / 2026-11-03; v3.3.7 | 100 / 100 / 100 | amrmelsayed: 51, waleedkadous: 49 |

| [issue-orchestrator/issue-orchestrator](https://api.github.com/repos/issue-orchestrator/issue-orchestrator) | 2025-12-04 / False | v0.10.0 / 2026-06-09; v0.10.0 | 100 / 100 / 4 | BruceBGordon: 98, issue-orchestrator-bot[bot]: 2 |

| [mattpocock/sandcastle](https://api.github.com/repos/mattpocock/sandcastle) | 2026-03-17 / False | v0.14.0 / 2026-11-02; v0.14.0 | 100 / 0 / 24 | mattpocock: 91, nicht zugeordnet: 7, kuLm1Fs: 2 |

| [on-par/software-factory](https://api.github.com/repos/on-par/software-factory) | 2026-07-09 / False | v0.9.28 / 2026-11-05; v0.9.36 | 0 / 100 / 100 | patrob: 96, dependabot[bot]: 4 |

| [mikeyobrien/ralph-orchestrator](https://api.github.com/repos/mikeyobrien/ralph-orchestrator) | 2025-09-07 / False | v2.11.0 / 2026-11-13; v2.11.0 | 15 / 4 / 5 | mikeyobrien: 64, rookopenclaw: 13, cdriscol: 3 |

| [gastownhall/gastown](https://api.github.com/repos/gastownhall/gastown) | 2025-12-16 / False | v1.2.1 / 2026-06-06; v1.2.1 | 100 / 100 / 8 | Bella-Giraffety: 50, Thalia-geraghty: 31, julianknutsen: 14 |

| [iradulovic/software-factory](https://api.github.com/repos/iradulovic/software-factory) | 2026-09-17 / False | keine Latest-Antwort / —; kein Tag | 0 / 0 / 1 | iradulovic: 87, nicht zugeordnet: 13 |

| [Tencent/LoopForge](https://api.github.com/repos/Tencent/LoopForge) | 2026-07-28 / False | v0.3.8 / 2026-11-12; v0.3.8 | 0 / 1 / 8 | fx-kurumi: 96, crispinhu27: 2, cocofhu: 1 |

| [OrchestratorInc/agent-orchestrator](https://api.github.com/repos/OrchestratorInc/agent-orchestrator) | 2026-02-13 / False | v0.14.10 / 2026-11-06; v0.14.11-nightly.202611130904 | 100 / 100 / 100 | ronishrohan: 32, illegalcall: 23, Prasad-D-Ware: 15 |

### Externe Beiträge und Grenzen der Community-Aussage

- **nrslib/takt:** [PR #2028 (zonuexe)](https://github.com/nrslib/takt/pull/2028) [Commits](https://api.github.com/repos/nrslib/takt/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/nrslib/takt/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/nrslib/takt/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **cluesmith/codev:** [PR #1889 (waleedkadous)](https://github.com/cluesmith/codev/pull/1889), [PR #1888 (waleedkadous)](https://github.com/cluesmith/codev/pull/1888), [PR #1884 (waleedkadous)](https://github.com/cluesmith/codev/pull/1884) [Commits](https://api.github.com/repos/cluesmith/codev/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/cluesmith/codev/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/cluesmith/codev/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **issue-orchestrator/issue-orchestrator:** Keine belastbare externe gemergte PR in dieser begrenzten Stichprobe isoliert; das beweist nicht deren Abwesenheit. [Commits](https://api.github.com/repos/issue-orchestrator/issue-orchestrator/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/issue-orchestrator/issue-orchestrator/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/issue-orchestrator/issue-orchestrator/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **mattpocock/sandcastle:** Keine belastbare externe gemergte PR in dieser begrenzten Stichprobe isoliert; das beweist nicht deren Abwesenheit. [Commits](https://api.github.com/repos/mattpocock/sandcastle/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/mattpocock/sandcastle/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/mattpocock/sandcastle/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **on-par/software-factory:** Keine belastbare externe gemergte PR in dieser begrenzten Stichprobe isoliert; das beweist nicht deren Abwesenheit. [Commits](https://api.github.com/repos/on-par/software-factory/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/on-par/software-factory/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/on-par/software-factory/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **mikeyobrien/ralph-orchestrator:** [PR #396 (dannyward630)](https://github.com/mikeyobrien/ralph-orchestrator/pull/396), [PR #379 (varun1505)](https://github.com/mikeyobrien/ralph-orchestrator/pull/379), [PR #373 (caniko)](https://github.com/mikeyobrien/ralph-orchestrator/pull/373) [Commits](https://api.github.com/repos/mikeyobrien/ralph-orchestrator/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/mikeyobrien/ralph-orchestrator/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/mikeyobrien/ralph-orchestrator/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **gastownhall/gastown:** Keine belastbare externe gemergte PR in dieser begrenzten Stichprobe isoliert; das beweist nicht deren Abwesenheit. [Commits](https://api.github.com/repos/gastownhall/gastown/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/gastownhall/gastown/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/gastownhall/gastown/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **iradulovic/software-factory:** Keine belastbare externe gemergte PR in dieser begrenzten Stichprobe isoliert; das beweist nicht deren Abwesenheit. [Commits](https://api.github.com/repos/iradulovic/software-factory/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/iradulovic/software-factory/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/iradulovic/software-factory/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **Tencent/LoopForge:** Keine belastbare externe gemergte PR in dieser begrenzten Stichprobe isoliert; das beweist nicht deren Abwesenheit. [Commits](https://api.github.com/repos/Tencent/LoopForge/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/Tencent/LoopForge/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/Tencent/LoopForge/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).
- **OrchestratorInc/agent-orchestrator:** Keine belastbare externe gemergte PR in dieser begrenzten Stichprobe isoliert; das beweist nicht deren Abwesenheit. [Commits](https://api.github.com/repos/OrchestratorInc/agent-orchestrator/commits?per_page=100), [PR-Stichprobe](https://api.github.com/repos/OrchestratorInc/agent-orchestrator/pulls?state=closed&per_page=50), [Monatsbeispiel](https://api.github.com/repos/OrchestratorInc/agent-orchestrator/commits?per_page=100&since=2026-07-01T00:00:00Z&until=2026-07-28T23:59:59Z).

Die PR-Beispiele sind Nachweise zusammengeführter Beiträge, keine Behauptung eines breiten unabhängigen Maintainer-Teams. Für TAKT und Ralph sind solche Beiträge sichtbar; Issue-Orchestrator und On-Par bleiben klar konzentriert. Auch Codevs zwei dominante Konten garantieren keinen hohen Bus-Faktor.

## Breites Screening und Ausschlüsse

Die Suche umfasste GitHub-Repository-Suche und neue Websuchen zu Coding-Orchestrator, Codex/Claude-Workflow, Worktrees, Ralph und Codev. Generische Suchtreffer und Namenskollisionen wurden entfernt. Die folgenden zusätzlichen Coding-Repositories wurden anhand neu gelesener API-Metadaten, README und Dateibaum gescreent, nicht alle tief im Code auditiert:

- [openai/symphony](https://github.com/openai/symphony) ([Metadaten](https://api.github.com/repos/openai/symphony)): WORKFLOW.md plus Tracker/Agent-Schleife ist kein ausreichender Beleg für getrennte Implement/Test/Review-Kanten.
- [AndyMik90/Aperant](https://github.com/AndyMik90/Aperant) ([Metadaten](https://api.github.com/repos/AndyMik90/Aperant)): Auto-Claude wird hierhin umgeleitet; letzter beobachteter Default-Branch-Commit 14.06.2026. Nicht ungeprüft als aktuell gepflegtes Auto-Claude empfehlen.
- [BloopAI/vibe-kanban](https://github.com/BloopAI/vibe-kanban) ([Metadaten](https://api.github.com/repos/BloopAI/vibe-kanban)): Coding-Workspace/Session- und Approval-Verwaltung; gewünschter ausführbarer Mehrschritt-DAG nicht belegt.
- [automazeio/ccpm](https://github.com/automazeio/ccpm) ([Metadaten](https://api.github.com/repos/automazeio/ccpm)): Commands/Prompts und Projektmanagement; keine ausreichende ausführbare unabhängige Pipeline nachgewiesen.
- [ruvnet/ruflo](https://github.com/ruvnet/ruflo) ([Metadaten](https://api.github.com/repos/ruvnet/ruflo)): Sehr großer Coding-/Swarm-Baukasten; Workflow-Dateien vorhanden, aber gewünschter verbindlicher CLI/Test/Approval-Vertrag nicht tief genug verifiziert. Nicht pauschal prompt-only.
- [vercel-labs/eve-software-factory-template](https://github.com/vercel-labs/eve-software-factory-template) ([Metadaten](https://api.github.com/repos/vercel-labs/eve-software-factory-template)): Junges Template seit August; Eval-Pipeline und Write-Approval belegen nicht automatisch Produktionspipeline und breite Pflege.
- [theninjadojo/software-factory](https://github.com/theninjadojo/software-factory) ([Metadaten](https://api.github.com/repos/theninjadojo/software-factory)): Erstellt Oktober 2026; hohe Release-/PR-Zahlen sind noch kein Langzeitnachweis.
- [cisco-open/repository-agent-orchestrator](https://github.com/cisco-open/repository-agent-orchestrator) ([Metadaten](https://api.github.com/repos/cisco-open/repository-agent-orchestrator)): Coding/Review-Workflow vorhanden; dokumentiertes Auto-Merge nach Human-Review kollidiert mit gewünschtem manuellen Merge; erst August 2026.
- [mohamedwaleed/agent-orchestrator](https://github.com/mohamedwaleed/agent-orchestrator) ([Metadaten](https://api.github.com/repos/mohamedwaleed/agent-orchestrator)): Task-Wellen/Worktrees, Abhängigkeiten im Text; Approval als geplant dokumentiert, beobachtete Historie August 8.–9.
- [kastrah/orchestrator](https://github.com/kastrah/orchestrator) ([Metadaten](https://api.github.com/repos/kastrah/orchestrator)): Passender CLI-Orchestrierungsansatz, aber fester geprüfter Coding/Test-DAG und nachhaltige Community nicht bestätigt.
- [felipemelendez/llm-orchestrator](https://github.com/felipemelendez/llm-orchestrator) ([Metadaten](https://api.github.com/repos/felipemelendez/llm-orchestrator)): Claude-Plugin mit strukturierten Prompts; unabhängige ausführbare Übergänge nicht ausreichend belegt.
- [jakeparkcolde/agent-worktree-orchestrator](https://github.com/jakeparkcolde/agent-worktree-orchestrator) ([Metadaten](https://api.github.com/repos/jakeparkcolde/agent-worktree-orchestrator)): Worktree-Control-Plane; Policies allein erzwingen laut Projektbeschreibung keine Freigabe oder PR-Merges.
- [moazbuilds/CodeMachine-CLI](https://github.com/moazbuilds/CodeMachine-CLI) ([Metadaten](https://api.github.com/repos/moazbuilds/CodeMachine-CLI)): Echte Workflow-Runtime als weiterer Prüfkandidat; letzter beobachteter Commit Februar 2026, daher keine aktuelle Pflegeempfehlung.
- [stellarlinkco/myclaude](https://github.com/stellarlinkco/myclaude) ([Metadaten](https://api.github.com/repos/stellarlinkco/myclaude)): Coding-Workflow-/Agentensammlung; letzte beobachtete Aktivität Mai 2026, harter Vertrag nicht bestätigt.
- [gastownhall/gascity](https://github.com/gastownhall/gascity) ([Metadaten](https://api.github.com/repos/gastownhall/gascity)): Weiterer Formula/Molecule-Control-Plane-Kandidat; getrennt von Gas Town, botdominierte aktuelle PR-Stichprobe. Nicht als zusätzlicher etablierter Treffer gezählt.
- [iYoungblood/agentforge](https://github.com/iYoungblood/agentforge): Abruf lieferte keinen verwertbaren Repository-/README-Nachweis; unaufgelöst, nicht als geprüfter Treffer zählen.

Damit wurden **25 benennbare Coding-Projekte** in engerem Screening betrachtet, zusätzlich der unaufgelöste Agentforge-Seed; zehn vertieft geklont. Nicht jede ausgeschlossene Lösung ist ungeeignet: fehlender Nachweis ist ausdrücklich kein negativer Funktionsbeweis.

## Nächster sinnvoller Prüfvertrag

Eine spätere, separat beauftragte Evaluation sollte TAKT, Codev und Issue-Orchestrator gegen denselben kleinen Test prüfen: Mensch startet ein konkretes Issue; Implementierung läuft separat; ein absichtlich fehlschlagender Test sperrt Review/Publikation; Reviewänderungen lösen erneute Tests aus; Neustart während Human-Wartezustand erhält den Pipelinezustand; PR bleibt ungemergt; offener Vorgänger blockiert den Nachfolger nach eindeutig festgelegter Semantik. Zusätzlich Codex-Abonnement mit Token-Refresh und gewünschter Sandbox prüfen. Für den kleinsten Podman-Prototyp ist Sandcastle die Alternative, allerdings mit mehr eigener Workflow-Logik.

Diese Recherche liefert eine wesentlich passendere Kandidatenklasse, aber keine unbelegte Zusicherung von zehn reifen Communities oder vollständig fertigen Integrationen.
