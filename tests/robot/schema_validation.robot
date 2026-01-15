*** Settings ***
Documentation     Schema-driven validation tests - validates all examples from YAML schema
Library           Browser
Library           RequestsLibrary
Library           Collections
Library           OperatingSystem
Library           yaml_loader.py
Resource          keywords.resource

*** Test Cases ***
Test All Schema Examples Through UI
    [Documentation]    Validate all examples from schema through the web UI
    # Load schema and get all field definitions with examples
    ${schema}=    Load Schema Examples

    # For each field type with examples, test through UI
    FOR    ${field_info}    IN    @{schema}
        Test Field Examples    ${field_info}
    END

*** Keywords ***
Test Field Examples
    [Arguments]    ${field_info}
    ${field_name}=    Set Variable    ${field_info['field']}
    ${field_id}=    Set Variable    ${field_info['fieldId']}
    ${valid_examples}=    Set Variable    ${field_info['valid']}
    ${invalid_examples}=    Set Variable    ${field_info['invalid']}
    ${visibility_steps}=    Set Variable    ${field_info.get('visibility', None)}

    Log    Testing field: ${field_name}    console=yes

    # Start fresh for each field
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Make field visible if needed
    IF    ${visibility_steps} is not None
        Execute Visibility Steps    ${visibility_steps}
    END

    Wait For Elements State    id=${field_id}    visible    timeout=5s

    # Check for console errors before testing
    Check No Console Errors    ${field_name}

    # Test all invalid examples first
    FOR    ${invalid_example}    IN    @{invalid_examples}
        Test Invalid Example    ${field_id}    ${field_name}    ${invalid_example}
    END

    # Test all valid examples
    FOR    ${valid_example}    IN    @{valid_examples}
        Test Valid Example    ${field_id}    ${field_name}    ${valid_example}
    END

    # Check for console errors after testing
    Check No Console Errors    ${field_name}

Execute Visibility Steps
    [Arguments]    ${steps}
    FOR    ${step}    IN    @{steps}
        ${action}=    Set Variable    ${step['action']}
        ${target}=    Set Variable    ${step['target']}

        IF    '${action}' == 'select'
            ${value}=    Set Variable    ${step['value']}
            Wait For Elements State    id=${target}    visible    timeout=2s
            Select Options By    id=${target}    value    ${value}
            # Trigger change event to update field visibility (with bubbles: true)
            Evaluate JavaScript    id=${target}    (elem) => elem.dispatchEvent(new Event('change', { bubbles: true }))
            Sleep    0.5s
        ELSE IF    '${action}' == 'check'
            Wait For Elements State    id=${target}    visible    timeout=2s
            Check Checkbox    id=${target}
            # Trigger change event to update field visibility (with bubbles: true)
            Evaluate JavaScript    id=${target}    (elem) => elem.dispatchEvent(new Event('change', { bubbles: true }))
            Sleep    0.5s
        ELSE IF    '${action}' == 'uncheck'
            Wait For Elements State    id=${target}    visible    timeout=2s
            Uncheck Checkbox    id=${target}
            # Trigger change event to update field visibility (with bubbles: true)
            Evaluate JavaScript    id=${target}    (elem) => elem.dispatchEvent(new Event('change', { bubbles: true }))
            Sleep    0.5s
        ELSE IF    '${action}' == 'wait'
            Wait For Elements State    id=${target}    visible    timeout=2s
        END
    END

Test Invalid Example
    [Arguments]    ${field_id}    ${field_name}    ${example}
    # Skip empty string for invalid examples (used for optional fields)
    IF    '${example}' == ''
        RETURN
    END

    Fill Text    id=${field_id}    ${example}
    # Trigger validation by clicking elsewhere (click twice to restore state)
    Click    id=GPU_NODE
    Click    id=GPU_NODE
    Sleep    0.5s

    # Check for console errors after filling this example
    Check No Console Errors    ${field_name} invalid example: ${example}

    ${error_id}=    Set Variable    error-${field_id}
    ${errorText}=    Get Text    id=${error_id}
    Should Not Be Empty    ${errorText}    msg=Field ${field_name} should reject invalid example: ${example}

Test Valid Example
    [Arguments]    ${field_id}    ${field_name}    ${example}
    Fill Text    id=${field_id}    ${example}
    # Trigger validation by clicking elsewhere (click twice to restore state)
    Click    id=GPU_NODE
    Click    id=GPU_NODE
    Sleep    0.5s

    # Check for console errors after filling this example
    Check No Console Errors    ${field_name} valid example: ${example}

    ${error_id}=    Set Variable    error-${field_id}
    ${errorText}=    Get Text    id=${error_id}
    Should Be Empty    ${errorText}    msg=Field ${field_name} should accept valid example: ${example}

Check No Console Errors
    [Arguments]    ${field_name}
    ${console_logs}=    Get Console Log
    FOR    ${log_entry}    IN    @{console_logs}
        ${type}=    Set Variable    ${log_entry}[type]
        ${text}=    Set Variable    ${log_entry}[text]
        IF    '${type}' == 'error'
            Fail    Console error found while testing ${field_name}: ${text}
        END
    END
