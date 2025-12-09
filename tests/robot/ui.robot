*** Settings ***
Documentation     UI functionality tests for Bloom V2 Web UI
Library           Browser

*** Variables ***
${BASE_URL}       http://localhost:62080

*** Test Cases ***
Test Web UI Loads Successfully
    [Documentation]    Verify the web UI loads in browser
    New Page    ${BASE_URL}
    Get Title    ==    Bloom Configuration Generator
    Get Text    h1    ==    Bloom Configuration Generator

Test Form Elements Are Visible
    [Documentation]    Verify form elements render correctly
    New Page    ${BASE_URL}
    Wait For Elements State    id=form-fields    visible    timeout=10s
    Get Element Count    .form-group    >    0
    Get Element Count    .section-header    ==    6

Test Conditional Fields Work
    [Documentation]    Verify conditional field visibility works
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Uncheck FIRST_NODE - additional node fields should appear
    Click    id=FIRST_NODE
    Wait For Elements State    id=SERVER_IP    visible    timeout=2s
    Get Element States    id=DOMAIN    not contains    visible

    # Check FIRST_NODE - domain should appear
    Click    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    visible    timeout=2s
    Get Element States    id=SERVER_IP    not contains    visible

Test Form Generates YAML
    [Documentation]    Verify form submission generates YAML preview
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Ensure FIRST_NODE is checked
    ${checked}=    Get Checkbox State    id=FIRST_NODE
    IF    '${checked}' == 'false'
        Check Checkbox    id=FIRST_NODE
    END
    Wait For Elements State    id=DOMAIN    visible    timeout=2s

    # Fill in required fields with valid values
    Fill Text    id=DOMAIN    cluster.example.com
    Select Options By    id=CERT_OPTION    value    generate
    # Trigger blur to clear any validation errors
    Click    id=GPU_NODE

    # Submit form
    Click    button[type="submit"]
    Wait For Elements State    id=preview    visible    timeout=5s
    ${yaml}=    Get Text    id=yaml-preview
    Should Contain    ${yaml}    DOMAIN: cluster.example.com
