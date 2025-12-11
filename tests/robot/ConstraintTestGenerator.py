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
            "../../schema/bloom.yaml.schema.yaml",   # Local relative
            "../schema/bloom.yaml.schema.yaml",      # Alternative local
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

    def extract_fields_from_conditions(self, conditions):
        """Extract field names from condition strings"""
        field_set = set()
        for condition in conditions:
            # Split by && for AND logic
            parts = condition.split(' && ')
            for part in parts:
                part = part.strip()
                # Extract field name (everything before == or !=)
                if ' == ' in part:
                    field_name = part.split(' == ')[0].strip()
                    field_set.add(field_name)
                elif ' != ' in part:
                    field_name = part.split(' != ')[0].strip()
                    field_set.add(field_name)
        return list(field_set)

    def parse_condition_for_test(self, condition):
        """
        Parse a condition string and return field/value mappings for testing.
        Example: "NO_DISKS_FOR_CLUSTER == true && CLUSTER_DISKS == ''"
        Returns: {'NO_DISKS_FOR_CLUSTER': 'true', 'CLUSTER_DISKS': ''}
        """
        config = {}
        parts = condition.split(' && ')
        for part in parts:
            part = part.strip()
            if ' == ' in part:
                tokens = part.split(' == ', 1)
                key = tokens[0].strip()
                value = tokens[1].strip().strip('"\'')
                # Convert string booleans to actual booleans
                if value.lower() == 'true':
                    config[key] = True
                elif value.lower() == 'false':
                    config[key] = False
                else:
                    config[key] = value
            elif ' != ' in part:
                # For != we need to set the opposite
                tokens = part.split(' != ', 1)
                key = tokens[0].strip()
                value = tokens[1].strip().strip('"\'')
                # For != "", we need a valid non-empty value
                if value == '':
                    # Get valid example from schema
                    valid_value = self.get_valid_example_for_field(key)
                    config[key] = valid_value if valid_value is not None else 'test-value'
                else:
                    # For != "something", set to empty
                    config[key] = ''
        return config

    def get_required_fields_for_valid_config(self):
        """Return minimal required fields for a valid config"""
        # For web UI, we need FIRST_NODE, DOMAIN, CERT_OPTION, and one storage option
        return {
            'FIRST_NODE': True,
            'DOMAIN': 'cluster.example.com',
            'CERT_OPTION': 'generate'
        }

    def get_field_default_values(self):
        """Return default values for fields to create invalid state"""
        return {
            'NO_DISKS_FOR_CLUSTER': False,
            'CLUSTER_DISKS': '',
            'CLUSTER_PREMOUNTED_DISKS': ''
        }

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

    def get_valid_examples_for_fields(self, field_names):
        """Get valid example values for multiple fields"""
        result = {}
        for field_name in field_names:
            example = self.get_valid_example_for_field(field_name)
            if example is not None:
                result[field_name] = example
        return result

    def can_merge_conditions(self, condition1, condition2):
        """
        Check if two conditions can be merged without conflicts.
        Returns (can_merge, merged_config)
        """
        config1 = self.parse_condition_for_test(condition1)
        config2 = self.parse_condition_for_test(condition2)

        # Check for conflicting values
        for key in config1:
            if key in config2 and config1[key] != config2[key]:
                return False, {}

        # Merge configs
        merged = {**config1, **config2}
        return True, merged
