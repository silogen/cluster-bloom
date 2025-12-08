# Bloom V2 Web UI Testing

## Overview

Created comprehensive Robot Framework test suite for Bloom V2 Web UI. Tests cover both API endpoints and browser-based UI interactions.

## Test Structure

```
tests/robot/
├── api/                    # API endpoint tests (no browser required)
│   ├── schema.robot        # GET /api/schema tests
│   ├── validate.robot      # POST /api/validate tests
│   └── generate.robot      # POST /api/generate tests
├── webui/                  # Browser-based UI tests
│   └── form.robot          # Form interaction tests
├── resources/              # Shared test resources
│   └── common.robot        # Keywords for server management
├── run_tests.sh            # Test runner script
└── README.md               # Test documentation
```

## Test Coverage

### API Tests (17 test cases)

**schema.robot** (4 tests):
- Schema endpoint returns 200 OK
- Returns valid JSON structure
- Contains all expected V1 fields (FIRST_NODE, DOMAIN, GPU_NODE, CLUSTER_DISKS)
- Arguments have required properties (key, type, description)

**validate.robot** (5 tests):
- Accepts valid first node configuration
- Accepts valid additional node configuration
- Rejects missing required fields (DOMAIN when FIRST_NODE=true)
- Rejects invalid enum values (CERT_OPTION)
- Requires SERVER_IP for additional nodes (FIRST_NODE=false)

**generate.robot** (4 tests):
- Creates valid YAML from configuration
- Rejects invalid configuration (returns 400)
- Includes all provided fields in YAML
- Handles boolean values correctly (lowercase true/false)

### Web UI Tests (10 test cases)

**form.robot** (10 tests):
- Form renders all fields from schema
- FIRST_NODE toggle shows/hides DOMAIN field
- FIRST_NODE=false shows SERVER_IP field
- GPU_NODE toggle shows/hides ROCM_BASE_URL field
- Validation button shows errors for invalid config
- Valid first node config validates successfully
- Generate button creates YAML preview
- Download button appears after generation
- Boolean fields format correctly in YAML (lowercase)
- USE_CERT_MANAGER toggle shows/hides CERT_OPTION field

## Running Tests

### Prerequisites

Tests require Python environment with Robot Framework. In production environment:

```bash
pip3 install robotframework robotframework-requests robotframework-browser
rfbrowser init
```

### Using Makefile

```bash
# Run all tests
make test

# Run API tests only (faster, no browser)
make test-api

# Run Web UI tests only
make test-webui

# Build binary without tests
make build
```

### Direct Execution

```bash
cd tests/robot
./run_tests.sh              # All tests
./run_tests.sh api/         # API tests only
./run_tests.sh webui/       # Web UI tests only
```

## Test Results

Results saved in `tests/robot/results/`:
- `report.html` - High-level test execution report
- `log.html` - Detailed test log with screenshots (UI tests)
- `output.xml` - Raw results in XML format

## Current Status

**Created**: All test files written and structured
**Verified**: Binary runs successfully, help text works
**Blocked**: Container environment lacks pip/Robot Framework for execution

Tests are ready to run in environment with Python package manager (pip) available. Alternatively, can be run on host system or in CI/CD pipeline.

## Next Steps (After Test Execution)

1. Run tests in proper Python environment
2. Fix any failures found
3. Add tests to CI/CD pipeline
4. Consider additional edge cases:
   - Empty string handling
   - Special characters in fields
   - Very long input values
   - Concurrent API requests
   - Browser compatibility (Firefox, Safari)

## V1 Fields Tested

All critical V1 configuration fields are covered:
- FIRST_NODE (boolean, conditional visibility trigger)
- DOMAIN (string, required when FIRST_NODE=true)
- GPU_NODE (boolean, shows ROCM_BASE_URL)
- CLUSTER_DISKS (string, disk list)
- SERVER_IP (string, required when FIRST_NODE=false)
- JOIN_TOKEN (string, for additional nodes)
- USE_CERT_MANAGER (boolean, hides CERT_OPTION)
- CERT_OPTION (enum: existing|generate)
- NO_DISKS_FOR_CLUSTER (boolean)
- ROCM_BASE_URL (string, conditional)

## Test Philosophy

- **Fail fast**: Tests verify critical path first (schema → validate → generate)
- **Real interactions**: UI tests use actual browser (Playwright via Browser library)
- **API-first**: Separated API tests from UI tests for faster feedback
- **Comprehensive coverage**: Both happy path and error cases tested
- **Maintainable**: Common resources extracted, DRY principle followed
