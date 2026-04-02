#!/bin/bash

# Longhorn Disk Setup Automation Script
# This script automates the disk formatting, mounting, and fstab configuration
# for Longhorn storage on cluster-bloom nodes
# 
# Features:
# - RAID detection and removal (with backup/restore capability)
# - Disk space analysis and recommendations
# - Dry-run mode for planning
# - Safe disk formatting and mounting

set -euo pipefail

# Configuration
MOUNT_BASE="/mnt/disk"
FILESYSTEM_TYPE="ext4"
FSTAB_OPTIONS="defaults,nofail 0 2"
BACKUP_DIR="/root/longhorn-raid-backup"

# Runtime flags
DRY_RUN=false
REMOVE_RAID=false
FORCE_RAID_REMOVAL=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN=true
                log_info "Running in dry-run mode - no changes will be made"
                shift
                ;;
            --remove-raid)
                REMOVE_RAID=true
                shift
                ;;
            --force-raid-removal)
                FORCE_RAID_REMOVAL=true
                REMOVE_RAID=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Show help
show_help() {
    cat << EOF
Longhorn Disk Setup Automation Script

Usage: $0 [OPTIONS]

OPTIONS:
    --dry-run              Show what would be done without making changes
    --remove-raid          Remove detected RAID configurations
    --force-raid-removal   Force RAID removal without confirmation
    -h, --help            Show this help message

Examples:
    $0 --dry-run          # See recommendations without changes
    $0                    # Interactive setup
    $0 --remove-raid      # Handle RAID removal if needed

This script will:
1. Check disk space requirements
2. Detect and optionally remove software RAID
3. Set up Longhorn-compatible disk configuration
4. Create proper fstab entries for persistent mounting

EOF
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Check disk space requirements
check_disk_space() {
    log_info "Checking disk space requirements..."
    
    local root_available_kb root_available_gb var_available_kb var_available_gb
    
    # Get available space in KB
    root_available_kb=$(df --output=avail / | tail -1)
    root_available_gb=$((root_available_kb / 1024 / 1024))
    
    log_info "Root partition available space: ${root_available_gb}GB"
    
    # Check minimum requirements
    if [[ $root_available_kb -lt 10485760 ]]; then  # Less than 10GB
        log_error "Root partition has less than 10GB available (minimum requirement)"
        return 1
    elif [[ $root_available_kb -lt 20971520 ]]; then  # Less than 20GB
        log_warning "Root partition has less than 20GB available (recommended)"
        log_warning "Consider creating dedicated /var/lib/rancher partition"
        echo "RECOMMEND_VAR_LIB_RANCHER=true"
    else
        log_success "Root partition has sufficient space (${root_available_gb}GB available)"
        echo "RECOMMEND_VAR_LIB_RANCHER=false"
    fi
    
    # Check /var if separately mounted
    if mountpoint -q /var; then
        var_available_kb=$(df --output=avail /var | tail -1)
        var_available_gb=$((var_available_kb / 1024 / 1024))
        log_info "/var partition available space: ${var_available_gb}GB"
        
        if [[ $var_available_kb -lt 5242880 ]]; then  # Less than 5GB
            log_warning "/var partition has less than 5GB available (recommended for container images)"
        fi
    fi
}

# Detect RAID arrays
detect_raid() {
    log_info "Checking for software RAID configurations..."
    
    if [[ ! -f /proc/mdstat ]]; then
        log_info "No software RAID support detected"
        return 1
    fi
    
    local md_arrays
    md_arrays=$(awk '/^md/ {print $1}' /proc/mdstat)
    
    if [[ -z "$md_arrays" ]]; then
        log_info "No active RAID arrays found"
        return 1
    fi
    
    log_warning "Found active RAID arrays:"
    cat /proc/mdstat
    echo ""
    
    log_warning "⚠️  Longhorn does NOT support RAID configurations!"
    log_warning "RAID arrays must be removed before configuring Longhorn storage"
    
    return 0
}

# Backup RAID configuration
backup_raid_config() {
    log_info "Backing up RAID configuration..."
    
    if [[ $DRY_RUN == true ]]; then
        log_info "[DRY-RUN] Would backup RAID config to $BACKUP_DIR"
        return 0
    fi
    
    # Create backup directory
    mkdir -p "$BACKUP_DIR"
    
    # Backup mdadm config
    if command -v mdadm >/dev/null 2>&1; then
        mdadm --detail --scan > "$BACKUP_DIR/mdadm.conf.backup"
        cp /proc/mdstat "$BACKUP_DIR/mdstat.backup"
        
        # Backup individual array details
        for md_device in /dev/md*; do
            if [[ -b "$md_device" ]]; then
                local md_name
                md_name=$(basename "$md_device")
                mdadm --detail "$md_device" > "$BACKUP_DIR/${md_name}_detail.backup" 2>/dev/null || true
            fi
        done
        
        log_success "RAID configuration backed up to $BACKUP_DIR"
        log_info "Backup includes: mdadm.conf, mdstat, and individual array details"
    else
        log_error "mdadm not found - cannot backup RAID configuration"
        return 1
    fi
}

# Remove RAID arrays
remove_raid_arrays() {
    log_warning "Removing RAID arrays..."
    
    if [[ $DRY_RUN == true ]]; then
        log_info "[DRY-RUN] Would remove the following RAID arrays:"
        awk '/^md/ {print "  /dev/" $1}' /proc/mdstat
        return 0
    fi
    
    # Get list of RAID arrays
    local md_arrays
    md_arrays=$(awk '/^md/ {print "/dev/" $1}' /proc/mdstat)
    
    if [[ -z "$md_arrays" ]]; then
        log_info "No RAID arrays to remove"
        return 0
    fi
    
    # Confirm removal unless forced
    if [[ $FORCE_RAID_REMOVAL != true ]]; then
        echo ""
        log_warning "This will DESTROY the following RAID arrays:"
        printf '%s\n' $md_arrays
        echo ""
        read -p "Are you sure you want to proceed? (yes/no): " -r
        
        if [[ $REPLY != "yes" ]]; then
            log_info "RAID removal cancelled by user"
            return 1
        fi
    fi
    
    # Stop and remove arrays
    for md_array in $md_arrays; do
        log_info "Stopping RAID array: $md_array"
        
        # Unmount if mounted
        if mount | grep -q "$md_array"; then
            log_info "Unmounting $md_array"
            umount "$md_array" || log_warning "Failed to unmount $md_array"
        fi
        
        # Stop the array
        mdadm --stop "$md_array" || log_warning "Failed to stop $md_array"
        
        # Remove the array
        mdadm --remove "$md_array" 2>/dev/null || true
    done
    
    # Zero superblocks on member disks
    log_info "Clearing RAID superblocks on member disks..."
    for disk in /dev/sd* /dev/nvme*n1; do
        if [[ -b "$disk" ]]; then
            mdadm --zero-superblock "$disk" 2>/dev/null || true
        fi
    done
    
    log_success "RAID arrays removed successfully"
    log_info "Individual disks are now available for Longhorn use"
}

# Restore RAID configuration (if needed)
restore_raid_config() {
    local backup_file="$BACKUP_DIR/mdadm.conf.backup"
    
    if [[ ! -f "$backup_file" ]]; then
        log_error "No RAID backup found at $backup_file"
        return 1
    fi
    
    log_warning "Restoring RAID configuration from backup..."
    log_warning "This will recreate the original RAID arrays"
    
    read -p "Are you sure you want to restore RAID? (yes/no): " -r
    if [[ $REPLY != "yes" ]]; then
        log_info "RAID restoration cancelled"
        return 1
    fi
    
    # Restore using backed up configuration
    while IFS= read -r line; do
        if [[ $line == ARRAY* ]]; then
            log_info "Restoring: $line"
            eval "mdadm --assemble $line"
        fi
    done < "$backup_file"
    
    log_success "RAID configuration restored"
    log_info "Check /proc/mdstat to verify arrays"
}

# Discover candidate disks (prioritized: NVMe > SSD > HDD)
discover_disks() {
    local disks=()
    
    # Priority 1: NVMe drives (excluding those in RAID)
    for disk in /dev/nvme*n1; do
        if [[ -b "$disk" ]] && ! is_disk_in_raid "$disk"; then
            disks+=("$disk")
        fi
    done
    
    # Priority 2: SATA/SCSI drives (excluding sda and those in RAID)
    for disk in /dev/sd[b-z]; do
        if [[ -b "$disk" ]] && ! is_disk_in_raid "$disk"; then
            disks+=("$disk")
        fi
    done
    
    printf '%s\n' "${disks[@]}" | sort
}

# Check if disk is part of a RAID array
is_disk_in_raid() {
    local disk="$1"
    
    # Check if disk or its partitions are part of any md array
    if [[ -f /proc/mdstat ]]; then
        local disk_base
        disk_base=$(basename "$disk")
        grep -q "$disk_base" /proc/mdstat 2>/dev/null
    else
        return 1
    fi
}

# Check if disk is formatted
is_disk_formatted() {
    local disk="$1"
    blkid -s UUID -o value "$disk" >/dev/null 2>&1
}

# Get disk UUID
get_disk_uuid() {
    local disk="$1"
    blkid -s UUID -o value "$disk"
}

# Check if disk is mounted
is_disk_mounted() {
    local disk="$1"
    mount | grep -q "^$disk"
}

# Get next available mount point
get_next_mount_point() {
    local counter=0
    while [[ -d "${MOUNT_BASE}${counter}" ]]; do
        if mount | grep -q "${MOUNT_BASE}${counter}"; then
            ((counter++))
        else
            break
        fi
    done
    echo "${MOUNT_BASE}${counter}"
}

# Format disk with ext4
format_disk() {
    local disk="$1"
    
    if [[ $DRY_RUN == true ]]; then
        log_info "[DRY-RUN] Would format $disk with $FILESYSTEM_TYPE"
        return 0
    fi
    
    log_warning "Formatting $disk with $FILESYSTEM_TYPE (THIS WILL DESTROY ALL DATA)"
    read -p "Are you sure you want to format $disk? (yes/no): " -r
    
    if [[ $REPLY == "yes" ]]; then
        mkfs.ext4 -F "$disk"
        log_success "Formatted $disk successfully"
    else
        log_info "Skipping formatting of $disk"
        return 1
    fi
}

# Create mount point
create_mount_point() {
    local mount_point="$1"
    
    if [[ $DRY_RUN == true ]]; then
        if [[ ! -d "$mount_point" ]]; then
            log_info "[DRY-RUN] Would create mount point: $mount_point"
        else
            log_info "[DRY-RUN] Mount point already exists: $mount_point"
        fi
        return 0
    fi
    
    if [[ ! -d "$mount_point" ]]; then
        mkdir -p "$mount_point"
        log_success "Created mount point: $mount_point"
    else
        log_info "Mount point already exists: $mount_point"
    fi
}

# Add entry to fstab
add_to_fstab() {
    local uuid="$1"
    local mount_point="$2"
    
    local fstab_entry="UUID=$uuid $mount_point $FILESYSTEM_TYPE $FSTAB_OPTIONS"
    
    if [[ $DRY_RUN == true ]]; then
        log_info "[DRY-RUN] Would add to /etc/fstab: $fstab_entry"
        return 0
    fi
    
    # Check if entry already exists
    if grep -q "$uuid" /etc/fstab; then
        log_warning "Entry for UUID $uuid already exists in /etc/fstab"
        return 0
    fi
    
    # Add entry to fstab
    echo "$fstab_entry" >> /etc/fstab
    log_success "Added to /etc/fstab: $fstab_entry"
}

# Validate mounts
validate_mounts() {
    log_info "Validating mounts..."
    
    if [[ $DRY_RUN == true ]]; then
        log_info "[DRY-RUN] Would validate all fstab entries can mount"
        log_info "[DRY-RUN] Current Longhorn mounts:"
        df -h | grep "${MOUNT_BASE}" || log_info "[DRY-RUN] No Longhorn disks currently mounted"
        return 0
    fi
    
    # Test mount all
    if mount -a; then
        log_success "All fstab entries mounted successfully"
    else
        log_error "Failed to mount some entries from fstab"
        return 1
    fi
    
    # Show mounted disks
    echo ""
    log_info "Currently mounted Longhorn storage disks:"
    df -h | grep "${MOUNT_BASE}" || log_warning "No Longhorn disks currently mounted"
}

# Display summary
display_summary() {
    echo ""
    echo "=========================================="
    log_info "Longhorn Disk Setup Summary"
    echo "=========================================="
    
    echo ""
    log_info "Mounted disks:"
    df -h | grep "${MOUNT_BASE}" || echo "None"
    
    echo ""
    log_info "fstab entries for Longhorn disks:"
    grep "${MOUNT_BASE}" /etc/fstab || echo "None"
    
    echo ""
    log_info "Next steps:"
    echo "1. Access Longhorn UI at: https://longhorn.cluster-name"
    echo "2. Navigate to Node tab"
    echo "3. For each node, select 'Edit node and disks'"
    echo "4. Add each mounted disk path (e.g., /mnt/disk0, /mnt/disk1)"
    echo "5. Enable scheduling for each disk"
}

# Main function
main() {
    # Parse command line arguments
    parse_args "$@"
    
    log_info "Starting Longhorn disk setup automation..."
    if [[ $DRY_RUN == true ]]; then
        log_info "=== DRY RUN MODE - No changes will be made ==="
    fi
    
    # Check prerequisites
    check_root
    
    # Check disk space requirements
    check_disk_space
    echo ""
    
    # Check for RAID and handle if necessary
    if detect_raid; then
        echo ""
        if [[ $REMOVE_RAID == true ]] || [[ $DRY_RUN == true ]]; then
            backup_raid_config
            remove_raid_arrays
        else
            log_warning "RAID detected but not removing. Use --remove-raid to handle this."
            log_warning "Longhorn requires individual disks, not RAID arrays."
            echo ""
            log_info "Options:"
            echo "  1. Run with --remove-raid to safely remove RAID"
            echo "  2. Run with --dry-run to see what would be done"
            echo "  3. Manually remove RAID configuration first"
            exit 1
        fi
        echo ""
    fi
    
    # Discover available disks
    log_info "Discovering candidate disks..."
    mapfile -t candidate_disks < <(discover_disks)
    
    if [[ ${#candidate_disks[@]} -eq 0 ]]; then
        log_warning "No candidate disks found"
        if [[ -f /proc/mdstat ]] && grep -q "^md" /proc/mdstat; then
            log_info "Note: Disks may be in RAID arrays. Use --remove-raid to make them available."
        fi
        exit 0
    fi
    
    log_info "Found ${#candidate_disks[@]} candidate disk(s):"
    printf '  %s\n' "${candidate_disks[@]}"
    
    echo ""
    
    # Process each disk
    for disk in "${candidate_disks[@]}"; do
        log_info "Processing disk: $disk"
        
        # Skip if already mounted
        if is_disk_mounted "$disk"; then
            log_info "Disk $disk is already mounted, skipping"
            continue
        fi
        
        # Check if formatted
        if ! is_disk_formatted "$disk"; then
            log_warning "Disk $disk appears to be unformatted"
            if ! format_disk "$disk"; then
                continue
            fi
        else
            log_info "Disk $disk is already formatted"
        fi
        
        # Get UUID (skip in dry run if not formatted)
        if [[ $DRY_RUN == true ]] && ! is_disk_formatted "$disk"; then
            log_info "[DRY-RUN] Would assign UUID after formatting"
            uuid="<uuid-after-format>"
        else
            uuid=$(get_disk_uuid "$disk")
            log_info "Disk UUID: $uuid"
        fi
        
        # Get mount point
        mount_point=$(get_next_mount_point)
        log_info "Mount point: $mount_point"
        
        # Create mount point
        create_mount_point "$mount_point"
        
        # Add to fstab
        add_to_fstab "$uuid" "$mount_point"
        
        echo ""
    done
    
    # Validate mounts
    validate_mounts
    
    # Display summary
    display_summary
    
    if [[ $DRY_RUN == true ]]; then
        log_info "=== DRY RUN COMPLETE - No changes were made ==="
        log_info "Run without --dry-run to apply these changes"
    else
        log_success "Longhorn disk setup completed successfully!"
        log_info "This is the way to configure storage that eliminates impurities."
    fi
}

# Run main function
main "$@"