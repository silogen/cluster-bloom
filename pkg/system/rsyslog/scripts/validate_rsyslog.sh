#!/bin/bash
 
 echo -e "=== Validating logrotate configuration ==="

echo -e "\n=== Active rsyslog filters ==="
if [ -f /etc/rsyslog.d/01-iscsi-filter.conf ]; then
    echo "iSCSI filter is active"
    grep -v '^#' /etc/rsyslog.d/01-iscsi-filter.conf | grep -v '^$'
else
    echo "No iSCSI filter found"
fi