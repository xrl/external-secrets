# SPDX-License-Identifier: Apache-2.0
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("xrl_images", Path(__file__).with_name("xrl-images.py"))
images = importlib.util.module_from_spec(spec)
spec.loader.exec_module(images)


class ImageIdentityTests(unittest.TestCase):
    def test_unique_tag(self):
        self.assertEqual(images.tag("123", "2", "a" * 40), "v2.9.0-xrl.123.2-gaaaaaaaaaaaa")
        for run, attempt, sha in [("0", "1", "a" * 40), ("1", "x", "a" * 40), ("1", "1", "HEAD"), ("1;echo", "1", "a" * 40)]:
            with self.assertRaises(ValueError):
                images.tag(run, attempt, sha)
        for forbidden in ("latest", "main", "v2.9.0", "v2.9.0-xrl.1.1-gaaaaaaaaaaaa-amd64"):
            with self.assertRaises(ValueError):
                images.validate_tag(forbidden)

    def test_publishing_guard(self):
        for repo in ("xrl/external-secrets", "external-secrets/external-secrets", "attacker/external-secrets"):
            for ref in ("refs/heads/xrl/integration", "refs/heads/main", "refs/pull/1/merge", "refs/tags/xrl/integration"):
                for event in ("push", "workflow_dispatch", "pull_request", "pull_request_target", "workflow_run"):
                    self.assertEqual(images.publish_allowed(repo, ref, event), repo == "xrl/external-secrets" and ref == "refs/heads/xrl/integration" and event in ("push", "workflow_dispatch"))

    def test_workflow_boundaries(self):
        workflows = Path(__file__).resolve().parents[1] / ".github/workflows"
        workflow = (workflows / "xrl-images.yml").read_text()
        guard = "if: github.repository == 'xrl/external-secrets' && github.ref == 'refs/heads/xrl/integration' && (github.event_name == 'push' || github.event_name == 'workflow_dispatch')"
        self.assertEqual(workflow.count(guard), 2)
        build = workflow.split("  build:\n", 1)[1].split("  publish-children:\n", 1)[0]
        self.assertNotIn("packages: write", build)
        self.assertNotIn("github.token", build)
        self.assertNotIn("login-action", build)
        self.assertNotIn("qemu", workflow.lower())
        self.assertIn("go-version: '1.26.5'", build)
        self.assertIn("runner: ubuntu-24.04-arm", build)
        self.assertIn("runner: ubuntu-24.04\n", build)
        self.assertEqual(workflow.count("persist-credentials: false"), 3)
        self.assertIn("branches-ignore: [xrl/integration]", (workflows / "ci.yml").read_text())
        self.assertEqual((workflows / "publish.yml").read_text().count("if: github.repository == 'external-secrets/external-secrets'"), 2)
        self.assertIn("if: github.repository == 'external-secrets/external-secrets'", (workflows / "release.yml").read_text().split("  promote:", 1)[1])

    def test_exact_manifest(self):
        refs = [f"{images.REPOSITORY}@sha256:{char * 64}" for char in "ab"]
        manifest = {"manifests": [{"digest": ref.split("@")[1], "platform": {"os": "linux", "architecture": arch}} for arch, ref in zip(images.ARCHES, refs)]}
        images.validate_manifest(manifest, refs)
        for field, value in (("architecture", "unknown"), ("architecture", "arm64"), ("os", "darwin")):
            broken = json.loads(json.dumps(manifest))
            broken["manifests"][0]["platform"][field] = value
            with self.assertRaises(ValueError):
                images.validate_manifest(broken, refs)
        for descriptors in ([], manifest["manifests"][:1], manifest["manifests"] * 2, list(reversed(manifest["manifests"]))):
            if len(descriptors) == 2:
                images.validate_manifest({"manifests": descriptors}, refs)
            else:
                with self.assertRaises(ValueError):
                    images.validate_manifest({"manifests": descriptors}, refs)
        manifest["manifests"][0]["digest"] = "sha256:" + "c" * 64
        with self.assertRaises(ValueError):
            images.validate_manifest(manifest, refs)

    def test_child_records(self):
        version = images.tag("1", "1", "a" * 40)
        with tempfile.TemporaryDirectory() as directory:
            records = {}
            for arch, char in zip(images.ARCHES, "ab"):
                records[arch] = {"image": f"{images.REPOSITORY}:{version}-{arch}", "architecture": arch, "digest": "sha256:" + char * 64}
                Path(directory, arch + ".json").write_text(json.dumps(records[arch]))
            self.assertEqual(len(images.references(directory, version)), 2)
            for field, value in (("image", "ghcr.io/external-secrets/external-secrets:latest"), ("architecture", "arm64"), ("digest", "sha256:nope"), ("digest", records["arm64"]["digest"])):
                broken = dict(records["amd64"], **{field: value})
                Path(directory, "amd64.json").write_text(json.dumps(broken))
                with self.assertRaises(ValueError):
                    images.references(directory, version)


if __name__ == "__main__":
    unittest.main()
