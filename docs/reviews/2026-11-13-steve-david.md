# Code-Review: Steve und David

Review des aktuellen Working Trees, einschließlich Python-Starter, Workflow-Prompts, Container und CI. Die Go-Orchestrierung ist noch nicht implementiert. Zwei unabhängige Reviews; keine Änderungen durch die Reviewer. Die ausdrücklich akzeptierte Create-/Listen-Sichtbarkeitslücke ist kein Befund.

## David

### P1: Credentials können in Prüf-Logs erscheinen

`scripts/issue-workflow.py:311` vererbt Projektprüfungen die vollständige Umgebung und speichert ihre Ausgabe unverändert. Bei einem Fehler wird diese Ausgabe zusätzlich in den Fabro-Step-Output kopiert. Eine Umgebungsdiagnose kann dadurch beispielsweise `FABRO_DEV_TOKEN` und `SESSION_SECRET` veröffentlichen. Mit einem künstlichen Token reproduziert; keine echten Secrets gelesen.

Vorschlag: Prüfprozessen keine unnötigen Credentials vererben und bekannte Secret-Werte vor Speicherung oder Weitergabe entfernen. Das allein ist keine Härtung gegen absichtlich bösartige Prozesse im gemeinsamen Container.

### P2: Finale Freigabeprüfung berücksichtigt Planungslabel nicht

`workflows/implement-issue/prompts/pr.md:3` verlangt offene Bereitschaft, geschlossene Blocker und keinen konkurrierenden PR, aber nicht den Ausschluss von `wayfinder:*`. Wird ein solches Label während der Implementierung ergänzt, kann der Agent gemäß Prompt trotzdem veröffentlichen.

Vorschlag: Den Planungslabel-Ausschluss auch in den finalen Agent-Prüfungen nennen. Kein zusätzlicher Publish-Helper erforderlich. David bewertet dies als blockierend für den vereinbarten Eligibility-Vertrag.

## Steve

### P1: Unterbrochene PR-Erstellung lässt sich nicht vollständig abgleichen

`workflows/implement-issue/prompts/pr.md:5` öffnet den PR vor dem Schreiben von `pr.json`. `scripts/issue-workflow.py:225` prüft einen früheren PR nur über diese gespeicherte Zuordnung oder einen alten `publishing.json`-Marker; der aktuelle PR-Schritt schreibt diesen Marker nicht mehr. Bricht der Agent nach der Erstellung ab und wird der PR anschließend ohne Merge geschlossen, kann ein neuer Run zugelassen werden.

Vorschlag: Den früheren Branch/PR auf GitHub abgleichen, statt einen neuen Publish-Helper einzuführen. David hat diesen Befund nicht aufgenommen, weil die README die nötige manuelle Klärung bei fehlendem `pr.json` bereits als Grenze dokumentiert. Der automatische Schutz ist in diesem Szenario dennoch unvollständig.

### P2: Unsauberer Workspace überspringt Reparatur

`scripts/issue-workflow.py:300` ruft `clean()` vor Initialisierung von `attempt.json` und `verify.log` auf. Schlägt das beim ersten Verify fehl, scheitert `retry` an der fehlenden Zählerdatei, statt den Reparatur-Agenten aufzurufen. Mit einem simulierten Dirty-Workspace-Fehler reproduziert.

Vorschlag: Versuchszähler und Diagnose vor den fehlschlagbaren Prüfungen initialisieren und den Fall als Regressionstest aufnehmen.

### P2: Planungslabel fehlt bei Veröffentlichung

Derselbe Befund wie bei David: `workflows/implement-issue/prompts/pr.md:3`.

## Umsetzung und Validierung

Die vier Befunde wurden nach Freigabe umgesetzt:

- Prüfprozesse erhalten ausschließlich grundlegende Prozessvariablen (PATH, HOME, Benutzer, temporäre Pfade, Locale und Zeitzone). Token-, Provider-, Shell-Startup- und Git-Credential-Variablen werden nicht vererbt.
- Bekannte Secret-Werte aus der Umgebung, dem Bot-PAT und Codex-Auth werden vor Speicherung der Check-Kommandos und ihrer Ausgabe maskiert. Ausgabe wird im Speicher gesammelt und erst nach Maskierung geschrieben. Das ist keine Isolation gegen Prozesse, die im gemeinsamen Container andere Credential-Dateien lesen oder Secrets transformieren.
- Versuchszähler und Log entstehen vor dem Clean-Check; ein Dirty-Workspace-Fehler liefert Diagnosen und erlaubt den vorgesehenen Repair-Schritt.
- Der finale PR-Prompt schließt `wayfinder:*` aus.
- Ohne `pr.json` gleicht der Starter den vorherigen Branch mit sämtlichen GitHub-PRs ab. Offene und ungemergt geschlossene PRs blockieren; ein gemergter PR erlaubt die normale erneute Eligibility-Prüfung. API-Fehler blockieren.

`make verify` besteht mit 21 Tests. Neue Regressionstests prüfen künstliche Secrets auf Erfolgs- und Fehlerpfaden, die initiale Dirty-Workspace-Reparatur und die Wiedererkennung ohne gespeicherte PR-Metadaten. Keine echten Credentials ausgegeben, keine GitHub-Schreiboperationen oder neuen Implementierungsruns ausgeführt. Vorhandene historische Logs wurden nicht geändert.
