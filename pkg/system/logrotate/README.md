# implementing aggressive log rotation for iSCSI logs

## Problem:
    - iSCSI logs can grow very large very quickly, especially in environments with high I/O activity (and Longhorn spam)
    .
    - Default logrotate settings may not be sufficient to manage the size of these logs effectively.

## Implementation Steps:

- Copy the logrotate configuration file to /etc/logrotate.d/iscsi-aggressive
    - `sudo cp pkg/system/logrotate/iscsi-aggressive /etc/logrotate.d/iscsi-aggressive`

- Copy the rsyslog filter configuration to /etc/rsyslog.d/01-iscsi-filter.conf
    - `sudo cp pkg/system/logrotate/01-iscsi-filter.conf /etc/rsyslog.d/01-iscsi-filter.conf`


 - Set up hourly logrotate check (instead of just daily)
    - `sudo nano /etc/cron.hourly/logrotate-hourly`
    - ```
        #!/bin/sh
        /usr/sbin/logrotate -f /etc/logrotate.d/iscsi-aggressive
      ```

- Implement rsyslog filtering (to reduce log volume)
    
    ```
        cp con /etc/rsyslog.d/01-iscsi-filter.conf
       
    ```
    - `sudo chmod +x /etc/cron.hourly/logrotate-hourly`


- Validation script:
    - cp pkg/scripts/check_log_rotation.sh /usr/local/bin/check-log-rotation.sh
    - sudo chmod +x /usr/local/bin/check-log-rotation.sh

- Apply config:
    - # Restart rsyslog to apply filters
      `sudo systemctl restart rsyslog`

      # Force an immediate rotation to clean up existing large logs
      `sudo logrotate -f /etc/logrotate.d/iscsi-aggressive`

      # Test the configuration
      `sudo logrotate -d /etc/logrotate.d/iscsi-aggressive`
