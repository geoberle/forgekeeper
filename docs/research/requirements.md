# Anforderungen an die Software Factory

Festgehalten am 13. November 2026 aus Gerds Auswahlkriterien.

## Pflicht

- Ein versionierter Workflow in Code oder Konfiguration gibt Schritte und erlaubte Übergänge vor. LLMs dürfen Entscheidungen innerhalb dieses Ablaufs treffen, aber nicht den Workflow selbst frei bestimmen oder verändern.
- Agentenschritte führen einen konfigurierten Prompt aus, insbesondere mit Codex und dem vorhandenen `implement`-Skill.
- LLM-Entscheidungen können zwischen definierten Pfaden wählen. Beispiel: kleiner Change zum Merge-Pfad, großer Change zur menschlichen Freigabe.
- Agentenfreie Schritte führen Tests, Linting und andere Kommandos aus. Konkrete Ergebnisse steuern Übergänge, beispielsweise fehlgeschlagene Tests zurück zur Implementierung.
- Der Ablauf ist flexibel und erweiterbar.
- Ein GitHub-Issue kann mehrere unabhängig geprüfte und gemergte PR-Inkremente umfassen (Beispiel: pi5-eeprom-probe #15). Ein erfolgreicher Run oder gemergter Teil-PR schließt das Gesamt-Issue nicht automatisch. Teil-PRs referenzieren das Issue ohne Closing-Keywords; ein vollständig abschließender PR darf ein Closing-Keyword verwenden. Der tatsächliche Issue-Zustand steuert erneute Auswahl und Abhängigkeiten.

## Bonus

- Ausführung mit OpenShell.
- Betrieb auf Kubernetes.
- Einzelne Workflow-Schritte als separate Pods starten.
- Über eine UI in Workspaces einsteigen und Fehler untersuchen.

## Weiterhin relevante Integrationsfragen

- GitHub-Issues und deren Abhängigkeiten, PR-Erstellung und kontrollierter Start.
- Vorhandene persönliche Skills und Agents.
- Codex-Abonnement-Authentifizierung und GitHub-Bot-PAT bei abgeschirmtem Host-Zugriff.
- Langfristige Wartung und Community.

Gerd mergt jeden PR selbst. Das Issue-Label autorisiert die Bearbeitung: `ready-for-test` oder `ready-for-agent`. Solange es gesetzt bleibt, darf nach einem gemergten Teil-PR automatisch das nächste Inkrement starten, sofern die übrigen Startbedingungen erfüllt sind. Ein Run endet nach Erstellung eines geprüften PRs. Ein weiterhin offenes Issue kann nach dem manuellen Merge erneut unabhängig ausgewählt werden; keine besondere Inkrement- oder Fortsetzungsverwaltung erforderlich. Pro Repository und Issue sind höchstens ein nicht beendeter Fabro-Run und ein zugehöriger offener PR erlaubt; beide dürfen gleichzeitig zum selben Inkrement gehören. Eine zusätzliche Issue-Freigabe gibt es nicht. Das Beispiel einer automatischen Merge-Entscheidung bleibt eine mögliche Fähigkeit, nicht die gewählte Betriebsregel.
