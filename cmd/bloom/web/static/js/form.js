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

        group.appendChild(input);
    } else {
        input = document.createElement('input');
        input.type = 'text';
        input.id = argument.key;
        input.name = argument.key;
        input.value = getDefaultValue(argument);
        input.placeholder = argument.default || '';
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
            if (!isFieldVisible(argument, config)) {
                field.classList.add('hidden');
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
        if (shouldBeVisible) {
            field.classList.remove('hidden');
        } else {
            field.classList.add('hidden');
        }
    });

    // Hide sections where all fields are hidden
    hideSectionsWithAllHiddenFields();
}

function getFormData(schema) {
    const config = {};

    schema.forEach(argument => {
        const field = document.getElementById(argument.key);
        if (!field) return;

        // Skip hidden fields
        const group = field.closest('.form-group');
        if (group && group.classList.contains('hidden')) {
            return;
        }

        if (argument.type === 'bool') {
            config[argument.key] = field.checked;
        } else if (argument.type === 'array') {
            // For now, arrays are stored as empty arrays or parsed from JSON
            config[argument.key] = argument.default || [];
        } else {
            const value = field.value.trim();
            if (value !== '') {
                config[argument.key] = value;
            } else if (argument.default !== '') {
                config[argument.key] = argument.default;
            }
        }
    });

    return config;
}
