*** Settings ***
Documentation     Dynamic constraint validation tests generated from schema
Library           Browser
Library           ConstraintTestGenerator.py

*** Variables ***
${BASE_URL}       http://localhost:62080

*** Test Cases ***
Test All Mutually Exclusive Constraints
    [Documentation]    Dynamically test all mutually_exclusive constraints from schema
    ${constraints}=    Get Mutually Exclusive Constraints
    FOR    ${fields}    IN    @{constraints}
        Test Mutually Exclusive Pair    ${fields}
    END

Test All One-Of Constraints
    [Documentation]    Dynamically test all one_of constraints from schema
    ${constraints}=    Get One Of Constraints
    FOR    ${constraint}    IN    @{constraints}
        Test One Of Constraint    ${constraint}
    END

*** Keywords ***
Setup Valid Config
    [Documentation]    Setup minimal valid configuration
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s
    Check Checkbox    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    visible    timeout=2s
    Fill Text    id=DOMAIN    cluster.example.com
    Select Options By    id=CERT_OPTION    value    generate

Set Field Value
    [Arguments]    ${field}    ${value}
    [Documentation]    Set a field value based on type

    ${element_exists}=    Run Keyword And Return Status    Get Element States    id=${field}
    IF    not ${element_exists}
        Log    Field ${field} not found, skipping
        RETURN
    END

    ${field_type}=    Get Attribute    id=${field}    type
    ${value_type}=    Evaluate    type($value).__name__

    IF    "${field_type}" == "checkbox"
        IF    ${value} or "${value}" == "true"
            Check Checkbox    id=${field}
        ELSE
            Uncheck Checkbox    id=${field}
        END
    ELSE
        Fill Text    id=${field}    ${value}
    END

Test Mutually Exclusive Pair
    [Arguments]    ${fields}
    [Documentation]    Test that fields ${fields} are mutually exclusive

    ${field1}=    Set Variable    ${fields}[0]
    ${field2}=    Set Variable    ${fields}[1]

    Log    Testing mutually exclusive: ${field1} and ${field2}

    Setup Valid Config
    Check Checkbox    id=NO_DISKS_FOR_CLUSTER

    # Get valid example values from schema
    ${examples}=    Get Valid Examples For Fields    ${fields}
    ${value1}=    Set Variable    ${examples}[${field1}]
    ${value2}=    Set Variable    ${examples}[${field2}]

    # Set both fields (should fail)
    Set Field Value    ${field1}    ${value1}
    Set Field Value    ${field2}    ${value2}

    # Submit form
    Click    button[type="submit"]
    Sleep    1s

    # Should show error
    ${error_visible}=    Get Element States    id=error    *=    visible
    ${has_error}=    Evaluate    "visible" in """${error_visible}"""
    Should Be True    ${has_error}    Expected error when both ${field1} and ${field2} are set

    # Verify error message mentions mutually exclusive
    ${error_text}=    Get Text    id=error
    Should Contain    ${error_text}    mutually exclusive    ignore_case=True

Test One Of Constraint
    [Arguments]    ${constraint}
    [Documentation]    Test one-of constraint dynamically

    ${fields}=    Set Variable    ${constraint}[fields]
    ${error_msg}=    Set Variable    ${constraint}[error]
    ${field_count}=    Get Length    ${fields}

    Log    Testing one-of constraint with ${field_count} fields: ${fields}

    # Test scenario 1: No fields set (should fail)
    Test No Fields Set    ${fields}

    # Test scenario 2: Exactly one field set (should pass)
    FOR    ${field}    IN    @{fields}
        Test Exactly One Field Set    ${fields}    ${field}
    END

    # Test scenario 3: Multiple fields set (should fail)
    IF    ${field_count} >= 2
        Test Multiple Fields Set    ${fields}[0]    ${fields}[1]
    END

Test No Fields Set
    [Arguments]    ${fields}
    [Documentation]    Test when no fields are set (should show error)

    Setup Valid Config

    # Set all fields to empty/false
    FOR    ${field}    IN    @{fields}
        ${element_exists}=    Run Keyword And Return Status    Get Element States    id=${field}
        IF    ${element_exists}
            ${field_type}=    Get Attribute    id=${field}    type
            IF    "${field_type}" == "checkbox"
                Uncheck Checkbox    id=${field}
            ELSE
                Fill Text    id=${field}    ${EMPTY}
            END
        END
    END

    # Submit form
    Click    button[type="submit"]
    Sleep    1s

    # Should show error
    ${error_visible}=    Get Element States    id=error    *=    visible
    ${has_error}=    Evaluate    "visible" in """${error_visible}"""
    Should Be True    ${has_error}    Expected error when no fields are set

Test Exactly One Field Set
    [Arguments]    ${all_fields}    ${field_to_set}
    [Documentation]    Test when exactly one field is set (should succeed)

    Setup Valid Config

    # Clear all fields first
    FOR    ${field}    IN    @{all_fields}
        ${element_exists}=    Run Keyword And Return Status    Get Element States    id=${field}
        IF    ${element_exists}
            ${field_type}=    Get Attribute    id=${field}    type
            IF    "${field_type}" == "checkbox"
                Uncheck Checkbox    id=${field}
            ELSE
                Fill Text    id=${field}    ${EMPTY}
            END
        END
    END

    # Set only the specified field with valid value
    ${example}=    Get Valid Example For Field    ${field_to_set}
    Set Field Value    ${field_to_set}    ${example}

    # Submit form
    Click    button[type="submit"]
    Wait For Elements State    id=preview    visible    timeout=5s

    # Should succeed and show preview
    ${yaml}=    Get Text    id=yaml-preview
    Should Not Be Empty    ${yaml}

Test Multiple Fields Set
    [Arguments]    ${field1}    ${field2}
    [Documentation]    Test when multiple fields are set (should show error)

    Setup Valid Config

    # Get valid examples for both fields
    ${example1}=    Get Valid Example For Field    ${field1}
    ${example2}=    Get Valid Example For Field    ${field2}

    # Set both fields
    Set Field Value    ${field1}    ${example1}
    Set Field Value    ${field2}    ${example2}

    # Submit form
    Click    button[type="submit"]
    Sleep    1s

    # Should show error
    ${error_visible}=    Get Element States    id=error    *=    visible
    ${has_error}=    Evaluate    "visible" in """${error_visible}"""
    Should Be True    ${has_error}    Expected error when multiple fields are set

    # Verify error message
    ${error_text}=    Get Text    id=error
    Should Contain Any    ${error_text}    Exactly one    must be set    ignore_case=True
