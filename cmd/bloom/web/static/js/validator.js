// validator.js - Client-side validation

function clearValidationErrors() {
    document.querySelectorAll('.validation-error').forEach(el => {
        el.textContent = '';
    });
}

function clearValidationError(key) {
    const errorDiv = document.getElementById(`error-${key}`);
    if (errorDiv) {
        errorDiv.textContent = '';
    }
}

function showValidationError(key, message) {
    const errorDiv = document.getElementById(`error-${key}`);
    if (errorDiv) {
        errorDiv.textContent = message;
    }
}

function validateField(argument, value, config) {
    // Check if field is required
    if (argument.required) {
        const isVisible = isFieldVisible(argument, config);
        if (isVisible) {
            if (argument.type === 'bool') {
                // Booleans are always valid
            } else if (!value || value === '') {
                return `${argument.key} is required`;
            }
        }
    }

    // Type-specific validation
    if (value && value !== '') {
        switch (argument.type) {
            case 'enum':
                if (argument.options && !argument.options.includes(value)) {
                    return `${argument.key} must be one of: ${argument.options.join(', ')}`;
                }
                break;
        }
    }

    return null;
}

function validateForm(schema, config) {
    clearValidationErrors();
    const errors = [];

    schema.forEach(argument => {
        const value = config[argument.key];
        const error = validateField(argument, value, config);
        if (error) {
            errors.push(error);
            showValidationError(argument.key, error);
        }
    });

    return errors;
}

async function validateWithServer(config) {
    try {
        const response = await fetch('/api/validate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ config }),
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const result = await response.json();
        return result;
    } catch (error) {
        console.error('Server validation failed:', error);
        return {
            valid: false,
            errors: ['Failed to validate with server: ' + error.message],
        };
    }
}
