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
            case 'seq':
                // Validate array/sequence fields
                if (Array.isArray(value) && argument.sequence && argument.sequence[0]) {
                    const itemSchema = argument.sequence[0];
                    if (itemSchema.pattern) {
                        const pattern = new RegExp(itemSchema.pattern);
                        for (let i = 0; i < value.length; i++) {
                            const item = value[i];
                            if (item && !pattern.test(item)) {
                                return itemSchema['pattern-title'] || `${argument.key} item "${item}" is invalid`;
                            }
                        }
                    }
                }
                break;
        }
    }

    return null;
}

async function validateForm(schema, config) {
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

    // Validate constraints (mutually exclusive, one-of, etc.)
    const constraintErrors = await validateConstraints(config);
    errors.push(...constraintErrors);

    return errors;
}
