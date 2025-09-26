#!/usr/bin/env bash
set -e

# Create fake block files (2 GB each)
dd if=/dev/zero of=disk1.img bs=1M count=2048
dd if=/dev/zero of=disk2.img bs=1M count=2048

# Mount them into the VM
multipass mount ./disk1.img cluster-bloom:/dev/sdb
multipass mount ./disk2.img cluster-bloom:/dev/sdc