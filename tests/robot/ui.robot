*** Settings ***
Documentation     UI functionality tests for Bloom V2 Web UI
Library           Browser
Resource          keywords.resource

*** Test Cases ***
Test Web UI Loads Successfully
    [Documentation]    Verify the web UI loads in browser
    New Page    ${BASE_URL}
    Get Title    ==    Cluster-Bloom Configuration Generator
    Get Text    h1    ==    Cluster-Bloom Configuration Generator

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
    Setup Minimal Valid First Node Config
    Check Checkbox    id=NO_DISKS_FOR_CLUSTER
    Submit And Wait For Preview
    ${yaml}=    Get Text    id=yaml-preview
    Should Contain    ${yaml}    DOMAIN: cluster.example.com

Test Required Fields Validation
    [Documentation]    Verify required fields prevent form submission
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s
    Fill Text    id=DOMAIN    ${EMPTY}
    Click    button[type="submit"]
    Get Element States    id=preview    not contains    visible
