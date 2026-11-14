import subprocess
import tomllib
import unittest

import yaml


class HelmTest(unittest.TestCase):
    def test_separate_pods_and_shared_workspaces(self):
        for preview in (True, False):
            with self.subTest(preview=preview):
                rendered = subprocess.check_output(
                    [
                        "helm",
                        "template",
                        "test",
                        "charts/factory",
                        "-f",
                        "charts/factory/example-values.yaml",
                        "--set",
                        f"orchestrator.preview={str(preview).lower()}",
                    ],
                    text=True,
                )
                documents = list(yaml.safe_load_all(rendered))
                objects = {d["metadata"]["name"]: d for d in documents if d}
                server = next(d for d in documents if d["kind"] == "StatefulSet")
                orch = next(d for d in documents if d["kind"] == "Deployment")
                self.assertEqual(orch["spec"]["strategy"]["type"], "Recreate")
                for workload in (server, orch):
                    self.assertEqual(workload["spec"]["replicas"], 1)
                    pod = workload["spec"]["template"]["spec"]
                    self.assertFalse(pod["automountServiceAccountToken"])
                    self.assertEqual(pod["nodeSelector"]["kubernetes.io/arch"], "arm64")
                    self.assertEqual(pod["securityContext"]["runAsUser"], 1000)
                    self.assertEqual(len(pod["containers"]), 1)
                    jobs = next(v for v in pod["volumes"] if v["name"] == "jobs")
                    self.assertEqual(
                        jobs["persistentVolumeClaim"]["claimName"], "test-factory-jobs"
                    )
                pod = orch["spec"]["template"]["spec"]
                storage = next(v for v in pod["volumes"] if v["name"] == "storage")
                self.assertIn("emptyDir", storage)
                self.assertEqual("--preview" in pod["containers"][0]["args"], preview)
                self.assertIn(
                    "--fabro-url=http://test-factory-fabro:32276", pod["containers"][0]["args"]
                )
                self.assertEqual(
                    objects["test-factory-jobs"]["spec"]["accessModes"], ["ReadWriteMany"]
                )
                self.assertEqual(
                    objects["test-factory-server"]["spec"]["accessModes"], ["ReadWriteOnce"]
                )
                config = tomllib.loads(objects["test-factory-settings"]["data"]["settings.toml"])
                self.assertEqual(config["server"]["storage"]["root"], "/storage")
                self.assertFalse(any(d["kind"] == "Secret" for d in documents))

    def test_existing_claims(self):
        rendered = subprocess.check_output(
            [
                "helm",
                "template",
                "test",
                "charts/factory",
                "-f",
                "charts/factory/example-values.yaml",
                "--set",
                "storage.server.existingClaim=server-existing",
                "--set",
                "storage.jobs.existingClaim=jobs-existing",
            ],
            text=True,
        )
        documents = list(yaml.safe_load_all(rendered))
        self.assertFalse(any(d["kind"] == "PersistentVolumeClaim" for d in documents))
        server = next(d for d in documents if d["kind"] == "StatefulSet")
        volumes = server["spec"]["template"]["spec"]["volumes"]
        claims = [
            v["persistentVolumeClaim"]["claimName"] for v in volumes if "persistentVolumeClaim" in v
        ]
        self.assertEqual(claims, ["server-existing", "jobs-existing"])

    def test_generic_ingress(self):
        rendered = subprocess.check_output(
            [
                "helm",
                "template",
                "test",
                "charts/factory",
                "-n",
                "test",
                "-f",
                "charts/factory/example-values.yaml",
                "--set",
                "ingress.enabled=true",
                "--set",
                "webURL=https://factory.example.org",
                "--set",
                "ingress.className=nginx",
                "--set",
                "storage.jobs.accessMode=ReadWriteOnce",
            ],
            text=True,
        )
        documents = list(yaml.safe_load_all(rendered))
        ingress = next(d for d in documents if d["kind"] == "Ingress")
        self.assertEqual(ingress["spec"]["ingressClassName"], "nginx")
        self.assertEqual(ingress["spec"]["rules"][0]["host"], "factory.example.org")
        backend = ingress["spec"]["rules"][0]["http"]["paths"][0]["backend"]["service"]
        self.assertEqual(backend, {"name": "test-factory-fabro", "port": {"number": 32276}})
        self.assertFalse(any(d["kind"] == "Middleware" for d in documents))
        claims = [d for d in documents if d["kind"] == "PersistentVolumeClaim"]
        self.assertTrue(all(d["spec"]["accessModes"] == ["ReadWriteOnce"] for d in claims))
