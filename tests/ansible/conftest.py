"""Shared pytest fixtures for Ansible playbook tests"""
import pytest
import yaml
import os
from pathlib import Path
from ansible_runner import run


def is_running_in_docker():
    """Check if we're running inside a Docker container

    Returns:
        bool: True if running in Docker, False otherwise
    """
    # Check for /.dockerenv file (Docker creates this)
    if Path("/.dockerenv").exists():
        return True

    # Check /proc/1/cgroup for docker
    try:
        with open('/proc/1/cgroup', 'r') as f:
            return 'docker' in f.read()
    except:
        return False


@pytest.fixture(scope="session", autouse=True)
def configure_ansible_for_tests():
    """Configure Ansible to use mock modules

    This fixture automatically runs for all tests and configures:
    - ANSIBLE_LIBRARY: Path to our mock modules
    - ANSIBLE_CONFIG: Path to our ansible.cfg

    Mock modules take precedence over built-in modules, allowing us to:
    - Mock systemd (no systemd in Docker container)
    - Mock kubectl commands (no Kubernetes cluster)
    - Run real file operations (blockinfile, copy, shell with sed/grep)
    """
    # Set Ansible library path to our mocks
    library_path = Path(__file__).parent / 'library'
    os.environ['ANSIBLE_LIBRARY'] = str(library_path)

    # Point to our ansible.cfg
    ansible_cfg = Path(__file__).parent / 'ansible.cfg'
    os.environ['ANSIBLE_CONFIG'] = str(ansible_cfg)

    yield

    # Cleanup
    if 'ANSIBLE_LIBRARY' in os.environ:
        del os.environ['ANSIBLE_LIBRARY']
    if 'ANSIBLE_CONFIG' in os.environ:
        del os.environ['ANSIBLE_CONFIG']


@pytest.fixture
def playbook_dir():
    """Return path to Ansible playbooks directory"""
    return Path(__file__).parent.parent.parent / 'pkg/ansible/runtime/playbooks'


@pytest.fixture
def ansible_runner_factory(playbook_dir, tmp_path):
    """Factory for running Ansible playbooks with custom vars"""
    project_root = Path(__file__).parent.parent.parent

    def _run_playbook(playbook_name, extravars=None):
        # If playbook is in tests/ directory, resolve from project root
        if playbook_name.startswith('tests/'):
            playbook_path = project_root / playbook_name
        # If absolute path, use as-is
        elif Path(playbook_name).is_absolute():
            playbook_path = Path(playbook_name)
        # Otherwise resolve relative to runtime playbooks dir
        else:
            playbook_path = playbook_dir / playbook_name

        result = run(
            private_data_dir=str(tmp_path),
            playbook=str(playbook_path),
            extravars=extravars or {},
            verbosity=2,
            quiet=False,
        )
        return result
    return _run_playbook


@pytest.fixture
def fake_rke2_root():
    """Setup /etc/rancher/rke2 filesystem for testing

    In Docker: Uses real /etc/rancher/rke2 path (safe in container)

    Returns path to rke2 config directory.

    Safety: Only runs in Docker containers. Exits if /etc/rancher/rke2 exists
    outside Docker to prevent modifying real RKE2 installations.
    """
    import os
    import shutil

    rke2_dir = Path("/etc/rancher/rke2")

    # Safety check:if we're not in Docker, refuse to run
    if not is_running_in_docker():
        pytest.fail(
            "SAFETY CHECK FAILED: \n"
            "These tests modify /etc/rancher/rke2 and should only run in containers.\n"
            "Run tests using: ./run_tests_docker.sh"
        )

    # Clean up any existing test files
    if rke2_dir.exists():
        for item in rke2_dir.glob("*"):
            if item.is_file():
                item.unlink()
            elif item.is_dir():
                shutil.rmtree(item)

    rke2_dir.mkdir(parents=True, exist_ok=True)

    # Create initial config.yaml with tls-san block
    config_yaml = rke2_dir / "config.yaml"
    config_yaml.write_text("""# BEGIN ANSIBLE MANAGED BLOCK - tls-san
tls-san:
  - k8s.old-domain.com
  - extra.server.com
# END ANSIBLE MANAGED BLOCK - tls-san
server: https://127.0.0.1:9345
""")

    # Create auth directory
    auth_dir = rke2_dir / "auth"
    auth_dir.mkdir(exist_ok=True)

    yield rke2_dir

    # Cleanup: remove test files but keep directory structure
    if config_yaml.exists():
        config_yaml.unlink()
    for item in (auth_dir).glob("*"):
        item.unlink()


@pytest.fixture
def rke2_config_fixtures():
    """Create multiple RKE2 config fixture states for testing

    Each fixture writes to /etc/rancher/rke2/config.yaml with different initial states.
    Tests should restore state between runs.
    """
    rke2_dir = Path("/etc/rancher/rke2")

    # Safety check: If we're not in Docker, refuse to run
    if not is_running_in_docker():
        pytest.fail(
            "SAFETY CHECK FAILED: \n"
            "These tests modify /etc/rancher/rke2 and should only run in containers.\n"
            "Run tests using: ./run_tests_docker.sh"
        )

    rke2_dir.mkdir(parents=True, exist_ok=True)
    (rke2_dir / "auth").mkdir(exist_ok=True)

    fixtures = {
        'clean': """# BEGIN ANSIBLE MANAGED BLOCK - tls-san
tls-san:
  - k8s.example.com
# END ANSIBLE MANAGED BLOCK - tls-san
server: https://127.0.0.1:9345
""",
        'multi_san': """# BEGIN ANSIBLE MANAGED BLOCK - tls-san
tls-san:
  - k8s.example.com
  - extra1.example.com
  - 192.168.1.100
# END ANSIBLE MANAGED BLOCK - tls-san
server: https://127.0.0.1:9345
""",
        'no_marker': "server: https://127.0.0.1:9345\n",
        'with_node_labels': """# BEGIN ANSIBLE MANAGED BLOCK - tls-san
tls-san:
  - k8s.example.com
# END ANSIBLE MANAGED BLOCK - tls-san
node-label:
  - node.longhorn.io/create-default-disk=config
  - cluster-bloom/gpu-node=false
server: https://127.0.0.1:9345
""",
    }

    def _setup_fixture(name):
        """Setup a specific fixture state"""
        config_file = rke2_dir / "config.yaml"
        config_file.write_text(fixtures[name])
        return rke2_dir

    return _setup_fixture
