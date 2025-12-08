// app.js - Main application logic

let schema = null;
let currentConfig = {};

async function init() {
    try {
        // Fetch schema
        schema = await fetchSchema();

        // Hide loading, show form
        document.getElementById('loading').classList.add('hidden');
        document.getElementById('config-form').classList.remove('hidden');

        // Render form with default config
        currentConfig = {};
        schema.forEach(arg => {
            currentConfig[arg.key] = getDefaultValue(arg);
        });
        renderForm(schema, currentConfig);

        // Setup event listeners
        setupEventListeners();
    } catch (error) {
        showError('Failed to load configuration schema: ' + error.message);
    }
}

function setupEventListeners() {
    // Form change listener - update visibility and config
    document.getElementById('config-form').addEventListener('change', (e) => {
        currentConfig = getFormData(schema);
        updateFieldVisibility(schema, currentConfig);
    });

    // Validate button
    document.getElementById('validate-btn').addEventListener('click', async () => {
        await handleValidate();
    });

    // Form submit - generate YAML
    document.getElementById('config-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        await handleGenerate();
    });

    // Download button
    document.getElementById('download-btn').addEventListener('click', () => {
        downloadYAML();
    });

    // Edit button
    document.getElementById('edit-btn').addEventListener('click', () => {
        document.getElementById('preview').classList.add('hidden');
        document.getElementById('config-form').classList.remove('hidden');
    });
}

async function handleValidate() {
    currentConfig = getFormData(schema);

    // Client-side validation
    const clientErrors = validateForm(schema, currentConfig);
    if (clientErrors.length > 0) {
        showError('Validation errors:\n' + clientErrors.join('\n'));
        return;
    }

    // Server-side validation
    const result = await validateWithServer(currentConfig);
    if (result.valid) {
        showSuccess('Configuration is valid!');
    } else {
        showError('Validation errors:\n' + result.errors.join('\n'));
    }
}

async function handleGenerate() {
    currentConfig = getFormData(schema);

    // Validate first
    const clientErrors = validateForm(schema, currentConfig);
    if (clientErrors.length > 0) {
        showError('Please fix validation errors before generating');
        return;
    }

    try {
        const response = await fetch('/api/generate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ config: currentConfig }),
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const result = await response.json();

        // Show preview
        document.getElementById('yaml-preview').textContent = result.yaml;
        document.getElementById('config-form').classList.add('hidden');
        document.getElementById('preview').classList.remove('hidden');

        // Store for download
        window.generatedYAML = result.yaml;
    } catch (error) {
        showError('Failed to generate YAML: ' + error.message);
    }
}

function downloadYAML() {
    if (!window.generatedYAML) {
        showError('No YAML generated yet');
        return;
    }

    const blob = new Blob([window.generatedYAML], { type: 'text/yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'bloom.yaml';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

function showError(message) {
    const errorDiv = document.getElementById('error');
    errorDiv.textContent = message;
    errorDiv.classList.remove('hidden');
    setTimeout(() => {
        errorDiv.classList.add('hidden');
    }, 5000);
}

function showSuccess(message) {
    // Reuse error div with different styling (could add success div later)
    const errorDiv = document.getElementById('error');
    errorDiv.textContent = message;
    errorDiv.style.background = '#d4edda';
    errorDiv.style.color = '#155724';
    errorDiv.style.borderColor = '#c3e6cb';
    errorDiv.classList.remove('hidden');
    setTimeout(() => {
        errorDiv.classList.add('hidden');
        errorDiv.style.background = '';
        errorDiv.style.color = '';
        errorDiv.style.borderColor = '';
    }, 3000);
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', init);
