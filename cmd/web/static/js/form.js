// form.js - Dynamic form generation from schema

function createFormField(argument, config) {
    const group = document.createElement('div');
    group.className = 'form-group';
    group.dataset.key = argument.key;

    // Add conditional class if field has dependencies
    if (argument.dependencies) {
        group.classList.add('conditional');
    }

    // Create label
    const label = document.createElement('label');
    label.setAttribute('for', argument.key);
    label.textContent = argument.key;
    if (argument.required) {
        label.textContent += ' *';
    }
    group.appendChild(label);

    // Create description
    if (argument.description) {
        const desc = document.createElement('div');
        desc.className = 'description';
        desc.textContent = argument.description;
        group.appendChild(desc);
    }

    // Create input based on type
    let input;
    if (argument.type === 'bool') {
        const wrapper = document.createElement('div');
        wrapper.className = 'checkbox-label';

        input = document.createElement('input');
        input.type = 'checkbox';
        input.id = argument.key;
        input.name = argument.key;
        input.checked = getDefaultValue(argument);

        const checkLabel = document.createElement('span');
        checkLabel.textContent = 'Enabled';

        wrapper.appendChild(input);
        wrapper.appendChild(checkLabel);
        group.appendChild(wrapper);
    } else if (argument.type === 'enum' && argument.options) {
        input = document.createElement('select');
        input.id = argument.key;
        input.name = argument.key;

        // Add required attribute
        if (argument.required) {
            input.required = true;
        }

        // Add empty option
        const emptyOption = document.createElement('option');
        emptyOption.value = '';
        emptyOption.textContent = '-- Select --';
        input.appendChild(emptyOption);

        // Add options
        argument.options.forEach(opt => {
            const option = document.createElement('option');
            option.value = opt;
            option.textContent = opt;
            if (opt === argument.default) {
                option.selected = true;
            }
            input.appendChild(option);
        });

        // Validate on change and blur for select
        const validateSelect = () => {
            const errorDiv = document.getElementById(`error-${argument.key}`);
            if (!input.validity.valid) {
                errorDiv.textContent = input.validationMessage;
            } else {
                errorDiv.textContent = '';
            }
        };

        input.addEventListener('blur', validateSelect);
        input.addEventListener('change', validateSelect);

        group.appendChild(input);
    } else if (argument.type === 'array') {
        // Handle array fields (like ADDITIONAL_OIDC_PROVIDERS)
        const arrayContainer = createArrayField(argument);
        group.appendChild(arrayContainer);
        
        // Arrays don't have a traditional input element
        input = null;
    } else {
        input = document.createElement('input');
        input.id = argument.key;
        input.name = argument.key;
        input.value = getDefaultValue(argument);
        input.placeholder = argument.default || '';

        // Apply HTML5 validation attributes from schema
        if (argument.pattern) {
            input.setAttribute('pattern', argument.pattern);
            if (argument.patternTitle) {
                input.title = argument.patternTitle;
            }
        }

        // Set input type based on schema type, not field name
        // Always use 'text' when pattern is provided - browser validation will use the pattern
        if (argument.pattern) {
            input.type = 'text';
        } else if (argument.key.includes('URL') || argument.key.includes('ISSUER')) {
            input.type = 'url';
        } else if (argument.key.includes('EMAIL')) {
            input.type = 'email';
        } else {
            input.type = 'text';
        }

        // Add required attribute
        if (argument.required) {
            input.required = true;
        }

        // Validate on blur (when focus leaves)
        input.addEventListener('blur', () => {
            const errorDiv = document.getElementById(`error-${argument.key}`);
            if (!input.validity.valid) {
                // Use custom message from title if pattern mismatch
                if (input.validity.patternMismatch && input.title) {
                    input.setCustomValidity(input.title);
                    errorDiv.textContent = input.title;
                } else {
                    input.setCustomValidity('');
                    errorDiv.textContent = input.validationMessage;
                }
            } else {
                input.setCustomValidity('');
                errorDiv.textContent = '';
            }
        });

        // Clear error and custom validity when user starts typing again
        input.addEventListener('input', () => {
            input.setCustomValidity('');
        });

        input.addEventListener('focus', () => {
            const errorDiv = document.getElementById(`error-${argument.key}`);
            errorDiv.textContent = '';
        });

        group.appendChild(input);
    }

    // Add validation error placeholder
    const errorDiv = document.createElement('div');
    errorDiv.className = 'validation-error';
    errorDiv.id = `error-${argument.key}`;
    group.appendChild(errorDiv);

    return group;
}

function renderForm(schema, config) {
    const container = document.getElementById('form-fields');
    container.innerHTML = '';

    // Group fields by section
    const sections = {};
    schema.forEach(argument => {
        const section = argument.section || 'Other';
        if (!sections[section]) {
            sections[section] = [];
        }
        sections[section].push(argument);
    });

    // Render sections in order
    const sectionOrder = [
        '📋 Basic Configuration',
        '🔗 Additional Node Configuration',
        '💾 Storage Configuration',
        '🔒 SSL/TLS Configuration',
        '⚙️ Advanced Configuration',
        '💻 Command Line Options',
        'Other'
    ];

    sectionOrder.forEach(sectionName => {
        const fields = sections[sectionName];
        if (!fields || fields.length === 0) return;

        // Create section container
        const sectionDiv = document.createElement('div');
        sectionDiv.className = 'config-section';

        // Create section header
        const headerDiv = document.createElement('div');
        headerDiv.className = 'section-header';
        headerDiv.textContent = sectionName;
        sectionDiv.appendChild(headerDiv);

        // Create section content
        const contentDiv = document.createElement('div');
        contentDiv.className = 'section-content';

        fields.forEach(argument => {
            const field = createFormField(argument, config);

            // Set initial visibility based on dependencies
            const shouldBeVisible = isFieldVisible(argument, config);
            if (!shouldBeVisible) {
                field.classList.add('hidden');
                // Remove required attribute for initially hidden fields
                const inputElement = document.getElementById(argument.key);
                if (inputElement) {
                    inputElement.required = false;
                }
            }

            contentDiv.appendChild(field);
        });

        sectionDiv.appendChild(contentDiv);
        container.appendChild(sectionDiv);
    });

    // Hide sections where all fields are initially hidden
    hideSectionsWithAllHiddenFields();
}

function hideSectionsWithAllHiddenFields() {
    document.querySelectorAll('.config-section').forEach(section => {
        const allFields = section.querySelectorAll('.form-group');
        const visibleFields = section.querySelectorAll('.form-group:not(.hidden)');

        if (allFields.length > 0 && visibleFields.length === 0) {
            section.classList.add('hidden');
        } else {
            section.classList.remove('hidden');
        }
    });
}

function updateFieldVisibility(schema, config) {
    schema.forEach(argument => {
        const field = document.querySelector(`.form-group[data-key="${argument.key}"]`);
        if (!field) return;

        const shouldBeVisible = isFieldVisible(argument, config);
        const inputElement = document.getElementById(argument.key);

        if (shouldBeVisible) {
            field.classList.remove('hidden');
            // Restore required attribute if field should be required
            if (inputElement && argument.required) {
                inputElement.required = true;
            }
        } else {
            field.classList.add('hidden');
            // Remove required attribute for hidden fields to prevent validation errors
            if (inputElement) {
                inputElement.required = false;
            }
        }
    });

    // Hide sections where all fields are hidden
    hideSectionsWithAllHiddenFields();
}

function getFormData(schema) {
    const config = {};

    schema.forEach(argument => {
        // Handle array fields differently since they don't have a single input element
        if (argument.type === 'array') {
            // Check if array field group exists and is visible
            const group = document.querySelector(`[data-key="${argument.key}"]`);
            if (group && !group.classList.contains('hidden')) {
                config[argument.key] = collectArrayData(argument.key);
            }
            return;
        }
        
        const field = document.getElementById(argument.key);
        if (!field) return;

        // Skip hidden fields
        const group = field.closest('.form-group');
        if (group && group.classList.contains('hidden')) {
            return;
        }

        if (argument.type === 'bool') {
            config[argument.key] = field.checked;
        } else {
            const value = field.value.trim();
            if (value !== '') {
                config[argument.key] = value;
            } else if (argument.default !== '') {
                // If field is empty but has non-empty default, set as explicit empty string
                // This indicates user intentionally cleared a pre-populated field
                config[argument.key] = "";
            } else {
                // Field has no default or empty default, use default or empty string
                config[argument.key] = argument.default || "";
            }
        }
    });

    return config;
}

// Create array field component for dynamic lists (like OIDC providers)
function createArrayField(argument) {
    const container = document.createElement('div');
    container.className = 'array-field-container';
    container.dataset.key = argument.key;
    
    // Create items container
    const itemsContainer = document.createElement('div');
    itemsContainer.className = 'array-items-container';
    container.appendChild(itemsContainer);
    
    // Create "Add Item" button
    const addButton = document.createElement('button');
    addButton.type = 'button';
    addButton.className = 'btn btn-secondary btn-sm array-add-btn';
    addButton.textContent = getAddButtonText(argument.key);
    container.appendChild(addButton);
    
    // Initialize with default items if any
    const defaultItems = argument.default || [];
    defaultItems.forEach((item, index) => {
        addArrayItem(argument, itemsContainer, item, index);
    });
    
    // Add click handler for the "Add" button
    addButton.addEventListener('click', () => {
        const newIndex = itemsContainer.children.length;
        addArrayItem(argument, itemsContainer, null, newIndex);
    });
    
    return container;
}

// Get appropriate button text based on array type
function getAddButtonText(key) {
    if (key === 'ADDITIONAL_OIDC_PROVIDERS') {
        return '+ Add OIDC Provider';
    }
    return '+ Add Item';
}

// Add a single array item
function addArrayItem(argument, container, itemData, index) {
    const item = document.createElement('div');
    item.className = 'array-item';
    item.dataset.index = index;
    
    // Create item header with remove button
    const itemHeader = document.createElement('div');
    itemHeader.className = 'array-item-header';
    
    const itemTitle = document.createElement('h4');
    itemTitle.className = 'array-item-title';
    itemTitle.textContent = getItemTitle(argument.key, index);
    itemHeader.appendChild(itemTitle);
    
    const removeButton = document.createElement('button');
    removeButton.type = 'button';
    removeButton.className = 'btn btn-danger btn-sm array-remove-btn';
    removeButton.textContent = '×';
    removeButton.title = 'Remove';
    itemHeader.appendChild(removeButton);
    
    item.appendChild(itemHeader);
    
    // Create item content based on schema
    const itemContent = document.createElement('div');
    itemContent.className = 'array-item-content';
    
    if (argument.key === 'ADDITIONAL_OIDC_PROVIDERS') {
        createOIDCProviderFields(itemContent, itemData, index);
    } else {
        // Generic array item (for future array fields)
        createGenericArrayItem(itemContent, itemData, index, argument);
    }
    
    item.appendChild(itemContent);
    container.appendChild(item);
    
    // Add remove button handler
    removeButton.addEventListener('click', () => {
        container.removeChild(item);
        reindexArrayItems(container);
    });
}

// Get appropriate title for array items
function getItemTitle(key, index) {
    if (key === 'ADDITIONAL_OIDC_PROVIDERS') {
        return `OIDC Provider ${index + 1}`;
    }
    return `Item ${index + 1}`;
}

// Create OIDC provider specific fields
function createOIDCProviderFields(container, providerData, index) {
    const urlGroup = document.createElement('div');
    urlGroup.className = 'form-group';
    
    const urlLabel = document.createElement('label');
    urlLabel.textContent = 'Provider URL *';
    urlLabel.setAttribute('for', `oidc_url_${index}`);
    urlGroup.appendChild(urlLabel);
    
    const urlInput = document.createElement('input');
    urlInput.type = 'url';
    urlInput.id = `oidc_url_${index}`;
    urlInput.name = `oidc_url_${index}`;
    urlInput.required = true;
    urlInput.placeholder = 'https://kc.example.com/realms/k8s';
    urlInput.value = providerData ? providerData.url || '' : '';
    urlGroup.appendChild(urlInput);
    
    const urlError = document.createElement('div');
    urlError.id = `error-oidc_url_${index}`;
    urlError.className = 'error-message';
    urlGroup.appendChild(urlError);
    
    container.appendChild(urlGroup);
    
    // Audiences field (simple comma-separated for now, can enhance later)
    const audiencesGroup = document.createElement('div');
    audiencesGroup.className = 'form-group';
    
    const audiencesLabel = document.createElement('label');
    audiencesLabel.textContent = 'Audiences';
    audiencesLabel.setAttribute('for', `oidc_audiences_${index}`);
    audiencesGroup.appendChild(audiencesLabel);
    
    const audiencesInput = document.createElement('input');
    audiencesInput.type = 'text';
    audiencesInput.id = `oidc_audiences_${index}`;
    audiencesInput.name = `oidc_audiences_${index}`;
    audiencesInput.placeholder = 'k8s, api (comma-separated)';
    
    // Convert audiences array to comma-separated string for editing
    if (providerData && providerData.audiences) {
        audiencesInput.value = providerData.audiences.join(', ');
    } else {
        audiencesInput.value = 'k8s'; // Default audience
    }
    
    audiencesGroup.appendChild(audiencesInput);
    
    const audiencesDesc = document.createElement('div');
    audiencesDesc.className = 'description';
    audiencesDesc.textContent = 'Comma-separated list of accepted audiences for this provider';
    audiencesGroup.appendChild(audiencesDesc);
    
    container.appendChild(audiencesGroup);
    
    // Add validation
    urlInput.addEventListener('blur', () => {
        const errorDiv = document.getElementById(`error-oidc_url_${index}`);
        if (!urlInput.validity.valid) {
            errorDiv.textContent = urlInput.validationMessage || 'Please enter a valid OIDC provider URL';
        } else {
            errorDiv.textContent = '';
        }
    });
}

// Create generic array item (for future use)
function createGenericArrayItem(container, itemData, index, argument) {
    const input = document.createElement('input');
    input.type = 'text';
    input.name = `${argument.key}_${index}`;
    input.value = itemData || '';
    input.placeholder = `${argument.key} item`;
    container.appendChild(input);
}

// Reindex array items after removal
function reindexArrayItems(container) {
    Array.from(container.children).forEach((item, newIndex) => {
        item.dataset.index = newIndex;
        
        // Update title
        const title = item.querySelector('.array-item-title');
        if (title) {
            const key = container.parentElement.dataset.key;
            title.textContent = getItemTitle(key, newIndex);
        }
        
        // Update field names and IDs
        const inputs = item.querySelectorAll('input');
        inputs.forEach(input => {
            const oldName = input.name;
            const baseName = oldName.replace(/_\d+$/, '');
            input.name = `${baseName}_${newIndex}`;
            input.id = input.id.replace(/_\d+$/, `_${newIndex}`);
        });
        
        // Update labels
        const labels = item.querySelectorAll('label');
        labels.forEach(label => {
            const forAttr = label.getAttribute('for');
            if (forAttr) {
                label.setAttribute('for', forAttr.replace(/_\d+$/, `_${newIndex}`));
            }
        });
        
        // Update error divs
        const errorDivs = item.querySelectorAll('.error-message');
        errorDivs.forEach(errorDiv => {
            errorDiv.id = errorDiv.id.replace(/_\d+$/, `_${newIndex}`);
        });
    });
}

// Collect data from array fields
function collectArrayData(key) {
    console.log('Collecting array data for:', key);
    const container = document.querySelector(`[data-key="${key}"] .array-items-container`);
    if (!container) {
        console.log('No container found for:', key);
        return [];
    }
    
    console.log('Container children count:', container.children.length);
    
    const items = [];
    
    if (key === 'ADDITIONAL_OIDC_PROVIDERS') {
        // Collect OIDC provider data
        Array.from(container.children).forEach((item, index) => {
            const urlInput = item.querySelector(`#oidc_url_${index}`);
            const audiencesInput = item.querySelector(`#oidc_audiences_${index}`);
            
            if (urlInput && urlInput.value.trim()) {
                const provider = {
                    url: urlInput.value.trim()
                };
                
                // Parse audiences from comma-separated string
                if (audiencesInput && audiencesInput.value.trim()) {
                    provider.audiences = audiencesInput.value
                        .split(',')
                        .map(aud => aud.trim())
                        .filter(aud => aud.length > 0);
                } else {
                    provider.audiences = ['k8s']; // Default audience
                }
                
                items.push(provider);
            }
        });
    } else {
        // Generic array collection (for future array fields)
        Array.from(container.children).forEach((item, index) => {
            const input = item.querySelector(`[name="${key}_${index}"]`);
            if (input && input.value.trim()) {
                items.push(input.value.trim());
            }
        });
    }
    
    return items;
}
