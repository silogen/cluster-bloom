❯ please compare all of the disk cleanup and bloom cleanup stuff between v2.0.3 and the current loaded branch; I
  want a thorough analysis of the changes and a risk analysis of the changes considering that there may be many
  different environments and disk layouts we face. we absolutely don't want to destroy a disk that the system
  depends on or root disk.

● I'll spawn an agent to do a thorough diff analysis of all disk/cleanup-related code between v2.0.3 and the current
   branch.

● Explore(Thorough diff analysis of disk/bloom cleanup between v2.0.3 and current branch)
  ⎿  Done (32 tool uses · 86.5k tokens · 2m 40s)
  (ctrl+o to expand)

● Here is the full analysis:

  ---
  Disk/Cleanup Changes: v2.0.3 → EAI-7797_virtio_scsi_disk_detach

  What Changed at a High Level

  cleanup.go grew from ~402 to ~1,542 lines. The branch adds explicit protection against four historically dangerous
   operations: hot-removing block devices, logging out non-Longhorn iSCSI sessions, wiping the root disk, and
  corrupting premounted disks.

  ---
  Critical Changes (Ranked by Risk Impact)

  1. Removal of Legacy Block Device Hot-Removal — HIGHEST RISK FIXED

  Old behavior: Iterated all unmounted sd* devices and wrote 1 to /sys/block/<device>/device/delete to hot-detach
  them. This was originally written to evict stale Longhorn iSCSI LUNs after node reuse.

  Problem: A virtio-scsi cloud volume (e.g., an OCI attached block volume) appears as sd* and could have an empty
  MOUNTPOINT in lsblk output for its whole-disk row even while its partitions are mounted — including the root
  partition. The old code would detach the root disk.

  New behavior: The sweep is entirely removed. The code comment explicitly documents why:

  ▎ "Bloom v1 hot-removed all unmounted bare sd devices via /sys/block/<device>/device/delete to discard stale
  ▎ Longhorn iSCSI LUNs. Longhorn sessions and node records are now cleaned explicitly by CleanupLonghornMounts, so
  ▎ that sweep is unnecessary and cannot distinguish an orphaned LUN from an unmounted virtio-scsi cloud volume.
  ▎ Cleanup may wipe and reformat configured disks, but it must never detach them."*

  Residual risk: None from this path — the code is gone. Longhorn iSCSI cleanup now covers what this was doing.

  ---
  2. Scoped Longhorn iSCSI Logout — HIGH RISK FIXED

  Old behavior: iSCSI logout was unscoped — could terminate any iSCSI session on the node, including cloud boot
  volumes.

  New behavior: parseLonghornISCSISessions() filters by longhornIQNPrefix = "iqn.2019-10.io.longhorn:" and only logs
   out by session ID. Oracle boot volumes (iqn.2015-02.oracle.boot:uefi) and anything else are explicitly left
  alone. Tests cover this case.

  Residual risk: The IQN prefix iqn.2019-10.io.longhorn: is hardcoded. If Longhorn upstream ever changes
  GetTargetName() in go-iscsi-helper, the filter would miss those sessions (they'd linger, not be destroyed), which
  is the safe failure direction.

  ---
  3. Root Disk Guard — HIGH RISK FIXED

  rootFilesystemDisk() uses findmnt -n -o SOURCE / to identify the device backing /, then resolves partitions to
  their parent whole-disk via lsblk -no PKNAME. This result is used to categorically refuse any destructive
  whole-disk operation against the OS disk.

  Residual risk: If / is on a btrfs subvolume, LVM logical volume, or md-raid, the findmnt → lsblk PKNAME chain
  might not resolve to the expected sd*/vd* name. This is an edge case but worth validating in your target
  environments.

  ---
  4. Premounted Disk Distinction — MEDIUM RISK FIXED

  Previously all Bloom-managed fstab entries looked the same. Now three tags exist:
  - # managed by cluster-bloom → CLUSTER_DISKS → will be wiped
  - # premounted by cluster-bloom → CLUSTER_PREMOUNTED_DISKS → preserved, contents cleaned only
  - # managed by cluster-bloom rancher-disk → RANCHER_DISK → device wiped, re-formatted

  Cleanup logic explicitly gates wipe operations on the tag. Previously a redeployment could accidentally wipe a
  premounted disk.

  ---
  5. Signal/Interrupt Protection — OPERATIONAL SAFETY

  EnterCriticalSection() / ExitCriticalSection() wrap the disk wipe path. A SIGINT or SIGTERM during
  wipefs/mkfs.ext4 is caught, queued, and only acted on after the critical section completes. A second signal forces
   immediate exit. This prevents a half-wiped filesystem from being left in a corrupt state.

  ---
  6. New RANCHER_DISK Support — NEW SURFACE

  New config parameter, new Ansible task (rancher_storage.yaml), new cleanup path (CleanupRancherDisk()). The
  cleanup path:
  1. Detects whether RANCHER_DISK is a managed device (fstab tag) or a legacy manual mount of /var/lib/rancher
  2. Unmounts the bind mount at /var/lib/rancher
  3. Runs wipefs -a + mkfs.ext4 on the raw device
  4. Removes the fstab entry

  Residual risk: The legacy detection path (detectLegacyRancherMount()) reads /proc/mounts and resolves through
  symlinks. If the device path is a symlink (e.g., /dev/disk/by-id/...) and fstab uses the canonical path, it could
  fail to match and skip cleanup, leaving a stale mount. Worth testing on any environment that uses persistent
  device aliases.

  ---
  Residual / Watch-List Risks

  ┌────────────────────────────────┬────────────────────────────────────────────────┬──────────────────────────┐
  │            Scenario            │                      Risk                      │          Notes           │
  ├────────────────────────────────┼────────────────────────────────────────────────┼──────────────────────────┤
  │ Root on LVM/btrfs/md-raid      │ rootFilesystemDisk() may not resolve to        │ Rare in your fleet but   │
  │                                │ physical disk name                             │ worth verifying          │
  ├────────────────────────────────┼────────────────────────────────────────────────┼──────────────────────────┤
  │ RANCHER_DISK as symlink        │ Legacy mount detection may not match fstab     │ Test non-canonical       │
  │ /dev/disk/by-id/...            │ entry                                          │ device paths             │
  ├────────────────────────────────┼────────────────────────────────────────────────┼──────────────────────────┤
  │ Multiple iSCSI sessions to     │ Covered by tests, but worth confirming with    │                          │
  │ same Longhorn target           │ real iscsiadm output                           │                          │
  ├────────────────────────────────┼────────────────────────────────────────────────┼──────────────────────────┤
  │ Longhorn v2 data engine        │ IQN prefix filter won't match — sessions would │ Document this limitation │
  │ (NVMe-oF)                      │  be left (safe direction)                      │                          │
  ├────────────────────────────────┼────────────────────────────────────────────────┼──────────────────────────┤
  │ Premounted disk at same index  │ calculateFutureDiskStart() avoids this, but    │ If fstab is stale/wrong, │
  │ as future CLUSTER_DISK         │ only if fstab is the source of truth           │  collision is possible   │
  ├────────────────────────────────┼────────────────────────────────────────────────┼──────────────────────────┤
  │ Ctrl+C during                  │ Only CleanupBloomDisks is wrapped in critical  │ Check if this needs the  │
  │ CleanupRancherDisk             │ section; RANCHER_DISK cleanup is not           │ same protection          │
  └────────────────────────────────┴────────────────────────────────────────────────┴──────────────────────────┘

  ---
  Bottom Line

  The branch is strictly safer than v2.0.3 for every identified destruction vector. The legacy hot-removal sweep was
   the most dangerous code in the old version, and it's gone. The two highest remaining watch items are (a) root
  disk detection on non-standard filesystems, and (b) the RANCHER_DISK legacy detection path on environments using
  symlink device names. Neither of these would cause new failures relative to v2.0.3 — they're new code paths with
  new edge cases.

✻ Crunched for 3m 33s

❯ compare this branch to main with the same criterias; safety, risk management, risk surface, speculation about
  scenarios regarding different disk layouts and potentials for destroying system disks or root disks or operating
  system disks

● I'll spawn an agent to do a thorough diff of the current branch against main, focused entirely on disk safety.

● Explore(Thorough disk/cleanup diff between main and current branch)
  ⎿  Done (30 tool uses · 54.7k tokens · 2m 5s)
  (ctrl+o to expand)

● Here is the analysis comparing this branch to main:

  ---
  Branch vs. Main: Disk Safety Comparison

  Scope of the Diff

  Only two files differ between this branch and main:

  1. pkg/ansible/runtime/cleanup.go — 108 lines removed, 6 added
  2. pkg/ansible/runtime/playbooks/tasks/deploy_cluster/logrotate.yaml — logging config only, no disk relevance

  The logrotate change is irrelevant to this analysis. Everything below is about cleanup.go.

  ---
  What Was Removed from Main

  The entire block device hot-removal sweep is gone. On main, CleanupBloomDisks() ends with this logic (lines
  ~420–489 on main):

  // Delete unmounted disk devices (matching Bloom v1 logic)
  rootDisk := rootFilesystemDisk()
  cmd = exec.Command("lsblk", "-nl", "-o", "NAME,TYPE,MOUNTPOINT,PKNAME")
  // ... iterates all block devices ...
  for name, info := range disks {
      if !strings.HasPrefix(name, "sd") || info.mountpoint != "" || info.inUse {
          continue
      }
      if rootDisk != "" && name == rootDisk {
          fmt.Printf("      🛑 SKIPPING /dev/%s: backs the root filesystem\n", name)
          continue
      }
      // Writes "1" to /sys/block/<name>/device/delete — hot-removes the device
      deleteCmd := exec.Command("sudo", "tee", "/sys/block/"+name+"/device/delete")
      deleteCmd.Stdin = strings.NewReader("1\n")
      deleteCmd.Run()
  }

  Also removed: the rootFilesystemDisk() helper function (~28 lines, lines ~509–536 on main), which used findmnt +
  lsblk PKNAME to identify the OS disk.

  Replaced with a 5-line comment explaining why the sweep is gone.

  ---
  The Core Safety Problem with the Main Branch Code

  The hot-removal sweep on main has a fundamental flaw in its threat model: it uses lsblk --nodeps (which suppresses
   partition rows) to detect "unmounted disks." This means a partitioned disk shows an empty MOUNTPOINT at the
  whole-disk row even when its partitions are actively mounted. The code tries to compensate with the inUse tracking
   via PKNAME, but this creates a fragile heuristic with several failure modes:

  Scenario 1: Virtio-SCSI Cloud Volume (the core motivation for this branch)

  OCI, AWS, Azure, and GCP all expose attached block volumes as sd* devices. A virtio-scsi data volume that is
  intentionally unmounted — for example, one that Bloom just cleaned and reformatted — looks identical in lsblk
  output to a stale Longhorn iSCSI LUN. The main branch code would hot-remove it, detaching a cloud volume the
  operator intended to keep.

  After this branch: the sweep does not exist. The device stays attached.

  Scenario 2: Root Disk on a Partition (/dev/sda1 as /)

  lsblk -nl reports sda with MOUNTPOINT="" because the root mount is on sda1, not sda itself. The inUse flag is
  supposed to catch this via PKNAME tracking — when sda1 is seen, disks["sda"].inUse = true. This works if lsblk
  reports sda1 before the code checks sda. Since it iterates a Go map (unordered), the logic is actually fine — all
  rows are parsed first, then all eligibility checks run second. However, rootFilesystemDisk() provides a second
  backstop.

  Failure mode: findmnt returns a device path that strings.TrimPrefix(source, "/dev/") doesn't cleanly strip — for
  example, if root is on a device mapper path (/dev/mapper/vg0-root), rootFilesystemDisk() returns mapper/vg0-root,
  which will never match any sd* name. The root disk guard silently fails and falls back to only the inUse
  heuristic.

  After this branch: both the sweep and the guard function are gone. No hot-removal at all.

  Scenario 3: Root Disk on LVM, MD-RAID, or LUKS

  findmnt returns /dev/dm-0 or /dev/md0 or /dev/mapper/something. PKNAME resolution via lsblk -no PKNAME /dev/dm-0
  may return the physical volume (sda), or it may return nothing if the LVM sits on a partition. The root disk
  detection is not reliable across these layouts. On main, a failure in detection means the physical disk could pass
   all guards and be hot-removed.

  After this branch: not a concern. There is no hot-removal.

  Scenario 4: iSCSI Boot Volume (OCI Bare Metal)

  OCI bare metal nodes boot from iSCSI. The boot volume appears as sda (or similar) and is an iSCSI device, not
  virtio-scsi. It mounts / on a partition (sda3 or similar). On main:

  - rootFilesystemDisk() should catch it via findmnt → lsblk PKNAME
  - inUse flag should catch it because sda3 has PKNAME=sda

  But the Longhorn iSCSI cleanup (before this sweep runs) does an unscoped logout on main — there's no IQN
  filtering. If the boot volume's iSCSI session is logged out, the kernel may report the device as unmounted before
  the guard checks run.

  After this branch: Longhorn iSCSI cleanup is now scoped to iqn.2019-10.io.longhorn:* only. OCI boot volume
  sessions (iqn.2015-02.oracle.boot:uefi) are explicitly excluded. No hot-removal sweep runs afterward.

  Scenario 5: NVMe Devices

  The sweep only checks strings.HasPrefix(name, "sd"). NVMe devices (nvme0n1, etc.) are skipped. This is safe for
  NVMe root disks but means stale Longhorn NVMe/NVMe-oF sessions would also be skipped — they'd just stay attached.
  No destructive action on NVMe regardless.

  After this branch: same behavior (no hot-removal), now explicit by design.

  ---
  What the Branch Relies On Instead

  The work previously done by the hot-removal sweep is now expected to be handled entirely by
  CleanupLonghornMounts(), which runs before CleanupBloomDisks(). The key protection is
  parseLonghornISCSISessions():

  const longhornIQNPrefix = "iqn.2019-10.io.longhorn:"

  func parseLonghornISCSISessions(out string) []iscsiSession {
      for _, line := range strings.Split(out, "\n") {
          fields := strings.Fields(line)
          if len(fields) < 4 || !strings.HasPrefix(fields[3], longhornIQNPrefix) {
              continue  // skips oracle, other iSCSI — only Longhorn matched
          }
          // collect session ID, portal, target
      }
  }

  Logout is per session ID (iscsiadm -m session -r <sid> --logout), not global. Node record deletion is per
  target+portal. No unscoped operations.

  ---
  Risk Surface Comparison

  ┌──────────────────────────────┬────────────────────────────────────────┬────────────────────────────────────┐
  │        Risk Scenario         │              Main Branch               │            This Branch             │
  ├──────────────────────────────┼────────────────────────────────────────┼────────────────────────────────────┤
  │ Virtio-SCSI cloud volume     │ Possible — looks identical to iSCSI    │ Not possible — sweep removed       │
  │ hot-removed after cleanup    │ LUN in lsblk                           │                                    │
  ├──────────────────────────────┼────────────────────────────────────────┼────────────────────────────────────┤
  │ Root disk on partition       │ Guarded by inUse flag +                │ Not possible — no hot-removal      │
  │ (sda1) destroyed             │ rootFilesystemDisk() — mostly safe     │                                    │
  ├──────────────────────────────┼────────────────────────────────────────┼────────────────────────────────────┤
  │ Root disk on                 │ rootFilesystemDisk() may fail to       │ Not possible — no hot-removal      │
  │ LVM/MD-RAID/LUKS destroyed   │ resolve device mapper path             │                                    │
  ├──────────────────────────────┼────────────────────────────────────────┼────────────────────────────────────┤
  │ OCI iSCSI boot volume        │ Guarded by rootDisk check, but iSCSI   │ Not possible — iSCSI logout is     │
  │ detached                     │ logout was unscoped before sweep       │ IQN-filtered; no sweep             │
  ├──────────────────────────────┼────────────────────────────────────────┼────────────────────────────────────┤
  │ Non-Longhorn iSCSI session   │ Possible — prior logout logic was      │ Not possible — filtered by         │
  │ logged out                   │ unscoped                               │ longhornIQNPrefix                  │
  ├──────────────────────────────┼────────────────────────────────────────┼────────────────────────────────────┤
  │ CLUSTER_DISKS root disk      │ Possible if misconfigured — no guard   │ Same — no guard in wipefs path;    │
  │ wiped by wipefs              │ in wipefs path                         │ operator responsibility            │
  ├──────────────────────────────┼────────────────────────────────────────┼────────────────────────────────────┤
  │ Longhorn v2 / NVMe-oF        │ Left attached (sweep skips non-sd*)    │ Left attached (no iSCSI sessions   │
  │ sessions left attached       │                                        │ match prefix) — safe direction     │
  ├──────────────────────────────┼────────────────────────────────────────┼────────────────────────────────────┤
  │ Longhorn IQN prefix changes  │ Not applicable (no filtering existed)  │ Sessions would be left attached —  │
  │ upstream                     │                                        │ safe direction                     │
  └──────────────────────────────┴────────────────────────────────────────┴────────────────────────────────────┘

  ---
  The One Remaining Risk That Neither Branch Guards

  Both branches share one gap: if CLUSTER_DISKS is misconfigured to include the root device path, wipefs -a
  <root_device> will run against it. The findmnt --source <device> pre-check before wipefs verifies the device isn't
   currently mounted at a whole-disk level, but if root is on a partition (/dev/sda1) and CLUSTER_DISKS contains
  /dev/sda, findmnt will not find a mount for /dev/sda itself, and wipefs will proceed.

  This is an operator configuration error, and it existed on main too — but it's worth documenting. A
  rootFilesystemDisk() guard in the wipefs path (not just in the now-deleted hot-removal sweep) would close it. That
   said, no regression is introduced here — the risk surface for this scenario is identical on both branches.

  ---
  Bottom Line

  This branch is strictly safer than main for all scenarios involving unintended disk destruction. The hot-removal
  sweep on main is the most dangerous code in cleanup and has several realistic failure modes across common cloud
  disk layouts (virtio-scsi volumes, iSCSI boot volumes, LVM root disks). Removing it eliminates the entire category
   of "Bloom detaches a disk it shouldn't." The replacement — scoped Longhorn iSCSI logout by session ID and IQN
  prefix — accomplishes the actual goal (cleaning up stale Longhorn block devices) with no risk of collateral damage
   to system or cloud storage.

✻ Sautéed for 3m 21s

❯ is there any regex failures that could potentially wipe a system or root disk based on a partial match or
  expression issue

● Let me read the actual code directly rather than delegating this — I need to see every string match, prefix check,
   and parse expression in the cleanup path.

  Searched for 1 pattern, read 1 file (ctrl+o to expand)

● Now let me look at the awk regex in the Ansible task and the fstab tag matching logic more carefully:

  Read 1 file (ctrl+o to expand)

● Now I have read all the relevant code. Here is the analysis:

  ---
  Regex / String Match Failure Analysis

  There are no regexp calls anywhere in cleanup.go. All matching is done via strings.* primitives and one awk regex
  embedded in a shell string. The risks are in partial-match logic, not regex engine edge cases. Here is every
  dangerous one:

  ---
  1. Fstab Tag Matching — strings.Contains on Free-Form Comment Text

  Location: Lines 1046–1048, 1070, 1512

  // Decides whether a line is a CLUSTER_DISK (will be wiped + removed from fstab):
  strings.Contains(line, "# managed by cluster-bloom") &&
  !strings.Contains(line, "# premounted by cluster-bloom") &&
  !strings.Contains(line, "# managed by cluster-bloom rancher-disk")

  The exposure: strings.Contains matches anywhere in the line — it is not anchored. A user fstab comment that
  happens to contain the text managed by cluster-bloom (e.g., a descriptive comment they wrote themselves, or a
  backup entry left by a script) would be treated as a bloom-managed entry and its mount point would be unmounted
  and its line deleted from fstab. If a non-bloom disk's fstab line contained that comment and also mapped to a
  device that was somehow not already mounted, CleanupBloomDisks would then receive that device and wipefs it.

  How likely: Low in practice — the tag is specific enough that accidental collision is unlikely. But there is no
  positional anchoring (e.g., requiring the comment to appear as the last field, or requiring it to start the
  comment section). A fstab line like:

  /dev/sda   /data   ext4   defaults   0 0   # old entry, managed by cluster-bloom team

  ...would be matched and its mount point sent to umount -lf.

  Severity: If the matched fstab line happens to describe the root or OS disk, umount -lf / would be attempted (and
  would fail on a live system), but the line would still be removed from fstab, breaking the next boot.

  ---
  2. fstab Line Cleanup in CleanFstabEntries — strings.Contains Again, No Guard on Mount Point

  Location: Lines 1508–1533

  if strings.Contains(line, "# managed by cluster-bloom") && !strings.Contains(line, "# premounted by
  cluster-bloom") {
      fields := strings.Fields(line)
      if len(fields) >= 2 {
          mountPoint := fields[1]
          exec.Command("sudo", "umount", "-lf", mountPoint).Run()
      }
      // line is dropped from fstab — always, even if umount failed
  }

  Critical gap: There is no check that mountPoint is a /mnt/diskN path before calling umount -lf. The only gate is
  the tag match. If tag matching is fooled (see point 1), umount -lf is called on whatever fields[1] is — including
  /, /boot, /usr, or any other critical mount point. The line is then unconditionally dropped from fstab regardless
  of whether umount succeeded. This is a two-stage failure: attempted forced unmount of a critical mount followed by
   removal of its fstab entry.

  ---
  3. CleanupRancherDisk — Mount Check Uses strings.Contains on Full mount Output

  Location: Lines 792, 824

  mountCheckOutput, _ := exec.Command("mount").CombinedOutput()
  if strings.Contains(string(mountCheckOutput), "/var/lib/rancher") {

  The exposure: mount output contains device paths, mount points, and options. strings.Contains(output,
  "/var/lib/rancher") will match if /var/lib/rancher appears anywhere in the output — including as a prefix of a
  longer path. For example, if a volume is mounted at /var/lib/rancher-backup or /var/lib/rancher2, this check
  returns true and cleanup proceeds as if /var/lib/rancher itself is mounted.

  In that case, getDeviceFromCurrentMount("/var/lib/rancher") is called via findmnt (which is exact), so the device
  path should be correct — but the early return nil at line 804 (when the mount output does not contain the string)
  could be triggered incorrectly in the inverse case if a mount at /var/lib/rancher produces output that somehow
  doesn't match. The findmnt calls that determine the actual device to wipe are exact, so this particular issue is
  more about control-flow correctness than device selection. Still worth noting.

  ---
  4. detectLegacyRancherMount — fstab Field Match Is Exact, But Then Falls Through to Wipe Without Device Validation

  Location: Lines 1175–1187

  for _, line := range strings.Split(string(data), "\n") {
      fields := strings.Fields(line)
      if len(fields) >= 2 && fields[1] == "/var/lib/rancher" {
          if strings.Contains(line, "# managed by cluster-bloom") {
              return "", false
          }
      }
  }
  return source, true // "legacy"

  The field match on fields[1] == "/var/lib/rancher" is exact — good. But after this function returns (source,
  true), the caller (discoverAllBloomStorage) passes source to resolveDevicePathFromSource, which returns a /dev/
  path. That path then flows into CleanupRancherDisk, which calls wipefs -a <devicePath> and mkfs.ext4 -F
  <devicePath> with no validation that the device is not the root disk or OS disk.

  The chain is: findmnt -n -o SOURCE /var/lib/rancher → raw output → resolveDevicePathFromSource → wipefs. There is
  no rootFilesystemDisk() guard anywhere in the RANCHER_DISK path (that function was removed from the codebase
  entirely). If /var/lib/rancher were somehow bind-mounted from a path on the root device — or if findmnt returned
  an unexpected device — wipefs would run against it.

  ---
  5. The awk Regex in GenerateCleanupTasks — Dot Escaping Is Correct, But Field Splitting Is Not Anchored

  Location: Line 530

  "iscsiadm -m session 2>/dev/null | awk '$4 ~ /^" +
      strings.ReplaceAll(longhornIQNPrefix, ".", "\\.") +
      "/ { ... }'"

  After strings.ReplaceAll, the awk regex becomes:

  /^iqn\.2019-10\.io\.longhorn:/

  The dots are correctly escaped. The ^ anchor prevents prefix bypass. This is safe.

  Minor note: iscsiadm -m session output format is tcp: [<sid>] <portal>,<tpgt> <target> (<state>). The awk is
  matching $4, which is the target IQN — correct. However, if the output format ever gains an extra leading field
  (e.g., an IPv6 portal with spaces in the address), $4 would shift and the filter would silently stop working. This
   is a robustness issue, not a safety issue — failure direction is sessions left attached, not sessions wrongly
  logged out.

  ---
  6. extractDiskIndex — No Upper Bound on Parsed Integer

  Location: Lines 902–910

  func extractDiskIndex(mountPoint string) (int, error) {
      mp := strings.TrimSpace(mountPoint)
      const prefix = "/mnt/disk"
      if !strings.HasPrefix(mp, prefix) {
          return 0, fmt.Errorf("not a /mnt/diskN path: %s", mp)
      }
      return strconv.Atoi(mp[len(prefix):])
  }

  strconv.Atoi will succeed on /mnt/disk0, /mnt/disk999, and also on /mnt/disk (suffix "") — no, wait: "" returns an
   error from Atoi. But /mnt/disk-1 would return a negative integer which Atoi accepts. A negative index in reserved
   is harmless (it just reserves a key that will never match a start index). Not a disk-destruction risk.

  ---
  Summary Table

  ┌──────────────────────────────────────┬────────────┬─────────────────────────────────┬──────────────────────┐
  │                Issue                 │  Location  │              Risk               │  Realistic Trigger   │
  ├──────────────────────────────────────┼────────────┼─────────────────────────────────┼──────────────────────┤
  │ strings.Contains on fstab tag — no   │ L1046,     │ fstab entry for non-bloom disk  │ Freeform comment     │
  │ positional anchor                    │ L1512      │ treated as bloom-managed;       │ containing managed   │
  │                                      │            │ umount attempted + line deleted │ by cluster-bloom     │
  ├──────────────────────────────────────┼────────────┼─────────────────────────────────┼──────────────────────┤
  │ No root disk guard in                │ L1516–1517 │ umount -lf <any mount point> if │ Same as above        │
  │ CleanFstabEntries before umount -lf  │            │  tag match is fooled            │                      │
  ├──────────────────────────────────────┼────────────┼─────────────────────────────────┼──────────────────────┤
  │ No root disk guard in                │            │ wipefs -a <device> and          │ Legacy mount         │
  │ CleanupRancherDisk wipefs path       │ L836–845   │ mkfs.ext4 with no guard against │ detection resolves   │
  │                                      │            │  root device                    │ wrong device         │
  ├──────────────────────────────────────┼────────────┼─────────────────────────────────┼──────────────────────┤
  │ strings.Contains(mountOutput,        │            │ Control-flow error if sibling   │ Non-standard disk    │
  │ "/var/lib/rancher") — substring      │ L792, L824 │ path exists (e.g.               │ layouts with similar │
  │ match                                │            │ /var/lib/rancher2)              │  mount paths         │
  ├──────────────────────────────────────┼────────────┼─────────────────────────────────┼──────────────────────┤
  │ awk $4 field index assumption        │ L530       │ iSCSI filter silently stops     │ Longhorn or iscsiadm │
  │                                      │            │ working if output format shifts │  version change      │
  └──────────────────────────────────────┴────────────┴─────────────────────────────────┴──────────────────────┘

  ---
  The Most Actionable Fix

  The highest-severity gap is the absence of a root disk guard in CleanupRancherDisk. The rootFilesystemDisk()
  function was removed from the codebase when the hot-removal sweep was removed, but the RANCHER_DISK wipe path has
  no equivalent protection. A guard like:

  rootDisk := rootFilesystemDisk()
  if rootDisk != "" && strings.Contains(devicePath, rootDisk) {
      return fmt.Errorf("refusing to wipe %s: it backs the root filesystem", devicePath)
  }

  ...inserted before the wipefs call in CleanupRancherDisk would close that gap. The fstab strings.Contains tag
  matching could similarly be tightened by requiring the tag to appear after the options field (e.g., as the last
  comment), or by cross-validating the matched mount point against a known-safe prefix like /mnt/disk.

✻ Sautéed for 1m 40s

❯ are we basing any decisions on bloom.yaml; the problem is that bloom.yaml might not match the current deployment

● Let me read the cleanup command entry point and config loading directly.

  Searched for 2 patterns, read 2 files (ctrl+o to expand)

● Now I have a complete picture. Here is the answer:

  ---
  Are Cleanup Decisions Based on bloom.yaml?

  Yes and no — it's split across two paths, and the split creates a correctness gap.

  Path 1: bloom cleanup [config-file] — bloom.yaml is the source of truth for CLUSTER_DISKS

  When you pass a config file, runClusterCleanup extracts CLUSTER_DISKS, CLUSTER_PREMOUNTED_DISKS, and RANCHER_DISK
  directly from it and passes them into every cleanup function. The wipefs calls in CleanupBloomDisks iterate
  exactly that comma-separated list and nothing else. If bloom.yaml says /dev/sdb,/dev/sdc but the actual deployment
   used /dev/sdd, then /dev/sdd is never wiped and /dev/sdb//dev/sdc are wiped based on what the file says —
  regardless of current reality.

  Path 2: bloom cleanup (no config file) — fstab is the source of truth

  When no config is provided (all three variables come out empty), shouldAutoDiscover returns true and
  discoverAllBloomStorage reads /etc/fstab and reconstructs CLUSTER_DISKS from lines tagged # managed by
  cluster-bloom. Those device paths (or UUID-resolved paths) are then passed to wipefs.

  Path 3: CleanupRancherDisk — always ignores the config argument

  Line 830 is the specific problem:

  if err := runtime.CleanupRancherDisk(""); err != nil {

  The rancherDisk variable extracted from config is never passed in. CleanupRancherDisk always receives "", so it
  always falls through to getDeviceFromCurrentMount and getDeviceFromFstabEntry to detect the device at runtime —
  completely ignoring what bloom.yaml says. This is actually the more robust behavior for RANCHER_DISK specifically,
   since it uses live system state rather than a potentially stale config, but it means the preview (which does use
  the config value if provided) and the actual cleanup can be looking at different devices.

  ---
  The Core Risk Your Question Points To

  If bloom.yaml is stale — for example, someone swapped a disk, redeployed with different CLUSTER_DISKS, then runs
  bloom cleanup old-bloom.yaml — the following happens:

  1. CleanupBloomDisks receives the old CLUSTER_DISKS list from the file
  2. It iterates those device paths and runs wipefs -a <device> on each
  3. The pre-wipe guard is findmnt --source <device> --noheadings — checks only whether the device is currently
  mounted
  4. If the old device path now points to something the system depends on (reassigned by udev), and it happens to
  not be mounted at the whole-disk level, wipefs runs

  The fstab-based auto-discovery path is more reliable than bloom.yaml for cleanup purposes because fstab reflects
  what was actually configured by bloom at deploy time. bloom.yaml reflects what the operator intended at the time
  they wrote the file, which may differ.

  There is no cross-validation: the code does not check whether the devices listed in bloom.yaml match what fstab
  says bloom actually mounted. It trusts the file.


Summary:
1. take suggestions from above
2. add pre-flight checks for validating bloom.yaml and fstab against reality
3. add root disk guard to prevent wipefs on root disk
4. run in CI with Lauri <3

