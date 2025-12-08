# Bloom V2 Robot Framework Tests

Automated tests for Bloom V2 Web UI using Robot Framework.

## Test Structure

```
tests/robot/
├── api/              # API endpoint tests
│   ├── schema.robot
│   ├── validate.robot
│   └── generate.robot
├── webui/            # Web UI interaction tests
│   └── form.robot
├── resources/        # Shared test resources
│   └── common.robot
└── run_tests.sh      # Test runner script
```

## Prerequisites

- Python 3.8+
- bloom-v2 binary built (`make build`)
- Robot Framework and dependencies (installed automatically by run_tests.sh)

## Running Tests

### All Tests

```bash
cd tests/robot
./run_tests.sh
```

### Specific Test Suite

```bash
./run_tests.sh api/schema.robot
./run_tests.sh webui/form.robot
```

### API Tests Only

```bash
./run_tests.sh api/
```

### Web UI Tests Only

```bash
./run_tests.sh webui/
```

## Test Coverage

### API Tests (`api/`)

- **schema.robot**: Tests `/api/schema` endpoint
  - Returns valid JSON
  - Contains all V1 fields
  - Arguments have required properties

- **validate.robot**: Tests `/api/validate` endpoint
  - Valid first node configuration
  - Valid additional node configuration
  - Missing required fields
  - Invalid enum values
  - Conditional field requirements

- **generate.robot**: Tests `/api/generate` endpoint
  - Creates valid YAML
  - Rejects invalid configuration
  - Includes all provided fields
  - Handles boolean values correctly

### Web UI Tests (`webui/`)

- **form.robot**: Tests browser-based interactions
  - Form rendering from schema
  - Field visibility based on dependencies
  - Validation button behavior
  - Generate button workflow
  - YAML preview rendering
  - Download button functionality
  - Boolean formatting in YAML
  - Conditional field toggling

## Test Results

Results are saved in `results/` directory:
- `report.html`: High-level test execution report
- `log.html`: Detailed test execution log with screenshots (for UI tests)
- `output.xml`: Raw test results in XML format

## Dependencies

The test runner automatically installs:
- robotframework
- robotframework-requests (for API tests)
- robotframework-browser (for UI tests with Playwright)

## Configuration

Tests use these variables (configurable via command line):
- `${BASE_URL}`: Web UI URL (default: http://localhost:8080)
- `${BROWSER}`: Browser for UI tests (default: chromium)
- `${HEADLESS}`: Run browser in headless mode (default: true)

Example with custom configuration:

```bash
robot --variable BASE_URL:http://localhost:9000 --variable HEADLESS:false webui/
```
