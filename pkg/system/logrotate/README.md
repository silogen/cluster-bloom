# implementing aggressive use of logrotate for iSCSI logs

## Problem:
  - iSCSI logs can grow very large very quickly, especially in environments with high I/O activity (and Longhorn flooding has been seen in some rare cases)
  - Default logrotate settings may not be sufficient to manage the size of these logs effectively.
  - RKE2 cluster logs are managed by /etc/rancher/rke2/config.yaml values with the `audit-log-` prefix, with /var/lib/rancher/rke2/agent/containerd/containerd.log being a possible exception (so it is scoped in here). Known RKE2 managed paths:
      - /var/lib/rancher/rke2/agent/logs/*.log
      - /var/lib/rancher/rke2/server/logs/*.log 
      - /var/log/pods/*/*/*.log
      
## Implementation:

- Logrotate configuration files are written to:
    - /etc/logrotate.d/iscsi-aggressive
    - /etc/logrotate.d/rke2
- They are triggered via the system cron at `/etc/cron.d/logrotate
- Rotation is based on a file size (see conf/*.conf for specific sizes)
- 10 rotations total are kept
- old logs are compressed
- compression is delayed by one rotation
- new log files are created with retained file permissions (0640) and ownership (root:adm)


## Mannual operations when needed:
  - ### Restart rsyslog to apply filters
    `sudo systemctl restart rsyslog`

    ### Force an immediate rotation
    `sudo logrotate -f /etc/logrotate.d/iscsi-aggressive`
    `sudo logrotate -f /etc/logrotate.d/rke2`

    ### Test configuration
    `sudo logrotate -d /etc/logrotate.d/iscsi-aggressive`
    `sudo logrotate -d /etc/logrotate.d/rke2`

    ### Rotation status:
      - After running the logrotate command, check the status of rotated logs in /var/log/logrotate-bloom.log* to confirm that rotation has occurred as expected.
      - To view last rotation times: `sudo cat /var/lib/logrotate/status`.

## Customization:
  - Administrators can safely modify `/etc/logrotate.d/*` after the initial deployment. The file will not be overwritten on subsequent runs, allowing for environment-specific adjustments for rotation frequency, retention policy, compression settings, and post-rotation scripts.

  - To reset to defaults, simply delete the config file of choice and re-run the bloom binary with bloom.yaml key `ENABLED_STEPS: ConfigLogrotateStep`

## Idempotency:
  - The `Configure()` function (used by the `ConfigLogrotateStep`) is idempotent - it can be safely run multiple times with the same effect as the first run
  - **Config file check**: Before deploying `/etc/logrotate.d/*`, the function checks if a given config already exists
  - **Skip if present**: If the file exists, configuration is skipped entirely with a log message
  - **Manual changes preserved**: Any manual modifications to the config file are preserved across runs
  - **Safe re-execution**: The entire setup can be re-run without overwriting existing configurations