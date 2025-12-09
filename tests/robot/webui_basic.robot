*** Settings ***
Documentation     Basic Web UI API Tests for Bloom V2
Library           RequestsLibrary
Library           Collections

*** Variables ***
${BASE_URL}       http://localhost:8080

*** Test Cases ***
Test Schema Endpoint Returns Valid JSON
    [Documentation]    Verify /api/schema returns configuration schema
    [Tags]    api    schema
    Create Session    bloom    ${BASE_URL}
    ${response}=    GET On Session    bloom    /api/schema
    Should Be Equal As Integers    ${response.status_code}    200
    ${json}=    Set Variable    ${response.json()}
    Dictionary Should Contain Key    ${json}    arguments
    ${args}=    Get From Dictionary    ${json}    arguments
    Should Not Be Empty    ${args}

Test Validate Endpoint Accepts Valid Config
    [Documentation]    Verify /api/validate accepts valid configuration
    [Tags]    api    validate
    Create Session    bloom    ${BASE_URL}
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    GPU_NODE=${False}
    ...    DOMAIN=test.example.com
    ...    CERT_OPTION=generate
    ...    USE_CERT_MANAGER=${False}
    ${payload}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/validate    json=${payload}
    Should Be Equal As Integers    ${response.status_code}    200
    ${json}=    Set Variable    ${response.json()}
    Dictionary Should Contain Key    ${json}    valid
    Should Be True    ${json}[valid]

Test Validate Endpoint Rejects Invalid Config
    [Documentation]    Verify /api/validate rejects invalid configuration (missing domain)
    [Tags]    api    validate    negative
    Create Session    bloom    ${BASE_URL}
    ${config}=    Create Dictionary    FIRST_NODE=${True}
    ${payload}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/validate    json=${payload}
    Should Be Equal As Integers    ${response.status_code}    200
    ${json}=    Set Variable    ${response.json()}
    Dictionary Should Contain Key    ${json}    valid
    Should Not Be True    ${json}[valid]
    Dictionary Should Contain Key    ${json}    errors
    ${errors}=    Get From Dictionary    ${json}    errors
    Should Not Be Empty    ${errors}

Test Generate Endpoint Creates YAML
    [Documentation]    Verify /api/generate creates valid bloom.yaml
    [Tags]    api    generate
    Create Session    bloom    ${BASE_URL}
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    GPU_NODE=${False}
    ...    DOMAIN=cluster.example.com
    ...    CERT_OPTION=generate
    ...    USE_CERT_MANAGER=${False}
    ...    NO_DISKS_FOR_CLUSTER=${True}
    ${payload}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/generate    json=${payload}
    Should Be Equal As Integers    ${response.status_code}    200
    ${json}=    Set Variable    ${response.json()}
    Dictionary Should Contain Key    ${json}    yaml
    ${yaml}=    Get From Dictionary    ${json}    yaml
    Should Contain    ${yaml}    FIRST_NODE: true
    Should Contain    ${yaml}    DOMAIN: cluster.example.com

Test Static Files Are Served
    [Documentation]    Verify static HTML/CSS/JS files are accessible
    [Tags]    static
    Create Session    bloom    ${BASE_URL}
    ${response}=    GET On Session    bloom    /
    Should Be Equal As Integers    ${response.status_code}    200
    Should Contain    ${response.text}    Bloom Configuration Generator

    ${response}=    GET On Session    bloom    /css/styles.css
    Should Be Equal As Integers    ${response.status_code}    200
    Should Contain    ${response.text}    .container

    ${response}=    GET On Session    bloom    /js/app.js
    Should Be Equal As Integers    ${response.status_code}    200
