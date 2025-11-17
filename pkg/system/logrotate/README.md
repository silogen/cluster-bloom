# implementing aggressive use of logrotate for iSCSI logs

## Problem:
    - iSCSI logs can grow very large very quickly, especially in environments with high I/O activity (and Longhorn flooding has been seen in some rare cases)
    - Default logrotate settings may not be sufficient to manage the size of these logs effectively.

## Implementation:

- logrotate configuration file is written to /etc/logrotate.d/iscsi-aggressive with:
    - triggered every 10 minutes via system cron using `/etc/cron.d/iscsi-logrotate
    - rotation is based on a file size of 200MB for /var/log/kern.log and /var/log/syslog
    - 10 rotations total are kept, so ~2GB of retained logs
    - old logs are compressed using
    - delaying compression by one rotation
    - new log files are created with retained file permissions (0640) and ownership (root:adm)


- Mannual operations if needed:
    - ### Restart rsyslog to apply filters
      `sudo systemctl restart rsyslog`

      ### Force an immediate rotation to clean up existing large logs
      `sudo logrotate -f /etc/logrotate.d/iscsi-aggressive`

      ### Test configuration
      `sudo logrotate -d /etc/logrotate.d/iscsi-aggressive`

      ### Rotation status:
        - After running the logrotate command, check the status of rotated logs in /var/log/iscsi.log* to confirm that rotation has occurred as expected.
        - To view last rotation times: `sudo cat /var/lib/logrotate/status`.

## User customization:
  - Users can safely modify `/etc/logrotate.d/iscsi-aggressive` after initial deployment. The file will not be overwritten on subsequent runs, allowing for environment-specific adjustments for rotation frequency, retention policy, compression settings, and post-rotation scripts

  - To reset to defaults, simply delete the config file and re-run the bloom binary with bloom.yaml key `ENABLED_STEPS: ConfigLogrotateStep` or `ENABLED_STEPS: ConfigureRsyslogStep`

  - Idempotency:
        - The `Configure()` function is **idempotent** - it can be safely run multiple times with the same effect as the first run
        - **Config file check**: Before deploying `/etc/logrotate.d/iscsi-aggressive`, the function checks if it already exists
        - **Skip if present**: If the file exists, configuration is skipped entirely with a log message
        - **Manual changes preserved**: Any manual modifications to the config file are preserved across runs
        - **Safe re-execution**: The entire setup can be re-run without overwriting existing configurations