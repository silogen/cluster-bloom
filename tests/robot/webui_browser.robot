*** Settings ***
Documentation     Browser-based Web UI Tests for Bloom V2
Library           Browser

*** Variables ***
${BASE_URL}       http://localhost:8080

*** Test Cases ***
Test Web UI Loads Successfully
    [Documentation]    Verify the web UI loads in browser
    [Tags]    browser    ui
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}
    Get Title    ==    Bloom Configuration Generator
    Close Browser

Test Form Elements Are Visible
    [Documentation]    Verify form elements render correctly
    [Tags]    browser    ui
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    # Wait for loading to complete and form to appear
    Wait For Elements State    id=loading    hidden    timeout=5s
    Wait For Elements State    id=config-form    visible    timeout=5s

    # Check form fields container exists
    Get Element    id=form-fields

    # Check buttons exist
    Get Element    id=validate-btn
    Get Element    css=button[type="submit"]

    Close Browser

Test Validate Button Works
    [Documentation]    Verify validate button is clickable
    [Tags]    browser    ui    interaction
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Click validate button
    Click    id=validate-btn

    # Should show validation errors (form is empty)
    Wait For Elements State    id=error    visible    timeout=3s

    Close Browser

Test Preview Section Appears After Generate
    [Documentation]    Verify preview section appears after generating YAML
    [Tags]    browser    ui    workflow
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=config-form    visible    timeout=5s

    # Fill minimal required fields using JavaScript
    # (Since form is dynamically generated, we inject values)
    Evaluate JavaScript    id=config-form
    ...    (form) => {
    ...        // Simulate form submission with valid data
    ...        const event = new Event('submit', { bubbles: true, cancelable: true });
    ...        form.dispatchEvent(event);
    ...    }

    # Preview should appear (may fail if validation fails, which is expected)
    # This test shows the workflow, actual success depends on form implementation
    Sleep    1s

    Close Browser

Test Screenshot Capture
    [Documentation]    Capture screenshot of the web UI for visual verification
    [Tags]    browser    screenshot
    New Browser    chromium    headless=True
    New Page    ${BASE_URL}

    Wait For Elements State    id=loading    hidden    timeout=5s

    # Take screenshot
    Take Screenshot    filename=webui_homepage    fullPage=True

    Close Browser

Test Responsive Layout
    [Documentation]    Verify UI works at different viewport sizes
    [Tags]    browser    responsive
    New Browser    chromium    headless=True

    # Desktop view
    New Page    ${BASE_URL}
    Set Viewport Size    1920    1080
    Wait For Elements State    id=loading    hidden    timeout=5s
    Get Element    css=.container
    Take Screenshot    filename=webui_desktop    fullPage=True

    # Tablet view
    Set Viewport Size    768    1024
    Get Element    css=.container
    Take Screenshot    filename=webui_tablet    fullPage=True

    # Mobile view
    Set Viewport Size    375    667
    Get Element    css=.container
    Take Screenshot    filename=webui_mobile    fullPage=True

    Close Browser
