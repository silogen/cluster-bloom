*** Settings ***
Documentation     Tests for /api/schema endpoint
Resource          ../resources/common.robot
Suite Setup       Setup Suite
Suite Teardown    Teardown Suite

*** Test Cases ***
Schema Endpoint Returns 200 OK
    [Documentation]    Verify schema endpoint returns success
    ${response}=    GET On Session    webui    /api/schema    expected_status=200
    Should Be Equal As Strings    ${response.status_code}    200

Schema Returns Valid JSON
    [Documentation]    Verify schema returns valid JSON structure
    ${response}=    GET On Session    webui    /api/schema
    Should Not Be Empty    ${response.json()}
    Dictionary Should Contain Key    ${response.json()}    arguments

Schema Contains Expected Fields
    [Documentation]    Verify schema contains all required V1 fields
    ${response}=    GET On Session    webui    /api/schema
    ${arguments}=    Set Variable    ${response.json()}[arguments]

    # Should be a list of argument definitions
    Should Not Be Empty    ${arguments}

    # Check for key V1 fields
    ${keys}=    Create List
    FOR    ${arg}    IN    @{arguments}
        Append To List    ${keys}    ${arg}[key]
    END

    List Should Contain Value    ${keys}    FIRST_NODE
    List Should Contain Value    ${keys}    DOMAIN
    List Should Contain Value    ${keys}    GPU_NODE
    List Should Contain Value    ${keys}    CLUSTER_DISKS

Schema Arguments Have Required Properties
    [Documentation]    Verify each argument has required properties
    ${response}=    GET On Session    webui    /api/schema
    ${arguments}=    Set Variable    ${response.json()}[arguments]

    FOR    ${arg}    IN    @{arguments}
        Dictionary Should Contain Key    ${arg}    key
        Dictionary Should Contain Key    ${arg}    type
        Dictionary Should Contain Key    ${arg}    description
    END

*** Keywords ***
Setup Suite
    Start Bloom Web UI
    Create Session To Web UI
    Verify Server Is Running

Teardown Suite
    Stop Bloom Web UI
