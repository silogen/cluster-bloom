*** Settings ***
Documentation     Form validation tests for Bloom V2 Web UI
Library           Browser

*** Variables ***
${BASE_URL}       http://localhost:62080

*** Test Cases ***
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
