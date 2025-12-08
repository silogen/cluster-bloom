*** Settings ***
Documentation     Tests for /api/validate endpoint
Resource          ../resources/common.robot
Suite Setup       Setup Suite
Suite Teardown    Teardown Suite

*** Test Cases ***
Validate Accepts Valid First Node Config
    [Documentation]    Verify validation accepts valid first node configuration
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    DOMAIN=test.example.com
    ...    GPU_NODE=${False}
    ...    USE_CERT_MANAGER=${False}
    ...    CERT_OPTION=generate
    ...    NO_DISKS_FOR_CLUSTER=${False}

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/validate    json=${body}    expected_status=200

    Should Be Equal    ${response.json()}[valid]    ${True}
    Should Be Empty    ${response.json()}[errors]

Validate Rejects Missing Required Fields
    [Documentation]    Verify validation rejects config missing required fields
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    GPU_NODE=${False}
    # Missing DOMAIN which is required when FIRST_NODE=true

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/validate    json=${body}    expected_status=200

    Should Be Equal    ${response.json()}[valid]    ${False}
    Should Not Be Empty    ${response.json()}[errors]

Validate Rejects Invalid Enum Values
    [Documentation]    Verify validation rejects invalid enum values
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    DOMAIN=test.example.com
    ...    GPU_NODE=${False}
    ...    USE_CERT_MANAGER=${False}
    ...    CERT_OPTION=invalid_option    # Should be 'existing' or 'generate'

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/validate    json=${body}    expected_status=200

    Should Be Equal    ${response.json()}[valid]    ${False}
    Should Not Be Empty    ${response.json()}[errors]

Validate Accepts Additional Node Config
    [Documentation]    Verify validation accepts valid additional node config
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${False}
    ...    SERVER_IP=10.0.0.1
    ...    JOIN_TOKEN=K10abc123::server:xyz
    ...    GPU_NODE=${False}
    ...    NO_DISKS_FOR_CLUSTER=${False}

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/validate    json=${body}    expected_status=200

    Should Be Equal    ${response.json()}[valid]    ${True}

Validate Requires Server IP For Additional Nodes
    [Documentation]    Verify SERVER_IP is required when FIRST_NODE=false
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${False}
    ...    JOIN_TOKEN=K10abc123::server:xyz
    ...    GPU_NODE=${False}
    # Missing SERVER_IP

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/validate    json=${body}    expected_status=200

    Should Be Equal    ${response.json()}[valid]    ${False}
    Should Not Be Empty    ${response.json()}[errors]

*** Keywords ***
Setup Suite
    Start Bloom Web UI
    Create Session To Web UI
    Verify Server Is Running

Teardown Suite
    Stop Bloom Web UI
