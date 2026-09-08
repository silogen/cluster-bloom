"""Unit tests for the Cilium HelmChartConfig rendered by cilium_config.yaml.

These run the real task file with the real `copy` module, so they check the
bytes that land on disk rather than asserting about the template that produces
them. That matters more here than anywhere else in the playbook: the manifest
is read by helm-controller, and a change to it - including a whitespace-only
change - triggers `helm upgrade rke2-cilium` and rolls the Cilium DaemonSet.
"""

import yaml


MANIFEST = "rke2-cilium-config.yaml"

# The exact bytes every small/medium cluster deployed before CILIUM_HELM_VALUES
# existed. Rendering has to reproduce this character for character, or the first
# bloom rerun after upgrading reports `changed` and restarts every Cilium agent
# on the cluster for a whitespace diff.
LEGACY_MANIFEST = """apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-cilium
  namespace: kube-system
spec:
  valuesContent: |-
    operator:
      replicas: 1
"""

HUBBLE = {
    "hubble": {
        "enabled": True,
        "relay": {"enabled": True},
        "ui": {"enabled": True},
    }
}


def run_cilium_config(ansible_runner_factory, cluster_size="medium", values=None):
    extravars = {"FIRST_NODE": True, "CLUSTER_SIZE": cluster_size}
    if values is not None:
        extravars["CILIUM_HELM_VALUES"] = values

    return ansible_runner_factory(
        "tests/ansible/playbooks/test_cilium_config.yaml", extravars=extravars
    )


def values_content(manifests_dir):
    """Parse the manifest and return spec.valuesContent as a dict."""
    doc = yaml.safe_load((manifests_dir / MANIFEST).read_text())
    assert doc["kind"] == "HelmChartConfig"
    assert doc["metadata"]["name"] == "rke2-cilium"
    assert doc["metadata"]["namespace"] == "kube-system"
    return yaml.safe_load(doc["spec"]["valuesContent"])


def test_small_cluster_output_is_byte_identical_to_the_legacy_manifest(
    fake_rke2_manifests, ansible_runner_factory
):
    """The migration test: existing clusters must see no diff at all."""
    result = run_cilium_config(ansible_runner_factory, cluster_size="small")
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    assert (fake_rke2_manifests / MANIFEST).read_text() == LEGACY_MANIFEST


def test_medium_cluster_output_is_byte_identical_to_the_legacy_manifest(
    fake_rke2_manifests, ansible_runner_factory
):
    result = run_cilium_config(ansible_runner_factory, cluster_size="medium")
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    assert (fake_rke2_manifests / MANIFEST).read_text() == LEGACY_MANIFEST


def test_rerun_is_idempotent(fake_rke2_manifests, ansible_runner_factory):
    """A second run must report ok, not changed.

    `changed` here is not cosmetic - it means the file's mtime and content were
    rewritten, and on a live cluster helm-controller reacts to that.
    """
    assert run_cilium_config(ansible_runner_factory).rc == 0
    before = (fake_rke2_manifests / MANIFEST).read_text()

    result = run_cilium_config(ansible_runner_factory)
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    assert (fake_rke2_manifests / MANIFEST).read_text() == before

    changed = [
        e
        for e in result.events
        if e.get("event") == "runner_on_ok"
        and e.get("event_data", {}).get("task", "") == "Deploy Cilium HelmChartConfig"
        and e.get("event_data", {}).get("res", {}).get("changed")
    ]
    assert not changed, "the second run rewrote the manifest"


def test_valuescontent_parses_as_yaml(fake_rke2_manifests, ansible_runner_factory):
    """The rendered block scalar must be a YAML document in its own right.

    indent(4, first=False) - the filter default - leaves the first line at
    column 0 inside the block scalar, which still produces a file, just not one
    helm-controller can read.
    """
    assert run_cilium_config(ansible_runner_factory, values=HUBBLE).rc == 0

    expected = dict(HUBBLE)
    expected["operator"] = {"replicas": 1}
    assert values_content(fake_rke2_manifests) == expected


def test_large_with_no_values_writes_nothing(
    fake_rke2_manifests, ansible_runner_factory
):
    """Unchanged behaviour for existing large clusters: no manifest, no dir.

    large runs 2 cilium-operator replicas with hard anti-affinity by design;
    writing a manifest that pins replicas would break cluster_ready.yaml's gate.
    """
    result = run_cilium_config(ansible_runner_factory, cluster_size="large")
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    assert not fake_rke2_manifests.exists()


def test_large_with_user_values_writes_only_user_values(
    fake_rke2_manifests, ansible_runner_factory
):
    result = run_cilium_config(
        ansible_runner_factory, cluster_size="large", values=HUBBLE
    )
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    content = values_content(fake_rke2_manifests)
    assert content == HUBBLE
    assert "operator" not in content


def test_user_values_override_bloom_default(
    fake_rke2_manifests, ansible_runner_factory
):
    """combine() operand order: the operator wins on a key bloom also sets."""
    assert (
        run_cilium_config(
            ansible_runner_factory, values={"operator": {"replicas": 3}}
        ).rc
        == 0
    )

    assert values_content(fake_rke2_manifests) == {"operator": {"replicas": 3}}


def test_user_values_merge_recursively_into_bloom_default(
    fake_rke2_manifests, ansible_runner_factory
):
    """A sibling key under `operator` must not replace the whole default dict."""
    assert (
        run_cilium_config(
            ansible_runner_factory,
            values={"operator": {"rollOutPods": True}},
        ).rc
        == 0
    )

    assert values_content(fake_rke2_manifests) == {
        "operator": {"replicas": 1, "rollOutPods": True}
    }


def test_string_valued_input_is_accepted(fake_rke2_manifests, ansible_runner_factory):
    """The web UI textarea and environment overrides send raw YAML text.

    Neither can express a nested map natively, so the task has to parse it.
    """
    assert (
        run_cilium_config(
            ansible_runner_factory,
            values="hubble:\n  enabled: true\n  relay:\n    enabled: true\n",
        ).rc
        == 0
    )

    assert values_content(fake_rke2_manifests) == {
        "operator": {"replicas": 1},
        "hubble": {"enabled": True, "relay": {"enabled": True}},
    }


def test_empty_string_input_is_treated_as_unset(
    fake_rke2_manifests, ansible_runner_factory
):
    assert run_cilium_config(ansible_runner_factory, values="").rc == 0

    assert (fake_rke2_manifests / MANIFEST).read_text() == LEGACY_MANIFEST


def test_scalar_input_fails_loudly(fake_rke2_manifests, ansible_runner_factory):
    """`bloom --playbook` skips schema validation, so this is the only check.

    Silently ignoring a malformed value would deploy a cluster whose networking
    is not what the operator asked for, with nothing in the log to say so.
    """
    result = run_cilium_config(ansible_runner_factory, values="3")
    assert result.rc != 0, f"scalar CILIUM_HELM_VALUES was accepted:\n{result.stdout}"
    assert "CILIUM_HELM_VALUES" in result.stdout.read()


def test_list_input_fails_loudly(fake_rke2_manifests, ansible_runner_factory):
    result = run_cilium_config(ansible_runner_factory, values=["hubble"])
    assert result.rc != 0, f"list CILIUM_HELM_VALUES was accepted:\n{result.stdout}"
    assert "CILIUM_HELM_VALUES" in result.stdout.read()
