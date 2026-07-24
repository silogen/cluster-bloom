#!/usr/bin/python
# -*- coding: utf-8 -*-
# Mock command module for testing
# Detects kubectl commands and returns canned responses

from ansible.module_utils.basic import AnsibleModule


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
