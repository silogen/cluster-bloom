"""Unit tests for RKE2 node label generation."""

import re

import pytest
import yaml


def test_disk_labels_do_not_embed_device_paths(
    fake_rke2_root, ansible_runner_factory
):
    """Stable device aliases must not exceed Kubernetes label limits."""
    result = ansible_runner_factory(
        "tests/ansible/playbooks/test_node_labels.yaml",
        extravars={
            "cluster_disks_list": [
                "/dev/disk/by-id/wwn-0x60499ac855614ad59ee9aec267da4eb0"
            ],
            "NO_DISKS_FOR_CLUSTER": False,
            "CLUSTER_PREMOUNTED_DISKS": "",
            "GPU_NODE": False,
            "FIRST_NODE": True,
        },
    )

    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    config = yaml.safe_load(
        (fake_rke2_root / "config.yaml").read_text()
    )
    labels = config["node-label"]

    assert "bloom.disk___mnt___disk0=disk0" in labels
    assert all(
        len(label.split("=", 1)[1]) <= 63
        for label in labels
    )
    assert all("/dev/disk/by-id" not in label for label in labels)


# cluster-bloom/revision records which bloom built the node. The value is
# BLOOM_VERSION, which pkg/ansible/runtime/playbook.go gives to every run: a
# tag (v2.3.0-rc5), a branch (EAI-8203-ubuntu-2604) or a branch with a commit.
#
# A label VALUE that Kubernetes refuses stops the node from registering, so the
# value is sanitised and never trusted. A branch name can hold a '/'.
LABEL_VALUE = re.compile(r"^[A-Za-z0-9]([A-Za-z0-9._-]{0,61}[A-Za-z0-9])?$")


def _node_labels(fake_rke2_root, ansible_runner_factory, bloom_version):
    result = ansible_runner_factory(
        "tests/ansible/playbooks/test_node_labels.yaml",
        extravars={
            "cluster_disks_list": [],
            "NO_DISKS_FOR_CLUSTER": True,
            "CLUSTER_PREMOUNTED_DISKS": "",
            "GPU_NODE": True,
            "FIRST_NODE": False,
            "BLOOM_VERSION": bloom_version,
        },
    )
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"
    config = yaml.safe_load((fake_rke2_root / "config.yaml").read_text())
    return dict(label.split("=", 1) for label in config["node-label"])


def test_the_revision_joins_the_labels_that_exist(
    fake_rke2_root, ansible_runner_factory
):
    """The two older labels keep their names and their values."""
    labels = _node_labels(fake_rke2_root, ansible_runner_factory, "v2.3.0-rc5")

    assert labels["cluster-bloom/gpu-node"] == "true"
    assert labels["cluster-bloom/first-node"] == "false"
    assert labels["cluster-bloom/revision"] == "v2.3.0-rc5"


@pytest.mark.parametrize(
    "bloom_version,expected",
    [
        ("EAI-8203-ubuntu-2604-44893b4", "EAI-8203-ubuntu-2604-44893b4"),
        ("feat/some/branch", "feat-some-branch"),
        ("v2.3.0+build1", "v2.3.0-build1"),
        ("-leading-", "leading"),
        ("", "unknown"),
        ("x" * 80, "x" * 63),
    ],
)
def test_a_refused_value_is_made_safe(
    fake_rke2_root, ansible_runner_factory, bloom_version, expected
):
    """A node that cannot register is worse than an imprecise record."""
    labels = _node_labels(
        fake_rke2_root, ansible_runner_factory, bloom_version
    )

    value = labels["cluster-bloom/revision"]
    assert value == expected, f"{bloom_version!r} gave {value!r}"
    assert LABEL_VALUE.match(value), f"{value!r} is not a valid label value"
