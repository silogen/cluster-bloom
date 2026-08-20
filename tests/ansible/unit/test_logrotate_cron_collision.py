"""The bloom logrotate cron jobs must not meet on the same minute.

Both jobs run `logrotate -f <config>` without -s, so both use the default
state file /var/lib/logrotate/status, and logrotate takes an exclusive lock
on it. A second logrotate that starts while the first still holds the lock
exits with:

    error: state file /var/lib/logrotate/status is already locked
    logrotate does not support parallel execution on the same set of logfiles.

Three runs used to meet on minute 0: the iSCSI job at */10, the RKE2 job at
`0 * * * *`, and the distribution logrotate.service, which logrotate.timer
starts at 00:00. The error is collected in
/var/log/logrotate-bloom.log.

Minute 0 is therefore reserved for the distribution unit, and the two bloom
jobs take a minute each of their own.
"""
import re
from pathlib import Path

import yaml

TASKS = (
    Path(__file__).resolve().parents[3]
    / "pkg/ansible/runtime/playbooks/tasks/deploy_cluster/logrotate.yaml"
)

# The minute logrotate.timer starts the distribution logrotate.service.
DISTRO_MINUTE = 0

CRON_LINE = re.compile(r"^(\S+)\s+(\S+)\s+\S+\s+\S+\s+\S+\s+root\s+\S*logrotate\b")


def cron_content():
    for task in yaml.safe_load(TASKS.read_text()):
        if task.get("copy", {}).get("dest") == "/etc/cron.d/logrotate-bloom":
            return task["copy"]["content"]
    raise AssertionError("no task writes /etc/cron.d/logrotate-bloom")


def minutes_of(field, hour):
    """The minutes a cron minute-field fires on, for the fields bloom uses:
    a number, a comma list, */step, or first-last/step."""
    assert hour == "*", f"hourly assumption broken by hour field {hour!r}"
    out = set()
    for part in field.split(","):
        body, _, step = part.partition("/")
        step = int(step) if step else 1
        if body == "*":
            first, last = 0, 59
        elif "-" in body:
            first, last = (int(x) for x in body.split("-"))
        else:
            first = last = int(body)
        out.update(range(first, last + 1, step))
    return out


def schedules():
    """Job name -> the set of minutes it fires on."""
    found = {}
    for line in cron_content().splitlines():
        match = CRON_LINE.match(line.strip())
        if match:
            config = line.rsplit("logrotate-bloom.d/", 1)[-1].split()[0]
            found[config] = minutes_of(match.group(1), match.group(2))
    return found


def test_both_jobs_are_present():
    """A renamed or dropped config lands here, not in a silent pass."""
    assert sorted(schedules()) == ["iscsi-aggressive.conf", "rke2.conf"]


def test_no_two_jobs_share_a_minute():
    jobs = list(schedules().items())
    for i, (name_a, minutes_a) in enumerate(jobs):
        for name_b, minutes_b in jobs[i + 1 :]:
            shared = minutes_a & minutes_b
            assert not shared, f"{name_a} and {name_b} both fire on {sorted(shared)}"


def test_no_job_runs_when_the_distribution_unit_does():
    for name, minutes in schedules().items():
        assert DISTRO_MINUTE not in minutes, (
            f"{name} fires on minute {DISTRO_MINUTE}, when logrotate.timer "
            "starts logrotate.service"
        )


def test_the_iscsi_job_still_runs_every_ten_minutes():
    """The offset must not quietly change the interval it was chosen for."""
    minutes = sorted(schedules()["iscsi-aggressive.conf"])
    assert len(minutes) == 6
    assert {b - a for a, b in zip(minutes, minutes[1:])} == {10}


def test_the_rke2_job_still_runs_hourly():
    assert len(schedules()["rke2.conf"]) == 1
