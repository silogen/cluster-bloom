*** Settings ***
Documentation     Test conditional field visibility in Bloom V2 Web UI
Library           Browser

*** Variables ***
${BASE_URL}       http://localhost:62080

*** Test Cases ***
Test First Node Checkbox Controls Additional Node Fields
    [Documentation]    When FIRST_NODE is unchecked, additional node fields should appear
    [Tags]    browser    conditional    first-node
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Check if FIRST_NODE checkbox exists
    ${first_node_exists}=    Get Element Count    input[name="FIRST_NODE"]
    IF    ${first_node_exists} > 0
        # Initially, FIRST_NODE should be checked (default)
        ${checked}=    Get Checkbox State    input[name="FIRST_NODE"]
        Should Be True    ${checked}

        # Additional node fields should be hidden
        ${server_ip_visible}=    Get Element Count    input[name="SERVER_IP"]:visible
        Should Be Equal As Integers    ${server_ip_visible}    0

        # Uncheck FIRST_NODE
        Uncheck Checkbox    input[name="FIRST_NODE"]
        Sleep    0.5s    # Wait for conditional logic to execute

        # Additional node fields should now be visible
        Wait For Elements State    input[name="SERVER_IP"]    visible    timeout=2s
        Wait For Elements State    input[name="JOIN_TOKEN"]    visible    timeout=2s

        # Check FIRST_NODE again
        Check Checkbox    input[name="FIRST_NODE"]
        Sleep    0.5s

        # Additional node fields should be hidden again
        ${server_ip_visible}=    Get Element Count    input[name="SERVER_IP"]:visible
        Should Be Equal As Integers    ${server_ip_visible}    0
    ELSE
        Log    FIRST_NODE checkbox not found - form may not be generated yet
    END

    Close Browser

Test Cert Manager Checkbox Controls Certificate Options
    [Documentation]    When USE_CERT_MANAGER is unchecked, CERT_OPTION should appear
    [Tags]    browser    conditional    cert-manager
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    ${cert_manager_exists}=    Get Element Count    input[name="USE_CERT_MANAGER"]
    IF    ${cert_manager_exists} > 0
        # Initially, USE_CERT_MANAGER should be unchecked (default)
        ${checked}=    Get Checkbox State    input[name="USE_CERT_MANAGER"]

        # If unchecked, CERT_OPTION should be visible
        IF    not ${checked}
            Wait For Elements State    select[name="CERT_OPTION"]    visible    timeout=2s
        END

        # Check USE_CERT_MANAGER
        Check Checkbox    input[name="USE_CERT_MANAGER"]
        Sleep    0.5s

        # CERT_OPTION should be hidden
        ${cert_option_visible}=    Get Element Count    select[name="CERT_OPTION"]:visible
        Should Be Equal As Integers    ${cert_option_visible}    0

        # Uncheck USE_CERT_MANAGER
        Uncheck Checkbox    input[name="USE_CERT_MANAGER"]
        Sleep    0.5s

        # CERT_OPTION should be visible again
        Wait For Elements State    select[name="CERT_OPTION"]    visible    timeout=2s
    ELSE
        Log    USE_CERT_MANAGER checkbox not found
    END

    Close Browser

Test Certificate Option Controls TLS Cert And Key Fields
    [Documentation]    When CERT_OPTION is "existing", TLS_CERT and TLS_KEY should appear
    [Tags]    browser    conditional    cert-option
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Ensure USE_CERT_MANAGER is unchecked so CERT_OPTION is visible
    ${cert_manager_exists}=    Get Element Count    input[name="USE_CERT_MANAGER"]
    IF    ${cert_manager_exists} > 0
        Uncheck Checkbox    input[name="USE_CERT_MANAGER"]
        Sleep    0.5s
    END

    ${cert_option_exists}=    Get Element Count    select[name="CERT_OPTION"]
    IF    ${cert_option_exists} > 0
        # TLS fields should be hidden initially
        ${tls_cert_visible}=    Get Element Count    input[name="TLS_CERT"]:visible
        Should Be Equal As Integers    ${tls_cert_visible}    0

        # Select "existing" option
        Select Options By    select[name="CERT_OPTION"]    value    existing
        Sleep    0.5s

        # TLS_CERT and TLS_KEY should now be visible
        Wait For Elements State    input[name="TLS_CERT"]    visible    timeout=2s
        Wait For Elements State    input[name="TLS_KEY"]    visible    timeout=2s

        # Select "generate" option
        Select Options By    select[name="CERT_OPTION"]    value    generate
        Sleep    0.5s

        # TLS fields should be hidden again
        ${tls_cert_visible}=    Get Element Count    input[name="TLS_CERT"]:visible
        Should Be Equal As Integers    ${tls_cert_visible}    0
        ${tls_key_visible}=    Get Element Count    input[name="TLS_KEY"]:visible
        Should Be Equal As Integers    ${tls_key_visible}    0
    ELSE
        Log    CERT_OPTION select not found
    END

    Close Browser

Test Multiple Conditional Fields Together
    [Documentation]    Test complex conditional logic: First node with existing certs
    [Tags]    browser    conditional    complex
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Scenario: First node (FIRST_NODE=true) with existing certificates
    # Certificate options should ONLY be visible for first node

    ${first_node_exists}=    Get Element Count    input[name="FIRST_NODE"]
    IF    ${first_node_exists} > 0
        # Step 1: Ensure FIRST_NODE is checked (default state)
        Check Checkbox    input[name="FIRST_NODE"]
        Sleep    0.5s

        # Additional node fields should be hidden
        ${server_ip_visible}=    Get Element Count    input[name="SERVER_IP"]:visible
        Should Be Equal As Integers    ${server_ip_visible}    0

        # Step 2: Configure certificates (only visible for first node)
        ${cert_manager_exists}=    Get Element Count    input[name="USE_CERT_MANAGER"]
        IF    ${cert_manager_exists} > 0
            # Uncheck cert-manager to show CERT_OPTION
            Uncheck Checkbox    input[name="USE_CERT_MANAGER"]
            Sleep    0.5s

            ${cert_option_exists}=    Get Element Count    select[name="CERT_OPTION"]
            IF    ${cert_option_exists} > 0
                # Select "existing" to show TLS cert/key fields
                Select Options By    select[name="CERT_OPTION"]    value    existing
                Sleep    0.5s

                # Verify TLS fields appear
                Wait For Elements State    input[name="TLS_CERT"]    visible    timeout=2s
                Wait For Elements State    input[name="TLS_KEY"]    visible    timeout=2s
            END
        END

        # Step 3: Switch to additional node (FIRST_NODE=false)
        Uncheck Checkbox    input[name="FIRST_NODE"]
        Sleep    0.5s

        # Additional node fields should now be visible
        Wait For Elements State    input[name="SERVER_IP"]    visible    timeout=2s
        Wait For Elements State    input[name="JOIN_TOKEN"]    visible    timeout=2s

        # Certificate fields should be HIDDEN for additional nodes
        ${cert_option_visible}=    Get Element Count    select[name="CERT_OPTION"]:visible
        Should Be Equal As Integers    ${cert_option_visible}    0
        ${tls_cert_visible}=    Get Element Count    input[name="TLS_CERT"]:visible
        Should Be Equal As Integers    ${tls_cert_visible}    0
    END

    Close Browser

Test Conditional Fields With Screenshot
    [Documentation]    Capture screenshots showing conditional field states
    [Tags]    browser    conditional    screenshot
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # State 1: First node (default)
    Take Screenshot    filename=conditional_first_node_checked    fullPage=True

    # State 2: Additional node
    ${first_node_exists}=    Get Element Count    input[name="FIRST_NODE"]
    IF    ${first_node_exists} > 0
        Uncheck Checkbox    input[name="FIRST_NODE"]
        Sleep    0.5s
        Take Screenshot    filename=conditional_first_node_unchecked    fullPage=True

        Check Checkbox    input[name="FIRST_NODE"]
        Sleep    0.5s
    END

    # State 3: Without cert-manager
    ${cert_manager_exists}=    Get Element Count    input[name="USE_CERT_MANAGER"]
    IF    ${cert_manager_exists} > 0
        Uncheck Checkbox    input[name="USE_CERT_MANAGER"]
        Sleep    0.5s
        Take Screenshot    filename=conditional_cert_manager_unchecked    fullPage=True

        # State 4: With existing certs
        ${cert_option_exists}=    Get Element Count    select[name="CERT_OPTION"]
        IF    ${cert_option_exists} > 0
            Select Options By    select[name="CERT_OPTION"]    value    existing
            Sleep    0.5s
            Take Screenshot    filename=conditional_cert_option_existing    fullPage=True
        END
    END

    Close Browser

*** Keywords ***
Wait For Form Field
    [Arguments]    ${selector}    ${state}=visible
    Wait For Elements State    ${selector}    ${state}    timeout=2s
