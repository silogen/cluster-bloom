"""The cron commands must let the configs decide when to rotate.

`logrotate -f` forces a rotation whatever the config says, so the `size 200M`
and `size 100M` conditions never applied: every run rotated. Together with
`dateext` only the first run of a day could succeed, because the second one
found the dated name already taken:

    error: destination /var/log/syslog-20260820 already exists, skipping
    rotation

At */10 that is 143 error lines a day into /var/log/logrotate-bloom.log, and
a syslog that grows unchecked between midnights. Reproduced with logrotate
3.21.0.

The defect was invisible on the fleet because an earlier one hid it: the
configs lived in /etc/logrotate.d and had no `su root adm`, so every cron run
stopped at "parent directory has insecure permissions" before it reached the
rotation at all.
"""
import re
from pathlib import Path

import yaml

TASKS = (
    Path(__file__).resolve().parents[3]
    / "pkg/ansible/runtime/playbooks/tasks/deploy_cluster/logrotate.yaml"
)

CRON_DEST = "/etc/cron.d/logrotate-bloom"
CONFIG_DIR = "/etc/logrotate-bloom.d"

# `logrotate` with any option before the config path. -f and --force are the
# ones that matter; the pattern catches every option so a new one is looked at.
LOGROTATE_CALL = re.compile(r"/usr/sbin/logrotate\s+(?P<args>.*?)/etc/logrotate")


def tasks():
    return yaml.safe_load(TASKS.read_text())


def cron_lines():
    for task in tasks():
        if task.get("copy", {}).get("dest") == CRON_DEST:
            return [
                line.strip()
                for line in task["copy"]["content"].splitlines()
                if "/usr/sbin/logrotate" in line
            ]
    raise AssertionError(f"no task writes {CRON_DEST}")


def configs():
    """Destination path -> content, for each file under the bloom config dir."""
    found = {}
    for task in tasks():
        dest = task.get("copy", {}).get("dest", "")
        if dest.startswith(CONFIG_DIR + "/"):
            found[dest] = task["copy"]["content"]
    assert found, f"no task writes into {CONFIG_DIR}"
    return found


def test_no_cron_command_forces_a_rotation():
    for line in cron_lines():
        match = LOGROTATE_CALL.search(line)
        assert match, f"cannot read the logrotate call in: {line}"
        args = match.group("args").split()
        assert args == [], f"logrotate is called with {args}, which overrides the config"


def test_every_config_still_has_a_size_condition():
    """Without -f the size line is the only thing that triggers a rotation.
    A config that loses it would never rotate again."""
    for dest, content in configs().items():
        assert re.search(r"^\s*size\s+\d+[kMG]?\s*$", content, re.M), (
            f"{dest} has no size condition, so nothing triggers its rotation"
        )




def test_no_config_uses_dateext():
    """A size condition rotates more than once a day on a busy node, and
    dateext names every archive after the day alone, so the second rotation
    of a day collides with the first and is skipped."""
    for dest, content in configs().items():
        assert not re.search(r"^\s*dateext\s*$", content, re.M), (
            f"{dest} uses dateext, which caps it at one rotation a day"
        )
