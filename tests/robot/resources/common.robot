*** Settings ***
Documentation     Common resources for Bloom Web UI tests
Library           RequestsLibrary
Library           Collections
Library           OperatingSystem

*** Variables ***
${BASE_URL}       http://localhost:8080
${API_BASE}       ${BASE_URL}/api

*** Keywords ***
Start Bloom Web UI
    [Documentation]    Start the Bloom web UI server in background
    [Arguments]        ${port}=8080
    ${result}=    Start Process    ./dist/bloom-v2    webui    --port    ${port}
    ...           stdout=webui.log    stderr=webui.log    alias=webui
    Sleep    2s    Wait for server to start
    RETURN    ${result}

Stop Bloom Web UI
    [Documentation]    Stop the Bloom web UI server
    Terminate Process    webui
    Wait For Process    webui    timeout=5s

Create Session To Web UI
    [Documentation]    Create HTTP session to Web UI
    Create Session    webui    ${BASE_URL}    verify=False

Verify Server Is Running
    [Documentation]    Verify the web UI server is accessible
    ${response}=    GET On Session    webui    /    expected_status=200
    RETURN    ${response}
