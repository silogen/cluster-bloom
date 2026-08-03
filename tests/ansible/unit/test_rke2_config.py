"""Unit tests for RKE2 configuration file manipulation

Tests the critical file operations in update_rke2.yaml:
- blockinfile tls-san updates
- Preservation of additional TLS SANs
- YAML indentation correctness
- Missing marker block handling
"""
import pytest
import yaml
from pathlib import Path


def test_blockinfile_preserves_additional_sans(rke2_config_fixtures, ansible_runner_factory):
    """Test that blockinfile preserves non-domain TLS SANs

    This catches the bug where additional SANs (like custom load balancer DNS,
    IP addresses, or extra hostnames) were being lost during domain updates.
    """
    config_dir = rke2_config_fixtures('multi_san')

    # Verify fixture created file with expected content
    config_file = config_dir / 'config.yaml'
    assert config_file.exists(), "Fixture should create config.yaml"
    initial_content = config_file.read_text()
    assert 'extra1.example.com' in initial_content, "Fixture should contain additional SANs"

    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'CURRENT_DOMAIN': 'example.com',
            'NEW_DOMAIN': 'newdomain.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
            # Note: This test assumes the playbook can be parameterized with config path
            # May need playbook modifications to support this
        }
    )

    # Playbook should succeed
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    # Parse the updated config file
    config_text = (config_dir / 'config.yaml').read_text()
    config = yaml.safe_load(config_text)

    # Verify all expected SANs are present
    sans = config.get('tls-san', [])

    assert 'k8s.newdomain.com' in sans, "New domain SAN should be added"
    assert 'k8s.example.com' in sans, "Old domain SAN should be preserved"
    assert 'extra1.example.com' in sans, "Additional hostname SAN should be preserved"
    assert '192.168.1.100' in sans, "IP address SAN should be preserved"


def test_missing_marker_block_creates_new(rke2_config_fixtures, ansible_runner_factory):
    """Test that missing marker block is created

    Handles the case where the config.yaml exists but doesn't have
    the tls-san managed block yet (e.g., manually created config).
    """
    config_dir = rke2_config_fixtures('no_marker')

    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'CURRENT_DOMAIN': 'old.com',
            'NEW_DOMAIN': 'new.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    config_text = (config_dir / 'config.yaml').read_text()

    # Verify marker block was created
    assert '# BEGIN ANSIBLE MANAGED BLOCK - tls-san' in config_text
    assert '# END ANSIBLE MANAGED BLOCK - tls-san' in config_text
    assert 'k8s.new.com' in config_text


def test_yaml_indentation_is_correct(rke2_config_fixtures, ansible_runner_factory):
    """Test that generated YAML has correct 2-space indentation

    This catches the bug where incorrect indentation caused YAML parsing
    errors in RKE2 kube-apiserver, preventing it from starting.

    Correct format:
        tls-san:
          - k8s.domain.com

    Incorrect (would break):
        tls-san:
        - k8s.domain.com  (missing indentation)
    """
    config_dir = rke2_config_fixtures('clean')

    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'CURRENT_DOMAIN': 'example.com',
            'NEW_DOMAIN': 'newdomain.com',
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    config_text = (config_dir / 'config.yaml').read_text()

    # Verify indentation: list items should have 2 spaces before '-'
    for line in config_text.split('\n'):
        if line.strip().startswith('- k8s.'):
            # Line should start with exactly 2 spaces, then '-', then space
            assert line.startswith('  - '), f"Incorrect indentation on line: {line!r}"
            # Should NOT start with more or fewer spaces
            assert not line.startswith('   - '), f"Too much indentation: {line!r}"
            assert not line.startswith(' - '), f"Too little indentation: {line!r}"

    # Also verify the YAML parses correctly
    config = yaml.safe_load(config_text)
    assert 'tls-san' in config
    assert isinstance(config['tls-san'], list)


def test_san_extraction_filters_node_labels(rke2_config_fixtures):
    """Test that shell SAN extraction doesn't include node-label entries

    This catches the critical bug where the sed command read too many lines
    and included node-label entries in the tls-san list, causing
    kube-apiserver to fail with "runtime core not ready".

    The extraction should STOP at the END marker, not continue reading.
    """
    config_file = rke2_config_fixtures('with_node_labels') / 'config.yaml'

    # This simulates the shell command from update_rke2.yaml lines 27-36
    # sed -n '/^tls-san:/,/^# END ANSIBLE MANAGED BLOCK - tls-san/p'
    import subprocess

    result = subprocess.run(
        [
            'sed', '-n',
            '/^tls-san:/,/^# END ANSIBLE MANAGED BLOCK - tls-san/p',
            str(config_file)
        ],
        capture_output=True,
        text=True
    )

    extracted_block = result.stdout

    # Verify node-label entries are NOT in the extracted block
    assert 'node.longhorn.io' not in extracted_block, \
        "node-label entries should not be extracted"
    assert 'cluster-bloom/gpu-node' not in extracted_block, \
        "node-label entries should not be extracted"

    # Verify the actual tls-san entry IS in the block
    assert 'k8s.example.com' in extracted_block, \
        "Actual tls-san entry should be extracted"

    # Now filter out k8s.* domains with grep
    result2 = subprocess.run(
        ['grep', '-v', '^k8s\\.'],
        input=extracted_block,
        capture_output=True,
        text=True
    )

    # After filtering k8s.* domains, we should have an empty or minimal result
    # (This test would need actual additional SANs in the fixture to be meaningful)


def test_dry_run_mode_no_modifications(rke2_config_fixtures, ansible_runner_factory):
    """Test that DRY_RUN=true doesn't modify any files

    Verifies that when running in dry-run mode, the playbook shows
    what would change but doesn't actually write to disk.
    """
    config_dir = rke2_config_fixtures('clean')

    # Read original content
    original_content = (config_dir / 'config.yaml').read_text()

    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'CURRENT_DOMAIN': 'example.com',
            'NEW_DOMAIN': 'newdomain.com',
            'DRY_RUN': True,  # Dry-run mode
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    # Playbook should succeed
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    # File content should be unchanged
    new_content = (config_dir / 'config.yaml').read_text()
    assert new_content == original_content, \
        "DRY_RUN mode should not modify files"


def test_same_domain_update_is_idempotent(rke2_config_fixtures, ansible_runner_factory):
    """Test that updating with the same domain is idempotent

    This is important for recovery scenarios where the update failed
    partway through and needs to be re-run.
    """
    config_dir = rke2_config_fixtures('clean')

    # Run update with current domain = new domain
    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_update_rke2.yaml',
        extravars={
            'CURRENT_DOMAIN': 'example.com',
            'NEW_DOMAIN': 'example.com',  # Same domain
            'DRY_RUN': False,
            'FIRST_NODE': True,
            'CONTROL_PLANE': True,
        }
    )

    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    config = yaml.safe_load((config_dir / 'config.yaml').read_text())

    # Should have k8s.example.com but not duplicated
    sans = config.get('tls-san', [])
    assert sans.count('k8s.example.com') == 1, \
        "Same domain should not be duplicated"
