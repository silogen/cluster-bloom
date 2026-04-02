# TLS SAN Array Enhancement - Phase 2B Changelog

## Overview
Enhanced `ADDITIONAL_TLS_SAN_URLS` from legacy comma-separated strings to modern array format with comprehensive validation.

## Key Changes

### Schema & Configuration
- **Changed `ADDITIONAL_TLS_SAN_URLS` type** from `domainList` to `seq` (array)
- **Added pattern validation** to block wildcard domains (`*.domain.com`)
- **Maintains backward compatibility** with legacy string format

### UI Enhancements
- **Real-time validation** with visual feedback (red/green borders)
- **Immediate error messages** for wildcard domains
- **Form submission blocking** until validation passes
- **Enhanced placeholders** and button text for better UX

### Backend Processing
- **Robust array processing** in Ansible playbook
- **Wildcard filtering** with warning messages
- **Server-side validation** as safety net
- **Support for both array and string formats**

### Certificate Management
- **Automatic base domain** (`k8s.<DOMAIN>`) inclusion
- **Filtered wildcard domains** to prevent RKE2 failures
- **Clean RKE2 config generation** with validated domains

## Migration Path

**Before (Legacy):**
```yaml
ADDITIONAL_TLS_SAN_URLS: "api.example.com,kubectl.example.com"
```

**After (Enhanced):**
```yaml
ADDITIONAL_TLS_SAN_URLS:
  - "api.example.com"
  - "kubectl.example.com"
```

## Validation Features

### UI Validation
- Immediate feedback on invalid domains
- Visual indicators (colored borders)
- Clear error messages
- Form submission prevention

### Server Validation
- Pattern matching against wildcard domains
- Domain format validation
- Comprehensive error reporting
- Failsafe wildcard detection

### Backend Processing
- Wildcard filtering with warnings
- Graceful handling of mixed formats
- Automatic cleanup of invalid entries

## Benefits

1. **User-Friendly**: Clear array syntax instead of comma-separated strings
2. **Validation**: Prevents invalid configurations that break RKE2
3. **Robust**: Multiple validation layers ensure reliability
4. **Compatible**: Supports both old and new formats during transition
5. **Secure**: Blocks problematic wildcard patterns

## Files Modified

- `pkg/config/bloom.yaml.schema.yaml` - Schema definition
- `pkg/ansible/runtime/playbooks/cluster-bloom.yaml` - Backend processing
- `cmd/web/static/js/form.js` - UI form enhancements
- `cmd/web/static/js/validator.js` - Client-side validation
- `pkg/config/validator.go` - Server-side validation
- `pkg/config/schema.go` - Schema structure
- `pkg/config/schema_loader.go` - Schema parsing

## Documentation Added

- `docs/tls-san-configuration.md` - Comprehensive TLS SAN guide
- Updated `docs/configuration-reference.md` - Enhanced field documentation
- Updated `docs/certificate-management.md` - Added TLS SAN overview
- Updated `docs/README.md` - Added TLS SAN documentation link