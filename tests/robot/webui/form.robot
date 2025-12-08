*** Settings ***
Documentation     Tests for Web UI form interactions
Resource          ../resources/common.robot
Library           Browser

Suite Setup       Setup Suite
Suite Teardown    Teardown Suite
Test Setup        Navigate To Web UI
Test Teardown     Close Browser

*** Variables ***
${BROWSER}        chromium
${HEADLESS}       ${True}

*** Test Cases ***
Form Renders All Fields From Schema
    [Documentation]    Verify form renders all fields from schema
    Wait For Elements State    css=.form-group    visible    timeout=5s

    # Check for key V1 fields
    Get Element    id=FIRST_NODE
    Get Element    id=DOMAIN
    Get Element    id=GPU_NODE
    Get Element    id=CLUSTER_DISKS

First Node Toggle Shows Domain Field
    [Documentation]    Verify FIRST_NODE=true shows DOMAIN field
    Click    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    visible    timeout=2s

    # Uncheck and verify DOMAIN is hidden
    Click    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    hidden    timeout=2s

Additional Node Shows Server IP Field
    [Documentation]    Verify FIRST_NODE=false shows SERVER_IP field
    # FIRST_NODE should be unchecked by default
    Wait For Elements State    id=SERVER_IP    visible    timeout=2s

    # Check FIRST_NODE and verify SERVER_IP is hidden
    Click    id=FIRST_NODE
    Wait For Elements State    id=SERVER_IP    hidden    timeout=2s

GPU Node Shows ROCm URL Field
    [Documentation]    Verify GPU_NODE=true shows ROCM_BASE_URL field
    Click    id=GPU_NODE
    Wait For Elements State    id=ROCM_BASE_URL    visible    timeout=2s

    # Uncheck and verify field is hidden
    Click    id=GPU_NODE
    Wait For Elements State    id=ROCM_BASE_URL    hidden    timeout=2s

Validation Shows Errors For Invalid Config
    [Documentation]    Verify validation button shows errors
    # Try to validate without required fields
    Click    css=button[type="button"]:has-text("Validate")

    Wait For Elements State    css=.error    visible    timeout=2s
    ${error_text}=    Get Text    css=.error
    Should Contain    ${error_text}    required

Valid First Node Config Validates Successfully
    [Documentation]    Verify valid first node config passes validation
    Click    id=FIRST_NODE
    Fill Text    id=DOMAIN    test.example.com
    Click    css=select#CERT_OPTION
    Select Options By    css=select#CERT_OPTION    value    generate

    Click    css=button[type="button"]:has-text("Validate")
    Wait For Elements State    css=.success    visible    timeout=2s

Generate Button Creates YAML Preview
    [Documentation]    Verify generate button creates YAML
    Click    id=FIRST_NODE
    Fill Text    id=DOMAIN    cluster.local
    Click    css=select#CERT_OPTION
    Select Options By    css=select#CERT_OPTION    value    generate

    Click    css=button[type="submit"]
    Wait For Elements State    id=yamlPreview    visible    timeout=2s

    ${yaml}=    Get Text    id=yamlPreview
    Should Contain    ${yaml}    FIRST_NODE: true
    Should Contain    ${yaml}    DOMAIN: cluster.local

Download Button Appears After Generation
    [Documentation]    Verify download button appears after YAML generation
    Click    id=FIRST_NODE
    Fill Text    id=DOMAIN    test.local
    Click    css=select#CERT_OPTION
    Select Options By    css=select#CERT_OPTION    value    generate

    Click    css=button[type="submit"]
    Wait For Elements State    id=downloadBtn    visible    timeout=2s

Boolean Fields Format Correctly In YAML
    [Documentation]    Verify boolean fields show as lowercase true/false
    Click    id=FIRST_NODE
    Fill Text    id=DOMAIN    test.local
    Click    css=select#CERT_OPTION
    Select Options By    css=select#CERT_OPTION    value    generate
    Click    id=GPU_NODE
    Click    id=NO_DISKS_FOR_CLUSTER

    Click    css=button[type="submit"]
    Wait For Elements State    id=yamlPreview    visible    timeout=2s

    ${yaml}=    Get Text    id=yamlPreview
    Should Contain    ${yaml}    FIRST_NODE: true
    Should Contain    ${yaml}    GPU_NODE: true
    Should Contain    ${yaml}    NO_DISKS_FOR_CLUSTER: true

Cert Manager Toggle Shows Cert Option Field
    [Documentation]    Verify USE_CERT_MANAGER=false shows CERT_OPTION
    Click    id=FIRST_NODE

    # Check USE_CERT_MANAGER (should hide CERT_OPTION)
    Click    id=USE_CERT_MANAGER
    Wait For Elements State    css=select#CERT_OPTION    hidden    timeout=2s

    # Uncheck (should show CERT_OPTION)
    Click    id=USE_CERT_MANAGER
    Wait For Elements State    css=select#CERT_OPTION    visible    timeout=2s

*** Keywords ***
Setup Suite
    Start Bloom Web UI
    Sleep    3s    Wait for server to fully start

Teardown Suite
    Stop Bloom Web UI

Navigate To Web UI
    New Browser    browser=${BROWSER}    headless=${HEADLESS}
    New Page    ${BASE_URL}
    Wait For Load State    networkidle
