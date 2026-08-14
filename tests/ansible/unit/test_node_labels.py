"""Unit tests for RKE2 node label generation."""

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
