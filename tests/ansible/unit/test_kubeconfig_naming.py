"""Unit tests for naming the kubeconfig cluster after DOMAIN

Runs the real deploy_cluster/kubeconfig_cluster_name.yaml tasks against a
fixture kubeconfig shaped like the one RKE2 writes.
"""
import getpass

import pytest
import yaml

DOMAIN = 'cluster.example.com'

# Shape of /etc/rancher/rke2/rke2.yaml as written by RKE2, after bloom has
# already rewritten the server address.
RKE2_KUBECONFIG = """apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tdefaultCg==
    server: https://10.0.0.5:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
kind: Config
preferences: {}
users:
- name: default
  user:
    client-certificate-data: ZGVmYXVsdA==
    client-key-data: LS0tCg==
"""


@pytest.fixture
def named_kubeconfig(tmp_path, ansible_runner_factory):
    kubeconfig = tmp_path / 'config'
    kubeconfig.write_text(RKE2_KUBECONFIG)

    result = ansible_runner_factory(
        'tests/ansible/playbooks/test_kubeconfig_naming.yaml',
        extravars={
            'DOMAIN': DOMAIN,
            'kubeconfig_path': str(kubeconfig),
            'kubeconfig_owner': getpass.getuser(),
        },
    )
    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    return yaml.safe_load(kubeconfig.read_text())


def test_cluster_is_named_after_domain(named_kubeconfig):
    assert named_kubeconfig['clusters'][0]['name'] == DOMAIN
    # The context must follow the cluster rename or the kubeconfig is broken
    assert named_kubeconfig['contexts'][0]['context']['cluster'] == DOMAIN


def test_context_and_user_keep_default_name(named_kubeconfig):
    assert named_kubeconfig['contexts'][0]['name'] == 'default'
    assert named_kubeconfig['contexts'][0]['context']['user'] == 'default'
    assert named_kubeconfig['current-context'] == 'default'
    assert named_kubeconfig['users'][0]['name'] == 'default'


def test_credentials_and_server_survive_the_rewrite(named_kubeconfig):
    cluster = named_kubeconfig['clusters'][0]['cluster']

    assert cluster['server'] == 'https://10.0.0.5:6443'
    assert cluster['certificate-authority-data'] == 'LS0tdefaultCg=='
    assert named_kubeconfig['users'][0]['user'] == {
        'client-certificate-data': 'ZGVmYXVsdA==',
        'client-key-data': 'LS0tCg==',
    }


def test_kubeconfig_stays_usable(named_kubeconfig):
    """The context must resolve to a cluster and a user that exist"""
    context = named_kubeconfig['contexts'][0]['context']

    assert context['cluster'] in [c['name'] for c in named_kubeconfig['clusters']]
    assert context['user'] in [u['name'] for u in named_kubeconfig['users']]
    assert named_kubeconfig['current-context'] in [
        c['name'] for c in named_kubeconfig['contexts']
    ]
