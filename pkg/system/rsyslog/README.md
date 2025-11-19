# implementing aggressive use of logrotate for iSCSI logs
 
 ## Implementation:
 
- The rsyslog filter configuration is written to /etc/rsyslog.d/01-iscsi-filter.conf

- The config file can be safely updated, as it is it not written if detected to already exist

- It filters out specific iSCSI log messages which have been previously seen for certain Longhorn/iSCSI issues

- specific filtered messages include any lines containing any of the following substrings:

```
    "detected conn error"
    "longhorn"
    "session recovery timed out"
```
