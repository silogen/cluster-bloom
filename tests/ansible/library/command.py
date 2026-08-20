#!/usr/bin/python
# -*- coding: utf-8 -*-
# Mock command module for testing
# Detects kubectl commands and returns canned responses

from ansible.module_utils.basic import AnsibleModule

# Each module invocation is a fresh process, so a test that wants the
# cilium-operator readiness check to fail before it succeeds has to leave the
# remaining-failure count on disk. Absent the file the operator is Ready, which
# keeps every other test unaffected.
CILIUM_OPERATOR_STATE = '/tmp/mock_cilium_operator_not_ready'


def cilium_operator_response():
    try:
        with open(CILIUM_OPERATOR_STATE) as f:
            remaining = int(f.read().strip() or 0)
    except (IOError, OSError, ValueError):
        remaining = 0

    if remaining <= 0:
        return 0, 'True'

    with open(CILIUM_OPERATOR_STATE, 'w') as f:
        f.write(str(remaining - 1))
    # `grep -qx True` prints nothing and exits 1 when no operator is Ready.
    return 1, ''


def main():
    module = AnsibleModule(
        argument_spec=dict(
            _raw_params=dict(type='str'),
            _uses_shell=dict(type='bool', default=False),
            argv=dict(type='list'),
            chdir=dict(type='path'),
            creates=dict(type='path'),
            removes=dict(type='path'),
            stdin=dict(type='str'),
            # `shell:` tasks are dispatched to this module, so it has to accept
            # every argument the shell action plugin forwards.
            executable=dict(type='str'),
        ),
        supports_check_mode=True
    )

    # Get the command being run
    if module.params.get('_raw_params'):
        cmd = module.params['_raw_params']
    elif module.params.get('argv'):
        cmd = ' '.join(module.params['argv'])
    else:
        module.fail_json(msg="No command specified")

    # If _uses_shell is True, this is actually a shell command
    # Run it through subprocess with shell=True (except kubectl)
    if module.params.get('_uses_shell') and 'kubectl' not in cmd:
        import subprocess
        try:
            proc = subprocess.run(
                cmd,
                shell=True,
                cwd=module.params.get('chdir'),
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                timeout=30
            )
            result = {
                'changed': True,
                'rc': proc.returncode,
                'stdout': proc.stdout,
                'stderr': proc.stderr,
                'cmd': cmd,
            }
            if proc.returncode != 0:
                module.fail_json(msg=f"Shell command failed", **result)
            else:
                module.exit_json(**result)
        except Exception as e:
            module.fail_json(msg=f"Shell command failed: {str(e)}", rc=1)

    # Detect kubectl commands and return appropriate mock responses
    if 'kubectl' in cmd:
        if 'cluster-info' in cmd:
            result = {
                'changed': False,
                'rc': 0,
                'stdout': 'Kubernetes control plane is running at https://127.0.0.1:6443\nCoreNS is running at https://127.0.0.1:6443/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy',
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: cluster-info'
            }
        elif 'name=cilium-operator' in cmd:
            rc, stdout = cilium_operator_response()
            result = {
                'changed': False,
                'rc': rc,
                'stdout': stdout,
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: cilium-operator readiness'
            }
        elif '--raw=/readyz' in cmd:
            result = {
                'changed': False,
                'rc': 0,
                'stdout': 'ok',
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: apiserver readyz'
            }
        elif 'wait' in cmd and 'node' in cmd:
            result = {
                'changed': False,
                'rc': 0,
                'stdout': 'node/test-node condition met\nnode/test-node-2 condition met',
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: wait for nodes'
            }
        elif 'get configmap' in cmd and 'cluster-domain' in cmd:
            # Return JSON for cluster-domain ConfigMap
            result = {
                'changed': False,
                'rc': 0,
                'stdout': '{"data":{"DOMAIN":"test.example.com","use-cert-manager":"false"}}',
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: get configmap cluster-domain'
            }
        elif 'get deployment' in cmd:
            # Return ready replicas for deployment checks
            result = {
                'changed': False,
                'rc': 0,
                'stdout': '1',
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: get deployment replicas'
            }
        elif 'get statefulset' in cmd:
            # Return ready replicas for statefulset checks
            result = {
                'changed': False,
                'rc': 0,
                'stdout': '1',
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: get statefulset replicas'
            }
        elif 'get applications' in cmd:
            # Return empty for ArgoCD application checks
            result = {
                'changed': False,
                'rc': 0,
                'stdout': '',
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: get applications'
            }
        else:
            # Generic kubectl success
            result = {
                'changed': False,
                'rc': 0,
                'stdout': 'MOCK kubectl: command succeeded',
                'stderr': '',
                'cmd': cmd,
                'msg': 'MOCK kubectl: generic command'
            }
    else:
        # Non-kubectl command: just succeed
        result = {
            'changed': True,
            'rc': 0,
            'stdout': f'MOCK command: Executed {cmd}',
            'stderr': '',
            'cmd': cmd,
        }

    module.exit_json(**result)


if __name__ == '__main__':
    main()
