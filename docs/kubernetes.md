# Kubernetes

Das [Helm-Chart](../charts/factory) betreibt Fabro als StatefulSet und die
Go-Orchestrierung als separates Deployment mit `Recreate`. Beide haben genau eine
Replik. Fabro führt die Agent- und Testschritte in seinem eigenen Pod aus; einzelne
Workflow-Schritte bekommen derzeit keine eigenen Pods.

Fabros Datenbank, Provider-Konfiguration, Codex-Login und Run-Daten liegen auf einem
privaten RWO-Volume unter `/storage`. Nur `/storage/issue-jobs` wird über ein zweites
Workspace-Volume zwischen den Pods geteilt (RWX über mehrere Nodes oder RWO
bei gemeinsamer Platzierung auf einem Node). Der Orchestrator hält keinen eigenen
dauerhaften Steuerungszustand. Sein HOME liegt auf `emptyDir`.

## Image und Skills

Das derzeit gepinnte Fabro-Basisimage ist **ARM64**. Die Action baut auf
`ubuntu-24.04-arm`, und das Chart wählt ARM64-Nodes aus. Das passt zum lokalen
Kind-Cluster auf Apple Silicon; AMD64 ist mit diesem Image nicht unterstützt.
GitHub dokumentiert die ARM-Runner in seiner
[Runner-Referenz](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).

Die [Build-Action](../.github/workflows/image.yml) verwendet
`geoberle/skillshare-config` mit dem Commit aus `container/skills-ref`.
Sie übernimmt `library/` einschließlich Ressourcen und `agents/`. Konfiguration,
Git-Metadaten und Host-Anmeldungen werden nicht übernommen. Das ergibt derzeit
39 Skills, einschließlich `implement`, `pr`, `tdd` und `code-review`, sowie die
Reviewer-Agents. Native Plugin-Inhalte und accountgebundene Codex-Dienste sind
nicht Teil dieses Repositories und damit nicht Teil dieses CI-Bundles.

Bei einem öffentlichen Skillshare-Repository ist kein zusätzliches Lesesecret nötig.
Solange es privat ist, im Factory-Repository **`SKILLS_READ_TOKEN`** hinterlegen:
Fine-grained PAT, ausschließlich Repository `geoberle/skillshare-config`,
**Contents: Read**. Dieses Build-Secret wird weder an Docker übergeben noch
zur Laufzeit benötigt. Es ist unabhängig vom GitHub-Bot-PAT.

Pushes auf `main` bauen das Image und veröffentlichen es nach erfolgreichen
Image-Smoke-Tests automatisch in GHCR. Bei manuellen Läufen steuert weiterhin
`publish=true` die Veröffentlichung. Das Image heißt
`ghcr.io/<owner>/<factory-repository>:sha-<vollständiger-commit>`.
**Die GHCR-Packages bleiben privat.** Änderungen der Skills-Version brauchen einen neuen Image-Build. Builds aus Fork-PRs bekommen keinen Zugang zum
privaten Repo; die normale Verify-Action benötigt diesen Zugang nicht.

### Automatische Update-PRs

Die [Update-Action](../.github/workflows/update-skills.yml) prüft stündlich zur
Minute 17 `skillshare-config/main`; GitHub kann geplante Läufe verzögern. Sie ist
auch manuell startbar. Bei einem neuen Commit validiert sie das Bundle und
öffnet einen PR, der ausschließlich `container/skills-ref` ändert. Ein bestehender
PR auf `automation/update-skills` wird aktualisiert. Diesen Branch der Automation
überlassen. Ohne neue Änderungen entsteht kein weiterer PR.

Im Factory-Repository unter **Settings → Actions → General → Workflow permissions**
**Allow GitHub Actions to create and approve pull requests** aktivieren.
Die Action nutzt das repository-eigene `GITHUB_TOKEN`: `contents: write` und
`pull-requests: write` für den PR, `actions: write` für den ausdrücklichen Start
von `verify.yml` auf dem PR-Branch. Dafür ist kein persönlicher Schreib-PAT nötig.
Sie genehmigt und mergt keine PRs.

Auf automatisch ausgelöste PR-Workflows verlassen wir uns nicht; GitHub kann dafür
[zusätzliche Freigaben verlangen](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows).
Nach Erstellen oder Aktualisieren des PRs wird `Verify` per `workflow_dispatch`
gestartet. Bei einem Dispatch-Fehler die Verify-Action manuell für
`automation/update-skills` starten. Der Bundle-Check läuft bereits vor dem PR;
den Image-Build mit Veröffentlichung in GHCR löst erst dein Merge nach `main`
aus. Das Deployment bleibt manuell.

Die Workflows müssen zunächst auf `main` im Factory-Repository liegen. Diese
Vorbereitung erstellt weder einen GitHub-PR noch einen Zeitplan außerhalb von
GitHub Actions.

Lokal denselben Bundle-Modus verwenden:

```bash
./scripts/build.sh --skills-source /Users/goberlec/personal/skills
```

Ohne Argument bleibt der bisherige lokale Build aus den installierten Skills
und Plugin-Caches erhalten. Das Inventar liegt im Image unter
`/opt/experiment/bundle/skills-manifest.json`.

## Baseline-Chart und Installation

Forgekeeper veröffentlicht zwei OCI-Artefakte in GHCR:

- Container: `ghcr.io/geoberle/forgekeeper:sha-<commit>`
- Helm-Chart: `oci://ghcr.io/geoberle/forgekeeper-chart`, Version `0.1.0-sha.<commit>`

Die Action paketiert das Chart mit derselben Commit-ID wie das Image. Sie prüft
beides vor dem Publish und veröffentlicht zuerst das Image, danach das Chart.
Die konkrete Installation pinnt beide Versionen. Image-Tags bleiben explizit in
den Values; ein Chart-Upgrade wählt nicht stillschweigend ein anderes Image.

Das Chart enthält keine standortspezifischen Values oder Traefik-CRDs. Es bietet
Standard-Ingress, konfigurierbare Annotations, StorageClass, Workspace-Zugriffsmodus,
Node-Auswahl und Referenzen auf vorhandene Secrets. Für Pods auf verschiedenen
Nodes ist RWX nötig, bei gemeinsamer Platzierung kann RWO verwendet werden.

Die Hermine-Instanz wird im Repository `geoberle/hearth` unter
`forgekeeper/deploy/` verwaltet. Dort liegen Netzwerk- und Ressourcenwerte,
Image-/Chart-Pins sowie die Ansible-Einbindung für DNS, TLS-Annotations,
Traefik-Middleware und Secrets. Forgekeeper baut und veröffentlicht nur Artefakte;
ein Push führt kein Deployment aus.

## Installation

Benötigt werden Helm 3, persistenter Speicher und Zugang zum privaten Image.
Der Standard nutzt RWX für Workspaces auf mehreren Nodes. Bei gemeinsamer
Platzierung auf einem Node reicht RWO; die konkrete Installation legt dies fest. Das Server-Volume benötigt gewöhnlichen RWO-Block-/Dateispeicher;
die SQLite-Datenbank nicht auf das gemeinsame RWX-Volume verschieben.

```bash
kubectl create namespace factory
mkdir -p .local
cp charts/factory/example-values.yaml .local/kubernetes-values.yaml
```

In dieser Datei Image-Repository, tatsächlichen Image-Tag und RWX-StorageClass
eintragen. `projects` enthält die Projekt-YAMLs. Für bestehende PVCs stattdessen
`storage.server.existingClaim` und `storage.jobs.existingClaim` setzen.
Beide müssen für UID/GID 1000 beschreibbar sein.

Das bestehende lokale Setup hat `.local/server.env` mit `FABRO_DEV_TOKEN` und
`SESSION_SECRET`. Für eine neue Installation diese Datei mit eigenen Werten
anlegen: `fabro_dev_` plus 64 Hex-Zeichen für den Dev-Token, 64 zufällige
Hex-Zeichen für das Session-Secret. Nicht in Git ablegen. Anschließend:

```bash
kubectl -n factory create secret generic factory-auth \
  --from-env-file=.local/server.env \
  --from-file=github-pat=.local/secrets/github-pat
```

Für private GHCR-Images ein `kubernetes.io/dockerconfigjson`-Secret aus einer
dedizierten Registry-Konfiguration mit `read:packages` bereitstellen und in den
Values referenzieren:

```yaml
imagePullSecrets:
  - name: ghcr-pull
```

```bash
helm upgrade --install factory charts/factory -n factory \
  -f .local/kubernetes-values.yaml --wait --timeout 5m
kubectl -n factory port-forward svc/factory-factory-fabro 32277:32276
```

Die UI ist unter <http://localhost:32277> erreichbar. Den Dev-Token aus der
Secret-Datei über den Passwortmanager/Editor in die UI übernehmen, nicht in
Logs ausgeben. In der UI den OpenRouter-Provider konfigurieren. Codex verwendet
weiterhin den ChatGPT-Login:

```bash
kubectl -n factory exec -it factory-factory-fabro-0 -c fabro -- codex login --device-auth
```

Den angezeigten Link im Browser auf dem eigenen Rechner öffnen. Der Login bleibt
auf dem Fabro-PVC. Es werden keine statischen OpenAI-/Anthropic-Keys benötigt.
Die Reviewer-Dateien sind unter `$CODEX_HOME/agents` und die Skills unter
`$HOME/.agents/skills` verfügbar.

## Vorschau und Verarbeitung

Standardmäßig pollt die Orchestrierung nur lesend und zeigt ihre Entscheidungen:

```bash
kubectl -n factory logs deployment/factory-factory-orchestrator -c orchestrator -f
```

Erst nach erfolgreicher Anmeldung und Prüfung der Auswahl in den Values
`orchestrator.preview: false` setzen und erneut `helm upgrade` ausführen.
Damit beginnt die Verarbeitung freigegebener Issues. PRs werden weiterhin
manuell gemergt. `preview: true` verhindert neue Starts, beendet aber keine
bereits laufenden Fabro-Runs.

Nur einen Orchestrator für denselben Fabro-Server betreiben. Keine zweite
Helm-Release oder lokale Orchestrierung parallel starten; der Prozess-Lock
ersetzt keine verteilte Leader Election. Keine Pods gewaltsam löschen, um einen
zweiten aktiven Prozess zu erzwingen.

Die interne Service-Verbindung verwendet HTTP. Das Chart setzt dafür ausdrücklich
`--allow-http-fabro`; außerhalb eines vertrauenswürdigen Clusternetzes ist TLS
erforderlich. Die UI bleibt standardmäßig nur über Port-Forward erreichbar. Bei
einem eigenen HTTPS-Ingress `webURL` auf dessen externe Origin setzen.

Nach Secret-Rotation beide Workloads neu starten, damit Init-Container die
CLI-Anmeldung und PAT-Datei erneuern:

```bash
kubectl -n factory rollout restart statefulset/factory-factory-fabro
kubectl -n factory rollout restart deployment/factory-factory-orchestrator
```

Die PVCs haben `helm.sh/resource-policy: keep` und bleiben bei `helm uninstall`
erhalten. Sie enthalten Zugangsdaten und Workspaces; Backups und Löschen bewusst
behandeln. Der Orchestrator hat keinen Kubernetes-API-Zugriff. Die Agenten laufen
ohne innere Codex-Sandbox im unprivilegierten Fabro-Pod, wie vereinbart.

## Prüfen ohne Deployment

```bash
helm lint charts/factory -f charts/factory/example-values.yaml
helm template factory charts/factory -f charts/factory/example-values.yaml
make verify
```

`make verify` benötigt jetzt auch Helm. Es prüft unter anderem getrennte Pods,
den gemeinsamen Workspace-Mount, Preview und vorhandene PVCs. Rendering und
Container-Smoke-Tests ersetzen keinen Installationstest mit der StorageClass
des Zielclusters.
