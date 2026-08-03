"""Unit tests for auth-config.yaml generation

Tests the OIDC authentication configuration file generation in update_rke2.yaml:
- Template substitution correctness
- YAML structure and indentation
- Field placement (jwt: at root level, not nested)
"""
import pytest
import yaml
from pathlib import Path


def test_auth_config_generates_valid_yaml(fake_rke2_root, ansible_runner_factory):
    """Test that auth-config.yaml template generates valid YAML

    This catches bugs like:
    - Missing 'jwt:' field at root level
    - Incorrect indentation of 'jwt:' field
    - claimMappings nested under issuer instead of sibling
    """
    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'NEW_DOMAIN': 'test.example.com',
            'CURRENT_DOMAIN': 'old.example.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    auth_config_file = fake_rke2_root / 'auth' / 'auth-config.yaml'
    assert auth_config_file.exists(), "auth-config.yaml should be created"

    # Verify YAML is valid and parses correctly
    content = auth_config_file.read_text()
    try:
        config = yaml.safe_load(content)
    except yaml.YAMLError as e:
        pytest.fail(f"auth-config.yaml is not valid YAML: {e}\n\nContent:\n{content}")

    # Verify structure
    assert config['apiVersion'] == 'apiserver.config.k8s.io/v1beta1'
    assert config['kind'] == 'AuthenticationConfiguration'
    assert 'jwt' in config, "'jwt' field must be at root level"
    assert isinstance(config['jwt'], list), "'jwt' should be a list"


def test_auth_config_jwt_field_not_indented(fake_rke2_root, ansible_runner_factory):
    """Test that 'jwt:' field has NO indentation

    Bug scenario: 'jwt:' was indented with 2 spaces, making it nested
    under 'kind' instead of being at the root level.

    Correct:
    kind: AuthenticationConfiguration
    jwt:
      - issuer: ...

    Incorrect (bug):
    kind: AuthenticationConfiguration
      jwt:  # <-- indented, now nested under 'kind'
        - issuer: ...
    """
    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'NEW_DOMAIN': 'test.example.com',
            'CURRENT_DOMAIN': 'old.example.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0

    auth_config_file = fake_rke2_root / 'auth' / 'auth-config.yaml'
    content = auth_config_file.read_text()

    # Check raw text for indentation
    for line in content.split('\n'):
        if line.strip() == 'jwt:':
            # jwt: should have NO leading spaces
            assert line == 'jwt:', \
                f"'jwt:' field should not be indented. Found: {line!r}"


def test_auth_config_claim_mappings_not_nested(fake_rke2_root, ansible_runner_factory):
    """Test that claimMappings is NOT nested under issuer

    Bug scenario: claimMappings was indented as a child of issuer,
    causing error: 'unknown field "jwt[0].issuer.claimMappings"'

    Correct structure:
    jwt:
      - issuer:
          url: ...
          audiences: ...
        claimMappings:  # <-- sibling of issuer, not child
          username: ...

    Incorrect (bug):
    jwt:
      - issuer:
          url: ...
          audiences: ...
          claimMappings:  # <-- nested under issuer (WRONG)
            username: ...
    """
    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'NEW_DOMAIN': 'test.example.com',
            'CURRENT_DOMAIN': 'old.example.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0

    auth_config_file = fake_rke2_root / 'auth' / 'auth-config.yaml'
    config = yaml.safe_load(auth_config_file.read_text())

    # Navigate to first JWT entry
    jwt_entry = config['jwt'][0]

    # claimMappings should be at the top level of jwt_entry, not under issuer
    assert 'claimMappings' in jwt_entry, \
        "claimMappings should be at JWT entry level"
    assert 'claimMappings' not in jwt_entry['issuer'], \
        "claimMappings should NOT be nested under issuer"


def test_auth_config_domain_substitution(fake_rke2_root, ansible_runner_factory):
    """Test that NEW_DOMAIN is correctly substituted in issuer URL"""
    test_domain = 'custom-domain.example.com'

    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'NEW_DOMAIN': test_domain,
            'CURRENT_DOMAIN': 'old.example.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0

    auth_config_file = fake_rke2_root / 'auth' / 'auth-config.yaml'
    config = yaml.safe_load(auth_config_file.read_text())

    issuer_url = config['jwt'][0]['issuer']['url']
    expected_url = f'https://kc.{test_domain}/realms/airm'

    assert issuer_url == expected_url, \
        f"Expected issuer URL: {expected_url}, got: {issuer_url}"


def test_auth_config_has_required_fields(fake_rke2_root, ansible_runner_factory):
    """Test that all required OIDC fields are present"""
    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'NEW_DOMAIN': 'test.example.com',
            'CURRENT_DOMAIN': 'old.example.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0

    auth_config_file = fake_rke2_root / 'auth' / 'auth-config.yaml'
    config = yaml.safe_load(auth_config_file.read_text())

    jwt_entry = config['jwt'][0]

    # Required issuer fields
    assert 'url' in jwt_entry['issuer']
    assert 'audiences' in jwt_entry['issuer']
    assert jwt_entry['issuer']['audiences'] == ['k8s']

    # Required claimMappings
    assert 'username' in jwt_entry['claimMappings']
    assert 'groups' in jwt_entry['claimMappings']

    # Verify claim details
    assert jwt_entry['claimMappings']['username']['claim'] == 'preferred_username'
    assert jwt_entry['claimMappings']['username']['prefix'] == 'oidc:'
    assert jwt_entry['claimMappings']['groups']['claim'] == 'groups'
    assert jwt_entry['claimMappings']['groups']['prefix'] == 'oidc:'


def test_auth_config_overwrites_existing_file(fake_rke2_root, ansible_runner_factory):
    """Test that auth-config.yaml overwrites existing file

    This is important for recovery scenarios where the file exists
    but has incorrect content from a failed previous update.
    """
    auth_config_file = fake_rke2_root / 'auth' / 'auth-config.yaml'

    # Create existing file with old content
    auth_config_file.write_text("""apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
jwt:
  - issuer:
      url: https://kc.old-domain.com/realms/airm
      audiences: ['k8s']
    claimMappings:
      username:
        claim: preferred_username
        prefix: "oidc:"
      groups:
        claim: groups
        prefix: "oidc:"
""")

    # Run update with new domain
    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'NEW_DOMAIN': 'new-domain.com',
            'CURRENT_DOMAIN': 'old-domain.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0

    # Verify file was overwritten with new domain
    config = yaml.safe_load(auth_config_file.read_text())
    issuer_url = config['jwt'][0]['issuer']['url']

    assert 'new-domain.com' in issuer_url, \
        "File should be overwritten with new domain"
    assert 'old-domain.com' not in issuer_url, \
        "Old domain should be replaced"


def test_auth_config_domain_with_special_characters(fake_rke2_root, ansible_runner_factory):
    """Test that domains with hyphens and numbers work correctly"""
    test_domain = 'test-123.example-domain.com'

    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'NEW_DOMAIN': test_domain,
            'CURRENT_DOMAIN': 'old.example.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0

    auth_config_file = fake_rke2_root / 'auth' / 'auth-config.yaml'
    config = yaml.safe_load(auth_config_file.read_text())

    issuer_url = config['jwt'][0]['issuer']['url']
    assert test_domain in issuer_url, \
        "Domain with special characters should be substituted correctly"
