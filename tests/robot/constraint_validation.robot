*** Settings ***
Documentation     Test constraint validation in the web UI
Library           Browser

*** Variables ***
${BASE_URL}       http://localhost:62080

*** Test Cases ***
Test Mutually Exclusive Fields Constraint
    [Documentation]    Verify DISABLED_STEPS and ENABLED_STEPS are mutually exclusive
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Fill in required fields first
    Check Checkbox    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    visible    timeout=2s
    Fill Text    id=DOMAIN    cluster.example.com
    Select Options By    id=CERT_OPTION    value    generate

    # Set both DISABLED_STEPS and ENABLED_STEPS (mutually exclusive)
    Fill Text    id=DISABLED_STEPS    step1,step2
    Fill Text    id=ENABLED_STEPS    step3,step4

    # Submit form
    Click    button[type="submit"]
    Sleep    1s

    # Should show error (either in preview or error message)
    ${error_visible}=    Get Element States    id=error    *=    visible
    ${has_error}=    Evaluate    "visible" in """${error_visible}"""
    Should Be True    ${has_error}    Expected validation error for mutually exclusive fields

Test Storage One-Of Constraint - No Storage Configured
    [Documentation]    Verify storage one-of constraint: must configure exactly one storage option
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Fill in required fields
    Check Checkbox    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    visible    timeout=2s
    Fill Text    id=DOMAIN    cluster.example.com
    Select Options By    id=CERT_OPTION    value    generate

    # Configure no storage (all three options false/empty)
    Uncheck Checkbox    id=NO_DISKS_FOR_CLUSTER
    Fill Text    id=CLUSTER_DISKS    ${EMPTY}
    Fill Text    id=CLUSTER_PREMOUNTED_DISKS    ${EMPTY}

    # Submit form
    Click    button[type="submit"]
    Sleep    1s

    # Should show error
    ${error_visible}=    Get Element States    id=error    *=    visible
    ${has_error}=    Evaluate    "visible" in """${error_visible}"""
    Should Be True    ${has_error}    Expected validation error for no storage configured

Test Storage One-Of Constraint - NO_DISKS Only
    [Documentation]    Verify NO_DISKS_FOR_CLUSTER=true with empty disks is valid
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Fill in required fields
    Check Checkbox    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    visible    timeout=2s
    Fill Text    id=DOMAIN    cluster.example.com
    Select Options By    id=CERT_OPTION    value    generate

    # Configure NO_DISKS_FOR_CLUSTER only
    Check Checkbox    id=NO_DISKS_FOR_CLUSTER
    Fill Text    id=CLUSTER_DISKS    ${EMPTY}
    Fill Text    id=CLUSTER_PREMOUNTED_DISKS    ${EMPTY}

    # Submit form
    Click    button[type="submit"]
    Wait For Elements State    id=preview    visible    timeout=5s

    # Should succeed and show preview
    ${yaml}=    Get Text    id=yaml-preview
    Should Contain    ${yaml}    NO_DISKS_FOR_CLUSTER: true

Test Storage One-Of Constraint - Multiple Options Set
    [Documentation]    Verify error when both CLUSTER_DISKS and CLUSTER_PREMOUNTED_DISKS are set
    New Page    ${BASE_URL}
    Wait For Elements State    id=FIRST_NODE    visible    timeout=10s

    # Fill in required fields
    Check Checkbox    id=FIRST_NODE
    Wait For Elements State    id=DOMAIN    visible    timeout=2s
    Fill Text    id=DOMAIN    cluster.example.com
    Select Options By    id=CERT_OPTION    value    generate

    # Set multiple storage options (violates one-of)
    Uncheck Checkbox    id=NO_DISKS_FOR_CLUSTER
    Fill Text    id=CLUSTER_DISKS    /dev/sda
    Fill Text    id=CLUSTER_PREMOUNTED_DISKS    /mnt/disk1

    # Submit form
    Click    button[type="submit"]
    Sleep    1s

    # Should show error
    ${error_visible}=    Get Element States    id=error    *=    visible
    ${has_error}=    Evaluate    "visible" in """${error_visible}"""
    Should Be True    ${has_error}    Expected validation error for multiple storage options
