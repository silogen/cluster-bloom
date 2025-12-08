*** Settings ***
Documentation     Tests for /api/generate endpoint
Resource          ../resources/common.robot
Suite Setup       Setup Suite
Suite Teardown    Teardown Suite

*** Test Cases ***
Generate Creates Valid YAML
    [Documentation]    Verify generate endpoint creates valid YAML
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    DOMAIN=cluster.example.com
    ...    GPU_NODE=${False}
    ...    USE_CERT_MANAGER=${False}
    ...    CERT_OPTION=generate
    ...    CLUSTER_DISKS=/dev/nvme0n1,/dev/nvme1n1
    ...    NO_DISKS_FOR_CLUSTER=${False}
    ...    CLUSTERFORGE_RELEASE=none

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/generate    json=${body}    expected_status=200

    Should Not Be Empty    ${response.json()}[yaml]
    ${yaml}=    Set Variable    ${response.json()}[yaml]

    # Verify YAML contains expected fields
    Should Contain    ${yaml}    FIRST_NODE: true
    Should Contain    ${yaml}    DOMAIN: cluster.example.com
    Should Contain    ${yaml}    GPU_NODE: false

Generate Rejects Invalid Config
    [Documentation]    Verify generate rejects invalid configuration
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    GPU_NODE=${False}
    # Missing required DOMAIN

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/generate    json=${body}    expected_status=400

    Should Be Equal    ${response.json()}[valid]    ${False}
    Should Not Be Empty    ${response.json()}[errors]

Generate Includes All Provided Fields
    [Documentation]    Verify all provided config fields appear in YAML
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    DOMAIN=test.local
    ...    GPU_NODE=${True}
    ...    ROCM_BASE_URL=https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/
    ...    USE_CERT_MANAGER=${False}
    ...    CERT_OPTION=generate
    ...    CLUSTER_DISKS=/dev/sda,/dev/sdb
    ...    CLUSTERFORGE_RELEASE=none
    ...    PRELOAD_IMAGES=

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/generate    json=${body}    expected_status=200

    ${yaml}=    Set Variable    ${response.json()}[yaml]

    # Check all fields present
    Should Contain    ${yaml}    FIRST_NODE:
    Should Contain    ${yaml}    DOMAIN:
    Should Contain    ${yaml}    GPU_NODE:
    Should Contain    ${yaml}    ROCM_BASE_URL:
    Should Contain    ${yaml}    CLUSTER_DISKS:

Generate Handles Boolean Values Correctly
    [Documentation]    Verify boolean values are formatted correctly in YAML
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    DOMAIN=test.local
    ...    GPU_NODE=${False}
    ...    USE_CERT_MANAGER=${False}
    ...    CERT_OPTION=generate
    ...    NO_DISKS_FOR_CLUSTER=${True}

    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    webui    /api/generate    json=${body}    expected_status=200

    ${yaml}=    Set Variable    ${response.json()}[yaml]

    Should Contain    ${yaml}    FIRST_NODE: true
    Should Contain    ${yaml}    GPU_NODE: false
    Should Contain    ${yaml}    NO_DISKS_FOR_CLUSTER: true

*** Keywords ***
Setup Suite
    Start Bloom Web UI
    Create Session To Web UI
    Verify Server Is Running

Teardown Suite
    Stop Bloom Web UI
