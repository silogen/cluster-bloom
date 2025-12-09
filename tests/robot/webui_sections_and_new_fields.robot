*** Settings ***
Documentation     Test new V2 arguments and section visibility in Bloom V2 Web UI
Library           Browser
Library           RequestsLibrary

*** Variables ***
${BASE_URL}       http://localhost:62080

*** Test Cases ***
Test ADDITIONAL_TLS_SAN_URLS Field Exists
    [Documentation]    Verify ADDITIONAL_TLS_SAN_URLS field appears for first node
    [Tags]    new-fields    ssl
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Ensure FIRST_NODE is checked
    ${first_node_checked}=    Get Checkbox State    input[name="FIRST_NODE"]
    IF    not ${first_node_checked}
        Check Checkbox    input[name="FIRST_NODE"]
        Sleep    0.5s
    END

    # Check if ADDITIONAL_TLS_SAN_URLS field exists
    ${field_exists}=    Get Element Count    input[name="ADDITIONAL_TLS_SAN_URLS"]
    Should Be True    ${field_exists} > 0    msg=ADDITIONAL_TLS_SAN_URLS field not found

    Close Browser

Test ROCM_DEB_PACKAGE Field Exists For GPU Node
    [Documentation]    Verify ROCM_DEB_PACKAGE appears when GPU_NODE is true
    [Tags]    new-fields    rocm
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Ensure GPU_NODE is checked
    ${gpu_node_checked}=    Get Checkbox State    input[name="GPU_NODE"]
    IF    not ${gpu_node_checked}
        Check Checkbox    input[name="GPU_NODE"]
        Sleep    0.5s
    END

    # Check if ROCM_DEB_PACKAGE field exists and is visible
    ${field_exists}=    Get Element Count    .form-group[data-key="ROCM_DEB_PACKAGE"]:not(.hidden)
    Should Be True    ${field_exists} > 0    msg=ROCM_DEB_PACKAGE field not found or hidden

    Close Browser

Test RKE2 Fields Exist
    [Documentation]    Verify RKE2_INSTALLATION_URL, RKE2_VERSION, and RKE2_EXTRA_CONFIG fields exist
    [Tags]    new-fields    rke2
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Check RKE2_INSTALLATION_URL
    ${rke2_install_exists}=    Get Element Count    input[name="RKE2_INSTALLATION_URL"]
    Should Be True    ${rke2_install_exists} > 0    msg=RKE2_INSTALLATION_URL field not found

    # Check RKE2_VERSION
    ${rke2_version_exists}=    Get Element Count    input[name="RKE2_VERSION"]
    Should Be True    ${rke2_version_exists} > 0    msg=RKE2_VERSION field not found

    # Check RKE2_EXTRA_CONFIG
    ${rke2_config_exists}=    Get Element Count    input[name="RKE2_EXTRA_CONFIG"]
    Should Be True    ${rke2_config_exists} > 0    msg=RKE2_EXTRA_CONFIG field not found

    Close Browser

Test New Fields Have Correct Defaults
    [Documentation]    Verify new fields have correct default values from V1
    [Tags]    new-fields    defaults
    Create Session    bloom    ${BASE_URL}

    # Get schema
    ${response}=    GET On Session    bloom    /api/schema
    ${schema}=    Set Variable    ${response.json()}

    # Get arguments array
    ${args}=    Set Variable    ${schema}[arguments]

    # Check ROCM_BASE_URL default
    ${rocm_base_arg}=    Evaluate    [arg for arg in ${args} if arg['key'] == 'ROCM_BASE_URL'][0]
    Should Be Equal    ${rocm_base_arg}[default]    https://repo.radeon.com/amdgpu-install/7.0.2/ubuntu/

    # Check ROCM_DEB_PACKAGE default
    ${rocm_deb_arg}=    Evaluate    [arg for arg in ${args} if arg['key'] == 'ROCM_DEB_PACKAGE'][0]
    Should Be Equal    ${rocm_deb_arg}[default]    amdgpu-install_7.0.2.70002-1_all.deb

    # Check RKE2_INSTALLATION_URL default
    ${rke2_url_arg}=    Evaluate    [arg for arg in ${args} if arg['key'] == 'RKE2_INSTALLATION_URL'][0]
    Should Be Equal    ${rke2_url_arg}[default]    https://get.rke2.io

    # Check RKE2_VERSION default
    ${rke2_ver_arg}=    Evaluate    [arg for arg in ${args} if arg['key'] == 'RKE2_VERSION'][0]
    Should Be Equal    ${rke2_ver_arg}[default]    v1.34.1+rke2r1

    # Check PRELOAD_IMAGES default
    ${preload_arg}=    Evaluate    [arg for arg in ${args} if arg['key'] == 'PRELOAD_IMAGES'][0]
    Should Contain    ${preload_arg}[default]    rocm/pytorch
    Should Contain    ${preload_arg}[default]    rocm/vllm

Test Section Hides When All Fields Hidden
    [Documentation]    Verify section headers hide when all fields underneath are hidden
    [Tags]    sections    visibility
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Initially, "Additional Node Configuration" section should be hidden
    # because FIRST_NODE defaults to true
    ${section_visible}=    Get Element Count    .config-section:has(.section-header:text("🔗 Additional Node Configuration")):not(.hidden)
    Should Be Equal As Integers    ${section_visible}    0    msg=Additional Node Configuration section should be hidden when FIRST_NODE is true

    # Uncheck FIRST_NODE to show Additional Node Configuration
    Uncheck Checkbox    input[name="FIRST_NODE"]
    Sleep    0.5s

    # Now section should be visible
    ${section_visible_after}=    Get Element Count    .config-section:has(.section-header:text("🔗 Additional Node Configuration")):not(.hidden)
    Should Be True    ${section_visible_after} > 0    msg=Additional Node Configuration section should be visible when FIRST_NODE is false

    Close Browser

Test Section Visibility For SSL TLS Configuration
    [Documentation]    Verify SSL/TLS section fields visibility based on dependencies
    [Tags]    sections    visibility    ssl
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # SSL/TLS Configuration section should be visible for first node
    ${section_visible}=    Get Element Count    .config-section:has(.section-header:text("🔒 SSL/TLS Configuration")):not(.hidden)
    Should Be True    ${section_visible} > 0    msg=SSL/TLS Configuration section should be visible for first node

    # Uncheck FIRST_NODE
    Uncheck Checkbox    input[name="FIRST_NODE"]
    Sleep    0.5s

    # SSL/TLS section should now be hidden (all SSL fields depend on FIRST_NODE=true)
    ${section_hidden}=    Get Element Count    .config-section:has(.section-header:text("🔒 SSL/TLS Configuration")):not(.hidden)
    Should Be Equal As Integers    ${section_hidden}    0    msg=SSL/TLS Configuration section should be hidden when FIRST_NODE is false

    Close Browser

Test Advanced Configuration Section Always Visible
    [Documentation]    Verify Advanced Configuration section has some always-visible fields
    [Tags]    sections    visibility    advanced
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Advanced Configuration should be visible initially
    ${section_visible}=    Get Element Count    .config-section:has(.section-header:text("⚙️ Advanced Configuration")):not(.hidden)
    Should Be True    ${section_visible} > 0    msg=Advanced Configuration section should be visible

    # Even when FIRST_NODE is unchecked, Advanced section should remain visible
    # (because it has fields without dependencies like CLUSTERFORGE_RELEASE, CF_VALUES, etc.)
    Uncheck Checkbox    input[name="FIRST_NODE"]
    Sleep    0.5s

    ${section_still_visible}=    Get Element Count    .config-section:has(.section-header:text("⚙️ Advanced Configuration")):not(.hidden)
    Should Be True    ${section_still_visible} > 0    msg=Advanced Configuration section should still be visible

    Close Browser

Test Storage Configuration Section Visibility
    [Documentation]    Verify Storage Configuration section visibility based on NO_DISKS_FOR_CLUSTER
    [Tags]    sections    visibility    storage
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Storage Configuration section should be visible initially
    ${section_visible}=    Get Element Count    .config-section:has(.section-header:text("💾 Storage Configuration")):not(.hidden)
    Should Be True    ${section_visible} > 0    msg=Storage Configuration section should be visible

    # Check NO_DISKS_FOR_CLUSTER to hide disk-related fields
    Check Checkbox    input[name="NO_DISKS_FOR_CLUSTER"]
    Sleep    0.5s

    # Section should still be visible because NO_DISKS_FOR_CLUSTER itself is visible
    ${section_after}=    Get Element Count    .config-section:has(.section-header:text("💾 Storage Configuration")):not(.hidden)
    Should Be True    ${section_after} > 0    msg=Storage Configuration section should still be visible

    Close Browser

Test All Six Sections Exist
    [Documentation]    Verify all six section headers exist in the UI
    [Tags]    sections    structure
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Check all section headers exist
    ${basic_section}=    Get Element Count    .section-header:text("📋 Basic Configuration")
    Should Be Equal As Integers    ${basic_section}    1    msg=Basic Configuration section header not found

    ${additional_section}=    Get Element Count    .section-header:text("🔗 Additional Node Configuration")
    Should Be Equal As Integers    ${additional_section}    1    msg=Additional Node Configuration section header not found

    ${storage_section}=    Get Element Count    .section-header:text("💾 Storage Configuration")
    Should Be Equal As Integers    ${storage_section}    1    msg=Storage Configuration section header not found

    ${ssl_section}=    Get Element Count    .section-header:text("🔒 SSL/TLS Configuration")
    Should Be Equal As Integers    ${ssl_section}    1    msg=SSL/TLS Configuration section header not found

    ${advanced_section}=    Get Element Count    .section-header:text("⚙️ Advanced Configuration")
    Should Be Equal As Integers    ${advanced_section}    1    msg=Advanced Configuration section header not found

    ${cli_section}=    Get Element Count    .section-header:text("💻 Command Line Options")
    Should Be Equal As Integers    ${cli_section}    1    msg=Command Line Options section header not found

    Close Browser

Test Section Headers Have Green Background
    [Documentation]    Verify section headers have correct styling (green background)
    [Tags]    sections    styling
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Check section header styling
    ${bg_color}=    Get Style    .section-header >> nth=0    background-color
    Should Contain    ${bg_color}    76, 175, 80    msg=Section header should have green background (#4CAF50 = rgb(76, 175, 80))

    ${text_color}=    Get Style    .section-header >> nth=0    color
    Should Contain Any    ${text_color}    255, 255, 255    255,255,255    msg=Section header text should be white

    Close Browser
