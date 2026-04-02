*** Settings ***
Documentation     Shared keywords for Robot Framework tests

*** Variables ***
${BASE_URL}       http://localhost:62080

*** Keywords ***
Setup Minimal Valid First Node Config
    [Documentation]    Setup minimal valid configuration for first node
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s
    Check Checkbox    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    visible    timeout=2s
    Fill Text    id=DOMAIN    cluster.example.com
    Select Options By    id=CERT_OPTION    value    generate

Submit And Wait For Preview
    [Documentation]    Submit form and wait for preview, with error logging on failure
    Click    button[type="submit"]
    Sleep    2s
    TRY
        Wait For Elements State    id=preview    visible    timeout=5s
    EXCEPT
        Log To Console    ${\n}Form submission failed - logging debug info
        Log All Form Values
        ${error_text}=    Get Error Message If Visible
        IF    "${error_text}" != ""
            Log To Console    Error message: ${error_text}
        ELSE
            Log To Console    No error message visible
        END
        Fail    Preview did not appear - see console output above
    END

Should Show Validation Error
    [Documentation]    Verify that validation error is visible
    ${error_visible}=    Get Element States    id=error    *=    visible
    ${has_error}=    Evaluate    "visible" in """${error_visible}"""
    Should Be True    ${has_error}    Expected validation error to be visible

Get Error Message If Visible
    [Documentation]    Return error message text if visible, empty string otherwise
    ${error_visible}=    Get Element States    id=error
    ${has_error}=    Evaluate    "visible" in """${error_visible}"""
    IF    ${has_error}
        ${error_text}=    Get Text    id=error
        RETURN    ${error_text}
    END
    RETURN    ${EMPTY}

Log All Form Values
    [Documentation]    Log all form field values for debugging
    Log To Console    ${\n}===== FORM VALUES =====

    # Get all input fields
    ${inputs}=    Get Elements    input
    FOR    ${input}    IN    @{inputs}
        ${id}=    Get Attribute    ${input}    id
        ${type}=    Get Attribute    ${input}    type
        IF    "${type}" == "checkbox"
            ${value}=    Get Checkbox State    ${input}
        ELSE
            ${value}=    Get Property    ${input}    value
        END
        Log To Console    ${id}: ${value}
    END

    # Get all select fields
    ${selects}=    Get Elements    select
    FOR    ${select}    IN    @{selects}
        ${id}=    Get Attribute    ${select}    id
        ${value}=    Get Property    ${select}    value
        Log To Console    ${id}: ${value}
    END

    Log To Console    ===== END FORM VALUES =====${\n}
