"""containerd never reopens its log, so the config must not rename it.

rke2 starts containerd and redirects its stdout to
/var/lib/rancher/rke2/agent/containerd/containerd.log. There is no --log-file
flag and no reopen signal.

With `create`, logrotate renames containerd.log and makes a new empty one.
containerd holds the descriptor of the file it opened, so it goes on writing
into the renamed archive. The new containerd.log then stays at 0 bytes, never
passes `size 100M`, and never rotates again, while the archive it is still
writing to grows with no config watching it:

    -rw-r----- 1 root root        0 Sep  9 20:00 containerd.log
    -rw-r----- 1 root root 38492209 Sep 11 07:23 containerd.log-20260909

A survey of the whole fleet on 2026-09-11 found this on 20 of the 30 nodes
that run rke2, holding 365 MB in total. The oldest had been frozen since
2026-08-14. The largest single archive was 52 MB and was still being appended
to while it was measured.

`copytruncate` copies the file and truncates the original in place, so the
descriptor stays valid. The iSCSI config does not need it and must not have
it: rsyslog reopens on the HUP that rsyslog-rotate sends in its postrotate,
and copytruncate would lose lines there for no reason.
"""
from pathlib import Path

import yaml

TASKS = (
    Path(__file__).resolve().parents[3]
    / "pkg/ansible/runtime/playbooks/tasks/deploy_cluster/logrotate.yaml"
)

RKE2_CONF = "/etc/logrotate-bloom.d/rke2.conf"
ISCSI_CONF = "/etc/logrotate-bloom.d/iscsi-aggressive.conf"


def config_content(dest):
    for task in yaml.safe_load(TASKS.read_text()):
        if task.get("copy", {}).get("dest") == dest:
            return task["copy"]["content"]
    raise AssertionError(f"no task writes {dest}")


def directives(content):
    """The directive lines, without comments or the path and brace lines."""
    out = []
    for line in content.splitlines():
        line = line.strip()
        if not line or line.startswith("#") or line in ("{", "}"):
            continue
        if line.startswith("/"):
            continue
        out.append(line)
    return out


def test_the_containerd_config_uses_copytruncate():
    assert "copytruncate" in directives(config_content(RKE2_CONF))


def test_the_containerd_config_does_not_create():
    """create and copytruncate are mutually exclusive. Together they make
    logrotate create a file it has just truncated, and `create` is what
    orphans the descriptor in the first place."""
    assert not [d for d in directives(config_content(RKE2_CONF))
                if d.startswith("create")]


def test_the_iscsi_config_signals_rsyslog_instead():
    """rsyslog DOES reopen, on the HUP rsyslog-rotate sends. copytruncate
    there would lose lines between the copy and the truncate for nothing."""
    content = config_content(ISCSI_CONF)
    assert "/usr/lib/rsyslog/rsyslog-rotate" in content
    assert "copytruncate" not in directives(content)

