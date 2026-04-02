// schema.js - Fetch and cache configuration schema

let cachedSchema = null;

async function fetchSchema() {
    if (cachedSchema) {
        return cachedSchema;
    }

    try {
        const response = await fetch('/api/schema');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        cachedSchema = data.arguments;
        return cachedSchema;
    } catch (error) {
        console.error('Failed to fetch schema:', error);
        throw error;
    }
}

function evaluateDependency(depStr, config) {
    const parts = depStr.split('=');
    if (parts.length !== 2) {
        return false;
    }

    const [key, expectedValue] = parts.map(s => s.trim());
    const actualValue = config[key];

    // Handle boolean comparisons
    if (expectedValue === 'true') {
        return actualValue === true || actualValue === 'true';
    }
    if (expectedValue === 'false') {
        return actualValue === false || actualValue === 'false' || !actualValue;
    }

    // Handle string comparisons
    return actualValue === expectedValue;
}

function isFieldVisible(argument, config) {
    if (!argument.dependencies) {
        return true;
    }

    // Split by comma for multiple dependencies (AND logic)
    const deps = argument.dependencies.split(',');
    return deps.every(dep => evaluateDependency(dep.trim(), config));
}

function getDefaultValue(argument) {
    if (argument.type === 'bool') {
        return argument.default === true;
    }
    if (argument.type === 'array') {
        return argument.default || [];
    }
    return argument.default || '';
}
