*** Settings ***
Documentation     Test field format validation in Bloom V2 Web UI
Library           Browser
Library           RequestsLibrary

*** Variables ***
${BASE_URL}       http://localhost:8080

*** Test Cases ***
Test Domain Field Format Validation
    [Documentation]    Verify DOMAIN field accepts valid domains and rejects invalid formats
    [Tags]    validation    format    domain
    Create Session    bloom    ${BASE_URL}

    # Valid domain formats
    ${valid_domains}=    Create List
    ...    cluster.example.com
    ...    my-cluster.domain.io
    ...    test123.local
    ...    sub.domain.example.org

    FOR    ${domain}    IN    @{valid_domains}
        ${config}=    Create Dictionary
        ...    FIRST_NODE=${True}
        ...    GPU_NODE=${False}
        ...    DOMAIN=${domain}
        ...    USE_CERT_MANAGER=${False}
        ...    CERT_OPTION=generate
        ...    NO_DISKS_FOR_CLUSTER=${True}
        ${payload}=    Create Dictionary    config=${config}
        ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
        ${json}=    Set Variable    ${response.json()}
        Should Be True    ${json}[valid]    msg=Valid domain "${domain}" was rejected
    END

    # Invalid domain formats
    ${invalid_domains}=    Create List
    ...    invalid domain with spaces
    ...    -starts-with-dash.com
    ...    ends-with-dash-.com
    ...    has..double..dots.com
    ...    .starts-with-dot.com
    ...    ends-with-dot.com.
    ...    has_underscore.com

    FOR    ${domain}    IN    @{invalid_domains}
        ${config}=    Create Dictionary
        ...    FIRST_NODE=${True}
        ...    GPU_NODE=${False}
        ...    DOMAIN=${domain}
        ...    USE_CERT_MANAGER=${False}
        ...    CERT_OPTION=generate
        ...    NO_DISKS_FOR_CLUSTER=${True}
        ${payload}=    Create Dictionary    config=${config}
        ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
        ${json}=    Set Variable    ${response.json()}
        Should Not Be True    ${json}[valid]    msg=Invalid domain "${domain}" was accepted
    END

Test IP Address Field Format Validation
    [Documentation]    Verify SERVER_IP field accepts valid IPs and rejects invalid formats
    [Tags]    validation    format    ip-address
    Create Session    bloom    ${BASE_URL}

    # Valid IP addresses
    ${valid_ips}=    Create List
    ...    192.168.1.1
    ...    10.0.0.1
    ...    172.16.0.1
    ...    1.1.1.1
    ...    255.255.255.255

    FOR    ${ip}    IN    @{valid_ips}
        ${config}=    Create Dictionary
        ...    FIRST_NODE=${False}
        ...    GPU_NODE=${False}
        ...    SERVER_IP=${ip}
        ...    JOIN_TOKEN=test-token-123
        ${payload}=    Create Dictionary    config=${config}
        ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
        ${json}=    Set Variable    ${response.json()}
        Should Be True    ${json}[valid]    msg=Valid IP "${ip}" was rejected
    END

    # Invalid IP addresses
    ${invalid_ips}=    Create List
    ...    256.1.1.1
    ...    192.168.1.999
    ...    192.168
    ...    192.168.1
    ...    192.168.1.1.1
    ...    abc.def.ghi.jkl
    ...    192.168.-1.1

    FOR    ${ip}    IN    @{invalid_ips}
        ${config}=    Create Dictionary
        ...    FIRST_NODE=${False}
        ...    GPU_NODE=${False}
        ...    SERVER_IP=${ip}
        ...    JOIN_TOKEN=test-token-123
        ${payload}=    Create Dictionary    config=${config}
        ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
        ${json}=    Set Variable    ${response.json()}
        Should Not Be True    ${json}[valid]    msg=Invalid IP "${ip}" was accepted
    END

Test File Path Field Format Validation
    [Documentation]    Verify TLS_CERT and TLS_KEY accept valid file paths
    [Tags]    validation    format    file-path
    Create Session    bloom    ${BASE_URL}

    # Valid file paths
    ${valid_paths}=    Create List
    ...    /etc/ssl/certs/cert.pem
    ...    /home/user/certs/tls.crt
    ...    ./relative/path/cert.pem
    ...    /var/lib/kubernetes/cert.pem
    ...    /tmp/test-cert.pem

    FOR    ${path}    IN    @{valid_paths}
        ${config}=    Create Dictionary
        ...    FIRST_NODE=${True}
        ...    GPU_NODE=${False}
        ...    DOMAIN=test.example.com
        ...    USE_CERT_MANAGER=${False}
        ...    CERT_OPTION=existing
        ...    TLS_CERT=${path}
        ...    TLS_KEY=${path}
        ...    NO_DISKS_FOR_CLUSTER=${True}
        ${payload}=    Create Dictionary    config=${config}
        ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
        ${json}=    Set Variable    ${response.json()}
        Should Be True    ${json}[valid]    msg=Valid path "${path}" was rejected
    END

    # Invalid file paths (if validation exists)
    ${invalid_paths}=    Create List
    ...    ${EMPTY}
    ...    ${SPACE}${SPACE}${SPACE}

    FOR    ${path}    IN    @{invalid_paths}
        ${config}=    Create Dictionary
        ...    FIRST_NODE=${True}
        ...    GPU_NODE=${False}
        ...    DOMAIN=test.example.com
        ...    USE_CERT_MANAGER=${False}
        ...    CERT_OPTION=existing
        ...    TLS_CERT=${path}
        ...    TLS_KEY=${path}
        ...    NO_DISKS_FOR_CLUSTER=${True}
        ${payload}=    Create Dictionary    config=${config}
        ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
        ${json}=    Set Variable    ${response.json()}
        Should Not Be True    ${json}[valid]    msg=Empty/whitespace path was accepted
    END

Test Browser Field Format Validation With Real-Time Feedback
    [Documentation]    Verify browser shows validation errors for invalid field formats
    [Tags]    browser    validation    format
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Test invalid domain format in browser
    ${domain_field_exists}=    Get Element Count    input[name="DOMAIN"]
    IF    ${domain_field_exists} > 0
        # Enter invalid domain
        Fill Text    input[name="DOMAIN"]    invalid domain with spaces

        # Trigger validation by clicking validate button
        Click    id=validate-btn

        # Wait for error to appear
        Sleep    1s

        # Check if error message is shown (either in error div or field-specific error)
        ${error_visible}=    Get Element Count    css=.validation-error:not(:empty), #error:not(.hidden)
        Should Be True    ${error_visible} > 0    msg=No validation error shown for invalid domain

        Take Screenshot    filename=validation_error_invalid_domain    fullPage=True
    END

    Close Browser

Test Required Field Validation
    [Documentation]    Verify required fields are enforced
    [Tags]    validation    required
    Create Session    bloom    ${BASE_URL}

    # Missing required DOMAIN for first node
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${True}
    ...    GPU_NODE=${False}
    ${payload}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
    ${json}=    Set Variable    ${response.json()}
    Should Not Be True    ${json}[valid]    msg=Missing required DOMAIN was accepted
    Should Not Be Empty    ${json}[errors]    msg=No validation errors returned for missing DOMAIN

    # Missing required SERVER_IP for additional node
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${False}
    ...    GPU_NODE=${False}
    ...    JOIN_TOKEN=test-token
    ${payload}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
    ${json}=    Set Variable    ${response.json()}
    Should Not Be True    ${json}[valid]    msg=Missing required SERVER_IP was accepted

    # Missing required JOIN_TOKEN for additional node
    ${config}=    Create Dictionary
    ...    FIRST_NODE=${False}
    ...    GPU_NODE=${False}
    ...    SERVER_IP=192.168.1.1
    ${payload}=    Create Dictionary    config=${config}
    ${response}=    POST On Session    bloom    /api/validate    json=${payload}    expected_status=any
    ${json}=    Set Variable    ${response.json()}
    Should Not Be True    ${json}[valid]    msg=Missing required JOIN_TOKEN was accepted
