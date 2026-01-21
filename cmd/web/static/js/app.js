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
    // Form change listener - update visibility, config, and validate
    document.getElementById('config-form').addEventListener('change', (e) => {
        currentConfig = getFormData(schema);
        updateFieldVisibility(schema, currentConfig);

        // Real-time validation for changed field
        if (e.target.name) {
            const argument = schema.find(arg => arg.key === e.target.name);
            if (argument) {
                const value = currentConfig[argument.key];
                const error = validateField(argument, value, currentConfig);
                clearValidationError(argument.key);
                if (error) {
                    showValidationError(argument.key, error);
                }
            }
        }
    });

    // Form submit - generate YAML
    document.getElementById('config-form').addEventListener('submit', async (e) => {
        e.preventDefault();

        // Check HTML5 validation first
        const form = e.target;
        if (!form.checkValidity()) {
            form.reportValidity();
            return;
        }

        await handleGenerate();
    });

    // Download button - now saves to cwd
    document.getElementById('download-btn').addEventListener('click', async () => {
        await saveYAML();
    });

    // Edit button
    document.getElementById('edit-btn').addEventListener('click', () => {
        document.getElementById('preview').classList.add('hidden');
        document.getElementById('config-form').classList.remove('hidden');
    });
}

async function handleGenerate() {
    currentConfig = getFormData(schema);

    // Validate first
    const clientErrors = await validateForm(schema, currentConfig);
    if (clientErrors.length > 0) {
        showError('Validation errors:\n' + clientErrors.join('\n'));
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

async function saveYAML() {
    if (!currentConfig) {
        showError('No configuration available');
        return;
    }

    const filename = document.getElementById('filename').value.trim();
    if (!filename) {
        showError('Please enter a filename');
        return;
    }

    // Get button reference and store original text
    const saveBtn = document.getElementById('download-btn');
    const originalText = saveBtn.textContent;
    
    // Change button text immediately for visual feedback
    saveBtn.textContent = 'Saved';

    try {
        const response = await fetch('/api/save', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                config: currentConfig,
                filename: filename
            }),
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const result = await response.json();
        showSuccess(`Saved to ${result.path}`);
        
        // Keep "Saved" text for 3 seconds, then revert
        setTimeout(() => {
            saveBtn.textContent = originalText;
        }, 1000);
        
    } catch (error) {
        // On error: restore button text immediately
        saveBtn.textContent = originalText;
        showError('Failed to save YAML: ' + error.message);
    }
}

function showError(message) {
    const errorDiv = document.getElementById('error');
    errorDiv.style.whiteSpace = 'pre-line';
    errorDiv.textContent = message;
    errorDiv.classList.remove('hidden');
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
