*** Settings ***
Documentation     API endpoint tests for Bloom V2 Web UI
Library           RequestsLibrary
Library           Collections
Resource          keywords.resource

*** Test Cases ***
Test Schema Endpoint Returns Valid JSON
    [Documentation]    Verify /api/schema returns configuration schema
    Create Session    bloom    ${BASE_URL}
    ${response}=    GET On Session    bloom    /api/schema
    Should Be Equal As Integers    ${response.status_code}    200
    ${schema}=    Set Variable    ${response.json()}
    Should Not Be Empty    ${schema}[arguments]
    Length Should Be    ${schema}[arguments]    27

Test Generate Endpoint Creates YAML
    [Documentation]    Verify /api/generate creates YAML output
    Create Session    bloom    ${BASE_URL}
    ${config}=    Create Dictionary
    ...    FIRST_NODE=true
    ...    GPU_NODE=true
    ...    DOMAIN=cluster.example.com
    ...    NO_DISKS_FOR_CLUSTER=true
    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/generate    json=${body}
    Should Be Equal As Integers    ${response.status_code}    200
    ${result}=    Set Variable    ${response.json()}
    Should Contain    ${result}[yaml]    DOMAIN: cluster.example.com
