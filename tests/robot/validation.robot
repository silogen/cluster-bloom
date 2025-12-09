*** Settings ***
Documentation     Field validation tests for Bloom V2 Web UI
Library           Browser

*** Variables ***
${BASE_URL}       http://localhost:62080

*** Test Cases ***
Test Domain Field Validation
    [Documentation]    Verify DOMAIN field accepts valid domains and rejects invalid ones
    New Page    ${BASE_URL}
    Wait For Elements State    id=DOMAIN    visible    timeout=10s

    # Test valid domain
    Fill Text    id=DOMAIN    example.com
    Click    id=GPU_NODE
    Get Element States    id=DOMAIN    validate    value    ==    example.com

    # Test invalid domain (uppercase)
    Fill Text    id=DOMAIN    Example.com
    Click    id=GPU_NODE
    ${errorText}=    Get Text    id=error-DOMAIN
    Should Not Be Empty    ${errorText}

Test IP Address Field Validation
    [Documentation]    Verify SERVER_IP validates IP addresses
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Make SERVER_IP visible
    Click    id=FIRST_NODE
    Wait For Elements State    id=SERVER_IP    visible    timeout=2s

    # Test invalid IP - should show error
    Fill Text    id=SERVER_IP    999.999.999.999
    Click    id=CONTROL_PLANE
    Sleep    0.5s
    ${errorText}=    Get Text    id=error-SERVER_IP
    Should Not Be Empty    ${errorText}

    # Test valid public IP - should clear error
    Fill Text    id=SERVER_IP    203.0.113.1
    Click    id=CONTROL_PLANE
    Sleep    0.5s
    ${errorText}=    Get Text    id=error-SERVER_IP
    Should Be Empty    ${errorText}

Test Required Fields Validation
    [Documentation]    Verify required fields prevent form submission
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Leave DOMAIN empty (required field)
    Fill Text    id=DOMAIN    ${EMPTY}

    # Try to submit - should fail validation
    Click    button[type="submit"]
    # Form should not submit, preview should stay hidden
    Get Element States    id=preview    not contains    visible
