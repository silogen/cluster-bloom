"""
Dynamic constraint test generator for Robot Framework.
Loads constraints from schema and generates test cases automatically.
"""

import yaml
import os


class ConstraintTestGenerator:
    """Generates test cases dynamically from schema constraints"""

    ROBOT_LIBRARY_SCOPE = 'GLOBAL'

    def __init__(self):
        self.constraints = []
        self._load_constraints()

    def _load_constraints(self):
        """Load constraints from schema file"""
        # Try multiple paths for schema file (Docker vs local)
        possible_paths = [
            "/robot/schema/bloom.yaml.schema.yaml",  # Docker mount
            "../../pkg/config/bloom.yaml.schema.yaml",   # Current location relative
            "../pkg/config/bloom.yaml.schema.yaml",      # Alternative relative
            "../../schema/bloom.yaml.schema.yaml",   # Legacy location
            "../schema/bloom.yaml.schema.yaml",      # Legacy alternative
        ]

        schema_data = None
        for path in possible_paths:
            if os.path.exists(path):
                with open(path, 'r') as f:
                    schema_data = yaml.safe_load(f)
                break

        if schema_data is None:
            raise FileNotFoundError(
                f"Schema file not found in any of: {possible_paths}"
            )

        self.constraints = schema_data.get('constraints', [])
        self.schema = schema_data.get('schema', {})
        self.types = schema_data.get('types', {})

    def get_mutually_exclusive_constraints(self):
        """Return all mutually exclusive constraints"""
        result = []
        for constraint in self.constraints:
            if 'mutually_exclusive' in constraint:
                result.append(constraint['mutually_exclusive'])
        return result

    def get_one_of_constraints(self):
        """Return all one-of constraints"""
        result = []
        for constraint in self.constraints:
            if 'one_of' in constraint:
                result.append({
                    'fields': constraint['one_of'],
                    'error': constraint.get('error', '')
                })
        return result

    def get_valid_examples_for_fields(self, field_names):
        """Get valid example values for multiple fields"""
        result = {}
        for field_name in field_names:
            example = self.get_valid_example_for_field(field_name)
            if example is not None:
                result[field_name] = example
        return result

    def get_valid_example_for_field(self, field_name):
        """Get a valid example value for a field from schema"""
        # Get field definition from schema
        if 'mapping' not in self.schema:
            return None

        field_def = self.schema['mapping'].get(field_name)
        if not field_def:
            return None

        # Check if field has direct examples
        if 'examples' in field_def and field_def['examples']:
            return field_def['examples'][0]

        # Check field type
        field_type = field_def.get('type')
        if not field_type:
            return None

        # For string types, check if there's a custom type with examples
        if field_type in self.types:
            type_def = self.types[field_type]
            if 'examples' in type_def and 'valid' in type_def['examples']:
                valid_examples = type_def['examples']['valid']
                if valid_examples:
                    return valid_examples[0]

        # Fallback defaults by type
        type_defaults = {
            'str': 'test-value',
            'bool': True,
            'int': 1,
        }
        return type_defaults.get(field_type, 'test-value')
