*** Settings ***
Documentation     Config generation tests for Bloom V2 Web UI
Library           Browser
Library           RequestsLibrary
Library           Collections
Library           OperatingSystem
Resource          keywords.resource

*** Test Cases ***
Test Generate Valid First Node Config
    [Documentation]    Generate a valid bloom.yaml for first node deployment
    Setup Minimal Valid First Node Config
    Fill Text    id=CLUSTER_PREMOUNTED_DISKS    /mnt/disk1,/mnt/disk2
    Submit And Wait For Preview
    ${yamlContent}=    Get Text    id=yaml-preview
    Should Contain    ${yamlContent}    FIRST_NODE: true
    Should Contain    ${yamlContent}    DOMAIN: cluster.example.com
    Should Contain    ${yamlContent}    GPU_NODE: true
    Should Contain    ${yamlContent}    CLUSTER_PREMOUNTED_DISKS: "/mnt/disk1,/mnt/disk2"

Test Generate Valid Additional Node Config
    [Documentation]    Generate a valid bloom.yaml for additional node
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s
    Uncheck Checkbox    id=FIRST_NODE
    Wait For Elements State    id=SERVER_IP    visible    timeout=2s
    Fill Text    id=SERVER_IP    10.100.100.11
    Fill Text    id=JOIN_TOKEN    K1234567890abcdef::server:1234567890abcdef
    Check Checkbox    id=CONTROL_PLANE
    Check Checkbox    id=GPU_NODE
    Fill Text    id=CLUSTER_DISKS    /dev/nvme0n1,/dev/nvme1n1
    Submit And Wait For Preview
    ${yamlContent}=    Get Text    css=pre
    Should Contain    ${yamlContent}    FIRST_NODE: false
    Should Contain    ${yamlContent}    SERVER_IP: 10.100.100.11
    Should Contain    ${yamlContent}    JOIN_TOKEN:
    Should Contain    ${yamlContent}    CONTROL_PLANE: true

Test Generate Config With TLS Certificates
    [Documentation]    Generate config with existing TLS certificates
    Setup Minimal Valid First Node Config
    Select Options By    id=CERT_OPTION    value    existing
    Wait For Elements State    id=TLS_CERT    visible    timeout=2s
    Fill Text    id=TLS_CERT    /etc/ssl/certs/cluster.crt
    Fill Text    id=TLS_KEY    /etc/ssl/private/cluster.key
    Fill Text    id=CLUSTER_DISKS    /dev/sda
    Submit And Wait For Preview
    ${yamlContent}=    Get Text    css=pre
    Should Contain    ${yamlContent}    TLS_CERT: /etc/ssl/certs/cluster.crt
    Should Contain    ${yamlContent}    TLS_KEY: /etc/ssl/private/cluster.key
    Should Contain    ${yamlContent}    CERT_OPTION: existing

Test Generate Config With Advanced Options
    [Documentation]    Generate config with advanced ROCm and RKE2 settings
    Setup Minimal Valid First Node Config
    Check Checkbox    id=GPU_NODE
    Wait For Elements State    id=ROCM_BASE_URL    visible    timeout=2s
    Fill Text    id=ROCM_BASE_URL    https://custom.repo.com/rocm/
    Fill Text    id=ROCM_DEB_PACKAGE    custom-rocm-7.0.deb
    Fill Text    id=RKE2_VERSION    v1.33.0+rke2r1
    Fill Text    id=RKE2_INSTALLATION_URL    https://custom.rke2.install
    Fill Text    id=CLUSTER_PREMOUNTED_DISKS    /mnt/disk1
    Submit And Wait For Preview
    ${yamlContent}=    Get Text    css=pre
    Should Contain    ${yamlContent}    ROCM_BASE_URL: "https://custom.repo.com/rocm/"
    Should Contain    ${yamlContent}    ROCM_DEB_PACKAGE: custom-rocm-7.0.deb
    Should Contain    ${yamlContent}    RKE2_VERSION: v1.33.0+rke2r1

Test API Generate Endpoint
    [Documentation]    Test /api/generate endpoint directly
    Create Session    bloom    ${BASE_URL}
    ${config}=    Create Dictionary
    ...    FIRST_NODE=true
    ...    GPU_NODE=true
    ...    DOMAIN=api-test.example.com
    ...    CLUSTER_PREMOUNTED_DISKS=/mnt/disk1
    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/generate    json=${body}
    Status Should Be    200    ${response}
    ${result}=    Set Variable    ${response.json()}
    Should Contain    ${result}[yaml]    FIRST_NODE: true
    Should Contain    ${result}[yaml]    DOMAIN: api-test.example.com
    Should Contain    ${result}[yaml]    GPU_NODE: true

Test API Generate With Invalid Config
    [Documentation]    Test /api/generate rejects invalid config
    Create Session    bloom    ${BASE_URL}
    ${config}=    Create Dictionary
    ...    FIRST_NODE=true
    ...    GPU_NODE=true
    ...    CLUSTER_PREMOUNTED_DISKS=/mnt/disk1
    ${body}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/generate    json=${body}    expected_status=400
    Status Should Be    400    ${response}

Test Generate Config Minimal Output
    [Documentation]    Verify minimal YAML output (only non-defaults + FIRST_NODE/GPU_NODE)
    Setup Minimal Valid First Node Config
    Fill Text    id=CLUSTER_PREMOUNTED_DISKS    /mnt/disk1
    Submit And Wait For Preview
    ${yamlContent}=    Get Text    css=pre
    Should Contain    ${yamlContent}    FIRST_NODE: true
    Should Contain    ${yamlContent}    GPU_NODE: true
    Should Contain    ${yamlContent}    DOMAIN: cluster.example.com
    Should Contain    ${yamlContent}    CERT_OPTION: generate
    Should Not Contain    ${yamlContent}    SKIP_RANCHER_PARTITION_CHECK
    Should Not Contain    ${yamlContent}    NO_DISKS_FOR_CLUSTER: false

Test Field Visibility Updates Config
    [Documentation]    Verify conditional fields appear/disappear in generated config
    Setup Minimal Valid First Node Config
    Fill Text    id=CLUSTER_PREMOUNTED_DISKS    /mnt/disk1
    Click    id=GPU_NODE
    Submit And Wait For Preview
    ${yamlContent}=    Get Text    id=yaml-preview
    Should Contain    ${yamlContent}    DOMAIN: cluster.example.com
    Should Not Contain    ${yamlContent}    SERVER_IP

    Click    id=edit-btn
    Sleep    1s
    Uncheck Checkbox    id=FIRST_NODE
    Wait For Elements State    id=SERVER_IP    visible    timeout=2s
    Fill Text    id=SERVER_IP    10.100.100.11
    Fill Text    id=JOIN_TOKEN    testtoken::server:abc123
    Fill Text    id=CLUSTER_PREMOUNTED_DISKS    ${EMPTY}
    Fill Text    id=CLUSTER_DISKS    /dev/sda
    Submit And Wait For Preview
    ${yamlContent}=    Get Text    id=yaml-preview
    Should Not Contain    ${yamlContent}    DOMAIN
    Should Contain    ${yamlContent}    SERVER_IP: 10.100.100.11
    Should Contain    ${yamlContent}    JOIN_TOKEN:
