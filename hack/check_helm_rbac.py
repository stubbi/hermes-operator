#!/usr/bin/env python3
"""Compare the default chart's resource permissions with the generated role."""
import json
import subprocess
import sys

ROLE = "config/rbac/role.yaml"
CHART = "charts/hermes-operator"


def command(arguments, data=None):
    result = subprocess.run(arguments, input=data, capture_output=True, text=True,
                            check=True, timeout=30)
    return result.stdout


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate RBAC field: " + key)
        result[key] = value
    return result


def permissions(documents, description):
    if not isinstance(documents, list) or len(documents) != 1:
        raise ValueError(description + " must contain exactly one ClusterRole")
    role = documents[0]
    if not isinstance(role, dict) or role.get("kind") != "ClusterRole" \
            or role.get("apiVersion") != "rbac.authorization.k8s.io/v1":
        raise ValueError(description + " is not an RBAC v1 ClusterRole")
    rules = role.get("rules")
    if not isinstance(rules, list) or not rules:
        raise ValueError(description + " must have a nonempty rules array")
    result = set()
    for rule in rules:
        # Refuse unsupported constraints rather than silently discarding them.
        if not isinstance(rule, dict) or set(rule) != {"apiGroups", "resources", "verbs"}:
            raise ValueError(description + " has missing or unsupported rule fields")
        for field in ("apiGroups", "resources", "verbs"):
            values = rule[field]
            if not isinstance(values, list) or not values or not all(
                    isinstance(value, str) and (value or field == "apiGroups") for value in values):
                raise ValueError(description + " has an invalid " + field + " array")
        result.update((group, resource, verb) for group in rule["apiGroups"]
                      for resource in rule["resources"] for verb in rule["verbs"])
    return result


def main():
    try:
        generated = json.loads(command(["yq", "eval-all", "-o=json", "-I=0", "[.]", ROLE]),
                               object_pairs_hook=unique_object)
        expected = permissions(generated, "generated RBAC")
        rendered_yaml = command(["helm", "template", "hermes-operator", CHART])
        rendered = json.loads(command(["yq", "eval-all", "-o=json", "-I=0",
            '[select(.kind == "ClusterRole")]', "-"], rendered_yaml), object_pairs_hook=unique_object)
        actual = permissions(rendered, "rendered RBAC")
        if expected != actual:
            print("::error::Helm chart ClusterRole differs from generated RBAC.", file=sys.stderr)
            for label, values in (("missing", expected - actual), ("extra", actual - expected)):
                for group, resource, verb in sorted(values):
                    print(f"{label}: {group or '(core)'} {resource} {verb}", file=sys.stderr)
            return 1
        return 0
    except subprocess.CalledProcessError as error:
        print(f"::error::{error.cmd[0]} failed: {error.stderr.strip()}", file=sys.stderr)
    except (OSError, ValueError, subprocess.TimeoutExpired) as error:
        print(f"::error::Cannot compare Helm RBAC: {error}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
