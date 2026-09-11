"""bloom must remove the cron file it wrote under its earlier name.

The task writes /etc/cron.d/logrotate-bloom. It used to write
/etc/cron.d/logrotate, and renaming the destination does not remove the file
already on the node. The old file still carries the -f flag and the minute 0
schedule that later commits removed, aimed at the config paths in
/etc/logrotate.d that this task now deletes. So a node bloom reinstalls keeps
a cron job that runs logrotate against files which are no longer there, on the
minute the distribution logrotate.service takes the state file lock.

Measured on int_test on 2026-09-11: useocpm2m-silogen-int-test-001 and -002
had the corrected layout in every other respect, and this one leftover file
was the only reason they were not clean.
"""
from pathlib import Path

import yaml

TASKS = (
    Path(__file__).resolve().parents[3]
    / "pkg/ansible/runtime/playbooks/tasks/deploy_cluster/logrotate.yaml"
)

OLD_CRON = "/etc/cron.d/logrotate"
NEW_CRON = "/etc/cron.d/logrotate-bloom"


def removed_paths():
    out = set()
    for task in yaml.safe_load(TASKS.read_text()):
        spec = task.get("file", {})
        if spec.get("state") != "absent":
            continue
        path = spec.get("path", "")
        if "{{ item }}" in path:
            prefix = path.split("{{")[0]
            out.update(prefix + name for name in task.get("loop", []))
        else:
            out.add(path)
    return out

def test_the_old_cron_file_is_removed():
    """bloom wrote /etc/cron.d/logrotate before the file was renamed to
    logrotate-bloom. Left behind it still carries the -f flag and the minute 0
    schedule that were removed, aimed at config paths in /etc/logrotate.d that
    this task now deletes.

    useocpm2m-silogen-int-test-001 and -002 on 2026-09-11 had the corrected
    layout in every other respect, and this one file was the only reason they
    were not clean."""
    assert OLD_CRON in removed_paths()


def test_the_current_cron_file_is_not_removed():
    """The removal must name the old path only. Removing the file the task
    has just written would leave the node with no rotation at all."""
    assert NEW_CRON not in removed_paths()
