// constraints.js - Client-side constraint validation

let cachedConstraints = null;

// Load constraints from schema
async function loadConstraints() {
    if (cachedConstraints) {
        return cachedConstraints;
    }

    try {
        const response = await fetch('/api/schema');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        cachedConstraints = data.constraints || [];
        return cachedConstraints;
    } catch (error) {
        console.error('Failed to load constraints:', error);
        return [];
    }
}

// Check if any of the fields exist in config
function anyFieldPresent(config, fields) {
    for (const field of fields) {
        const val = config[field];
        if (val !== undefined && val !== null && val !== '') {
            return true;
        }
    }
    return false;
}

// Extract field names from condition strings
// Example: "NO_DISKS_FOR_CLUSTER == true && CLUSTER_DISKS == ''" -> ["NO_DISKS_FOR_CLUSTER", "CLUSTER_DISKS"]
function extractFieldsFromConditions(conditions) {
    const fieldMap = new Map();

    for (const condition of conditions) {
        // Split by && for AND logic
        const parts = condition.split(' && ');

        for (let part of parts) {
            part = part.trim();

            // Extract field name (everything before == or !=)
            if (part.includes(' == ')) {
                const tokens = part.split(' == ');
                if (tokens.length === 2) {
                    fieldMap.set(tokens[0].trim(), true);
                }
            } else if (part.includes(' != ')) {
                const tokens = part.split(' != ');
                if (tokens.length === 2) {
                    fieldMap.set(tokens[0].trim(), true);
                }
            }
        }
    }

    return Array.from(fieldMap.keys());
}

// Get config value as normalized string
function getConfigValue(config, key) {
    const val = config[key];
    if (val === undefined || val === null) {
        return '';
    }

    // Convert to string representation
    if (typeof val === 'boolean') {
        return val ? 'true' : 'false';
    }
    if (typeof val === 'string') {
        return val;
    }
    return String(val);
}

// Evaluate a boolean condition string
// Format: "KEY == value && KEY2 == value2" or "KEY != value"
function evaluateCondition(condition, config) {
    // Split by && for AND logic
    const parts = condition.split(' && ');

    for (let part of parts) {
        part = part.trim();

        // Check for != operator
        if (part.includes(' != ')) {
            const tokens = part.split(' != ');
            if (tokens.length !== 2) {
                return false;
            }

            const key = tokens[0].trim();
            let expectedValue = tokens[1].trim();
            expectedValue = expectedValue.replace(/^["']|["']$/g, ''); // Remove quotes

            const actualValue = getConfigValue(config, key);
            if (actualValue === expectedValue) {
                return false; // Values are equal, so != is false
            }
            continue;
        }

        // Check for == operator
        if (part.includes(' == ')) {
            const tokens = part.split(' == ');
            if (tokens.length !== 2) {
                return false;
            }

            const key = tokens[0].trim();
            let expectedValue = tokens[1].trim();
            expectedValue = expectedValue.replace(/^["']|["']$/g, ''); // Remove quotes

            const actualValue = getConfigValue(config, key);
            if (actualValue !== expectedValue) {
                return false;
            }
            continue;
        }

        return false;
    }

    return true;
}

// Check mutually exclusive constraint
function checkMutuallyExclusive(config, fields) {
    const setFields = [];

    for (const field of fields) {
        const val = config[field];
        if (val !== undefined && val !== null && val !== '') {
            setFields.push(field);
        }
    }

    if (setFields.length > 1) {
        return `Fields ${fields.join(', ')} are mutually exclusive, but ${setFields.join(', ')} are set`;
    }

    return null;
}

// Check if a field is set (has a truthy value)
function isFieldSet(config, field) {
    const val = config[field];
    if (val === undefined || val === null) {
        return false;
    }

    // Check boolean fields
    if (typeof val === 'boolean') {
        return val;
    }

    // Check string fields
    if (typeof val === 'string') {
        return val !== '';
    }

    return true;
}

// Check one-of constraint for fields
function checkOneOfFields(config, fields, errorMsg) {
    let setCount = 0;
    const setFields = [];

    for (const field of fields) {
        if (isFieldSet(config, field)) {
            setCount++;
            setFields.push(field);
        }
    }

    if (setCount !== 1) {
        if (errorMsg) {
            return errorMsg;
        }
        if (setCount === 0) {
            return `Exactly one of ${fields.join(', ')} must be set, but none are set`;
        }
        return `Exactly one of ${fields.join(', ')} must be set, but ${setFields.join(', ')} are set`;
    }

    return null;
}

// Validate all constraints
async function validateConstraints(config) {
    const errors = [];
    const constraints = await loadConstraints();

    for (const constraint of constraints) {
        // Mutually exclusive fields - only check if at least one field is present
        if (constraint.mutually_exclusive && constraint.mutually_exclusive.length >= 2) {
            if (anyFieldPresent(config, constraint.mutually_exclusive)) {
                const error = checkMutuallyExclusive(config, constraint.mutually_exclusive);
                if (error) {
                    errors.push(error);
                }
            }
        }

        // One-of constraints - only check if any relevant field is present
        if (constraint.one_of && constraint.one_of.length > 0) {
            if (anyFieldPresent(config, constraint.one_of)) {
                const error = checkOneOfFields(config, constraint.one_of, constraint.error);
                if (error) {
                    errors.push(error);
                }
            }
        }
    }

    return errors;
}
