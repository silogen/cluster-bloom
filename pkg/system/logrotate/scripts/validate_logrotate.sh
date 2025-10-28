#!/bin/bash

# Check if logs are rotating properly
echo "=== Log Rotation Status ==="
echo "Current log sizes:"
ls -lh /var/log/kern.log* /var/log/syslog* 2>/dev/null

echo -e "\n=== Disk Usage ==="
df -h / | grep -E '^/|Filesystem'

echo -e "\n=== Recent rotations ==="
grep -E "kern.log|syslog" /var/lib/logrotate/status

echo -e "\n=== Testing logrotate configuration ==="
/usr/sbin/logrotate -d /etc/logrotate.d/iscsi-aggressive 2>&1 | head -20
