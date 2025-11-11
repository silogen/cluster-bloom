# implementing aggressive log rotation for iSCSI logs

## Problem:
    - iSCSI logs can grow very large very quickly, especially in environments with high I/O activity (and Longhorn flooding has been seen in some rare cases)
    - Default logrotate settings may not be sufficient to manage the size of these logs effectively.

## Implementation:

- logrotate configuration file is written to /etc/logrotate.d/iscsi-aggressive
    - `sudo cp pkg/system/logrotate/iscsi-aggressive /etc/logrotate.d/iscsi-aggressive`

- the rsyslog filter configuration is written to /etc/rsyslog.d/01-iscsi-filter.conf
    - `sudo cp pkg/system/logrotate/01-iscsi-filter.conf /etc/rsyslog.d/01-iscsi-filter.conf`


 - an hourly cron job triggers the logrotate check
    - `sudo nano /etc/cron.hourly/logrotate-hourly`
    - ```
        #!/bin/sh
        /usr/sbin/logrotate -f /etc/logrotate.d/iscsi-aggressive
      ```

- rsyslog filtering is setup to reduce log volume related to the specific iSCSI events:
    
    ```
        cp logrotate/01-iscsi-filter.conf /etc/rsyslog.d/01-iscsi-filter.conf
       
    ```

- Applying the configuration and validation:
    - ### Restart rsyslog to apply filters
      `sudo systemctl restart rsyslog`

      ### Force an immediate rotation to clean up existing large logs
      `sudo logrotate -f /etc/logrotate.d/iscsi-aggressive`

      ### Test the configuration
      `sudo logrotate -d /etc/logrotate.d/iscsi-aggressive`

      ### Dedicated validation script:
        - the logrotate/scripts/check-log-rotation.sh is written to /opt/validate_logrotate.sh and logs the results for review