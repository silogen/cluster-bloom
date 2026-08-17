"""Tests for the first-node Cilium readiness gate (cluster_ready.yaml)

The gate exists so that bloom's first pod-creating tasks do not run while the
Cilium agent is still initializing, which produced pod sandboxes whose datapath
was never programmed. What matters here is that the operator check keeps
retrying rather than passing on its first look.
"""


def test_gate_passes_when_cilium_is_ready(ansible_runner_factory):
    result = ansible_runner_factory('tests/ansible/playbooks/test_cluster_ready.yaml')

    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"


def test_gate_retries_until_an_operator_is_ready(
    cilium_operator_not_ready, ansible_runner_factory
):
    """The operator check must loop, not accept the first answer

    On CLUSTER_SIZE large there are 2 operator replicas with hard anti-affinity
    and a single node, so one replica is Pending by design. The gate has to wait
    for at least one to go Ready instead of giving up or hanging on all of them.
    """
    cilium_operator_not_ready(2)

    result = ansible_runner_factory('tests/ansible/playbooks/test_cluster_ready.yaml')

    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    events = list(result.events)
    operator_task = [
        e for e in events
        if e.get('event_data', {}).get('task') == 'Wait for at least one cilium-operator to be Ready'
        and e.get('event') == 'runner_on_ok'
    ]
    assert operator_task, "Operator readiness task did not complete"

    attempts = operator_task[-1]['event_data']['res'].get('attempts')
    assert attempts == 3, f"Expected 3 attempts (2 not-ready, then Ready), got {attempts}"


def test_gate_waits_for_this_nodes_cilium_agent(
    cilium_agent_not_ready, ansible_runner_factory
):
    """The first node is gated on its own agent, not on fleet-wide state

    Fleet-wide waits (kubectl wait node --all, DaemonSet rollout status) would
    fail every first-node run whenever an unrelated node is drained or down.
    """
    cilium_agent_not_ready(2)

    result = ansible_runner_factory('tests/ansible/playbooks/test_cluster_ready.yaml')

    assert result.rc == 0, f"Playbook failed:\n{result.stdout}"

    agent_task = [
        e for e in result.events
        if e.get('event_data', {}).get('task') == 'Wait for the Cilium agent to report healthy on this node'
        and e.get('event') == 'runner_on_ok'
    ]
    assert agent_task, "Agent readiness task did not run — is it still included?"

    attempts = agent_task[-1]['event_data']['res'].get('attempts')
    assert attempts == 3, f"Expected 3 attempts (2 refused, then healthy), got {attempts}"
