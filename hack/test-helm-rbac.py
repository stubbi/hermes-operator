#!/usr/bin/env python3
"""Exercise the actual RBAC checker with Go-yq and Helm, without a cluster."""
import copy
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class HelmRBACCheckTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        result = subprocess.run(["yq", "--version"], capture_output=True, text=True, check=True)
        if "github.com/mikefarah/yq/" not in result.stdout or "version v4." not in result.stdout:
            raise RuntimeError("This regression requires the real Mike Farah Go-yq v4 CLI.")
        cls.generated = json.loads(subprocess.check_output(
            ["yq", "-o=json", ".", str(ROOT / "config/rbac/role.yaml")], text=True))

    def setUp(self):
        # CI installs yq as a Snap with its own /tmp mount namespace.
        # Keep private fixtures under the checkout so the real CLI can read them.
        self.temp = tempfile.TemporaryDirectory(prefix="hermes-rbac-check-", dir=ROOT)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        shutil.copytree(ROOT / "charts", self.root / "charts")
        role = self.root / "config/rbac/role.yaml"
        role.parent.mkdir(parents=True)
        shutil.copyfile(ROOT / "config/rbac/role.yaml", role)
        self.role = role
        self.template = self.root / "charts/hermes-operator/templates/clusterrole.yaml"

    def write_rendered(self, value):
        self.template.write_text(json.dumps(value) + "\n")

    def check(self):
        result = subprocess.run(["bash", str(ROOT / "hack/check-helm-rbac.sh")], cwd=self.root,
            env=dict(os.environ, KUBECONFIG="/dev/null"), capture_output=True, text=True, timeout=30)
        return result

    def test_current_valid_render_passes(self):
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse((self.root / ".yml").exists(), "The checker wrote a split-output artifact.")

    def test_equivalent_rule_order_and_grouping_pass(self):
        rendered = copy.deepcopy(self.generated)
        rendered["rules"] = [{"apiGroups": [group], "resources": [resource], "verbs": [verb]}
            for rule in reversed(self.generated["rules"]) for group in reversed(rule["apiGroups"])
            for resource in reversed(rule["resources"]) for verb in reversed(rule["verbs"])]
        self.write_rendered(rendered)
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_missing_or_extra_permissions_fail(self):
        for kind in ("missing", "extra"):
            with self.subTest(kind=kind):
                rendered = copy.deepcopy(self.generated)
                if kind == "missing":
                    rendered["rules"][0]["verbs"].remove("watch")
                else:
                    rendered["rules"].append({"apiGroups": ["unexpected.example"],
                        "resources": ["*"], "verbs": ["*"]})
                self.write_rendered(rendered)
                self.assertNotEqual(self.check().returncode, 0)

    def test_missing_empty_or_malformed_rules_fail(self):
        for side in ("generated", "rendered", "both"):
            for kind in ("missing", "empty", "missing-verbs", "scalar-resources", "empty-verbs", "nonstring-verbs", "constrained-rule", "duplicate-rules", "malformed-yaml"):
                with self.subTest(side=side, kind=kind):
                    shutil.copyfile(ROOT / "config/rbac/role.yaml", self.role)
                    shutil.copyfile(ROOT / "charts/hermes-operator/templates/clusterrole.yaml", self.template)
                    value = copy.deepcopy(self.generated)
                    if kind == "missing":
                        del value["rules"]
                    elif kind == "empty":
                        value["rules"] = []
                    elif kind == "missing-verbs":
                        del value["rules"][0]["verbs"]
                    elif kind == "scalar-resources":
                        value["rules"][0]["resources"] = "pods"
                    elif kind == "empty-verbs":
                        value["rules"][0]["verbs"] = []
                    elif kind == "nonstring-verbs":
                        value["rules"][0]["verbs"] = [True]
                    elif kind == "constrained-rule":
                        value["rules"][0]["resourceNames"] = ["specific"]
                    text = "rules: [\n" if kind == "malformed-yaml" else json.dumps(value)
                    if kind == "duplicate-rules":
                        text = text[:-1] + ', "rules": ' + json.dumps(value["rules"]) + "}"
                    if side in ("generated", "both"):
                        self.role.write_text(text)
                    if side in ("rendered", "both"):
                        self.template.write_text(text)
                    self.assertNotEqual(self.check().returncode, 0)

    def test_absent_or_ambiguous_clusterrole_fails(self):
        self.template.write_text("")
        self.assertNotEqual(self.check().returncode, 0)
        second = copy.deepcopy(self.generated)
        second["metadata"]["name"] = "another-role"
        self.template.write_text(json.dumps(self.generated) + "\n---\n" + json.dumps(second))
        self.assertNotEqual(self.check().returncode, 0)


if __name__ == "__main__":
    unittest.main()
