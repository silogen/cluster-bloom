"""
Helper library to load YAML schema and extract test examples for Robot Framework
"""
import yaml
import os


def load_schema_examples():
    """
    Load the YAML schema and extract all field examples for testing.

    Returns a list of field info dictionaries containing:
    - field: Field name (e.g., "DOMAIN")
    - fieldId: HTML element ID (same as field name)
    - type: Schema type (e.g., "domain", "ipv4")
    - valid: List of valid example values
    - invalid: List of invalid example values
    - visibility: List of steps to make field visible (optional)
    """
    # Try multiple possible paths
    script_dir = os.path.dirname(os.path.abspath(__file__))
    schema_paths = [
        '/robot/schema/bloom.yaml.schema.yaml',  # Docker mount
        os.path.join(script_dir, '../../schema/bloom.yaml.schema.yaml'),  # Local relative
        '/workspace/cluster-bloom/schema/bloom.yaml.schema.yaml',  # Absolute workspace path
    ]

    schema_path = None
    for path in schema_paths:
        if os.path.exists(path):
            schema_path = path
            break

    if not schema_path:
        # Debug: show what we tried
        tried = '\n  '.join(schema_paths)
        raise FileNotFoundError(f"Could not find bloom.yaml.schema.yaml. Tried:\n  {tried}")

    with open(schema_path, 'r') as f:
        schema = yaml.safe_load(f)

    # Map of field names to their types and examples
    field_examples = []

    # Get type definitions with examples
    type_defs = schema.get('types', {})

    # Get field mappings
    fields = schema.get('schema', {}).get('mapping', {})

    for field_name, field_def in fields.items():
        field_type = field_def.get('type')

        # Skip fields without custom types (bool, str without patterns, etc.)
        if field_type not in type_defs:
            continue

        # Get examples from type definition
        type_def = type_defs[field_type]
        examples = type_def.get('examples', {})

        valid_examples = examples.get('valid', [])
        invalid_examples = examples.get('invalid', [])

        # Skip if no examples
        if not valid_examples and not invalid_examples:
            continue

        # Determine visibility requirements
        visibility_steps = get_visibility_steps(field_name, field_def)

        field_info = {
            'field': field_name,
            'fieldId': field_name,
            'type': field_type,
            'valid': valid_examples,
            'invalid': invalid_examples,
        }

        if visibility_steps:
            field_info['visibility'] = visibility_steps

        field_examples.append(field_info)

    return field_examples


def get_visibility_steps(field_name, field_def):
    """
    Get the steps needed to make a field visible in the UI.

    Returns a list of step dictionaries with 'action' and 'target' keys.
    """
    steps = []

    # Fields that require CERT_OPTION=existing
    if field_name in ['TLS_CERT', 'TLS_KEY']:
        steps.append({'action': 'wait', 'target': 'CERT_OPTION'})
        steps.append({'action': 'select', 'target': 'CERT_OPTION', 'value': 'existing'})
        steps.append({'action': 'wait', 'target': field_name})

    # Fields that require FIRST_NODE=false
    elif field_name in ['SERVER_IP', 'JOIN_TOKEN', 'CONTROL_PLANE']:
        steps.append({'action': 'uncheck', 'target': 'FIRST_NODE'})
        steps.append({'action': 'wait', 'target': field_name})

    # Fields that require GPU_NODE=true (ensure it's checked)
    elif field_name in ['ROCM_BASE_URL', 'ROCM_DEB_PACKAGE']:
        steps.append({'action': 'check', 'target': 'GPU_NODE'})
        steps.append({'action': 'wait', 'target': field_name})

    return steps if steps else None


if __name__ == '__main__':
    # For testing
    examples = load_schema_examples()
    for field in examples:
        print(f"\n{field['field']} ({field['type']}):")
        print(f"  Valid: {field['valid']}")
        print(f"  Invalid: {field['invalid'][:3]}...")  # Show first 3
        if 'visibility' in field:
            print(f"  Visibility: {field['visibility']}")
