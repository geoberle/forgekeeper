# Issue-Orchestrierung

Die Orchestrierung wählt freigegebene GitHub-Issues zur Bearbeitung aus. Ein Workflow bearbeitet jeweils einen Beitrag bis zum offenen Pull Request.

## Language

**Freigegebenes Issue**:
Ein offenes Issue mit dem für das Projekt vorgesehenen Bereitschaftslabel. Offene Abhängigkeiten oder ein bereits zugeordneter offener PR können seine Bearbeitung weiterhin verhindern.

**Run**:
Eine einzelne Ausführung eines Workflows für ein Issue. Sie endet nach der PR-Erstellung oder bei einem Abbruch; sie wartet nicht auf den Merge.

**Teilbeitrag**:
Eine unabhängig prüfbare Änderung, die einen Teil eines Issues erfüllt. Ihr Merge schließt das Issue nicht zwingend ab.
_Avoid_: Inkrement mit eigenem Lebenszyklus

**Orchestrierung**:
Die Auswahl bearbeitbarer Issues und Koordination ihrer Runs. Die einzelnen Implementierungs-, Prüf- und PR-Schritte gehören zum Workflow.

**Retry-Freigabe**:
Die Erlaubnis für genau einen neuen Bearbeitungsversuch nach einem bestimmten fehlgeschlagenen Run. Sie hebt andere Voraussetzungen für die Bearbeitung nicht auf.
