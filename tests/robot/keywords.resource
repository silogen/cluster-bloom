*** Settings ***
Documentation     Shared keywords for Robot Framework tests

*** Keywords ***
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
