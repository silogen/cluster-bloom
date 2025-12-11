*** Settings ***
Documentation     UI functionality tests for Bloom V2 Web UI
Library           Browser
Resource          keywords.resource

*** Variables ***
${BASE_URL}       http://localhost:62080

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

    # Set storage option (required by one-of constraint)
    Check Checkbox    id=NO_DISKS_FOR_CLUSTER

    # Submit form
    Click    button[type="submit"]
    Sleep    1s

    # Check if preview appeared and log form values if not
    TRY
        Wait For Elements State    id=preview    visible    timeout=5s
    EXCEPT
        Log To Console    ${\n}Form submission failed - logging debug info
        Log All Form Values
        ${error_visible}=    Get Element States    id=error
        ${has_error}=    Evaluate    "visible" in """${error_visible}"""
        IF    ${has_error}
            ${error_text}=    Get Text    id=error
            Log To Console    Error message: ${error_text}
        ELSE
            Log To Console    No error message visible
        END
        Fail    Preview did not appear - see console output above for form values and any error
    END

    ${yaml}=    Get Text    id=yaml-preview
    Should Contain    ${yaml}    DOMAIN: cluster.example.com
