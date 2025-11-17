# implementing aggressive use of logrotate for iSCSI logs
 
 ## Implementation:
 
    - the rsyslog filter configuration is written to /etc/rsyslog.d/01-iscsi-filter.conf

    - filters out specific iSCSI log messages which have been previously seen for certain Longhorn/iSCSI issues

    - specific filtered messages include any lines containing any of the following substrings:
    
    ```
        "detected conn error"
        "session recovery timed out"
        "longhorn"
    ```