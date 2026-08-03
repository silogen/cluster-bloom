#!/usr/bin/python
# -*- coding: utf-8 -*-
# Mock shell module for testing
# Runs real commands EXCEPT kubectl (which gets mocked)

import subprocess
from ansible.module_utils.basic import AnsibleModule


def main():
    module = AnsibleModule(
        argument_spec=dict(
            _raw_params=dict(type='str'),
            _uses_shell=dict(type='bool', default=True),
            chdir=dict(type='path'),
            creates=dict(type='path'),
            removes=dict(type='path'),
            stdin=dict(type='str'),
            executable=dict(type='str'),
        ),
        supports_check_mode=True
    )

    cmd = module.params.get('_raw_params', '')

    if not cmd:
        module.fail_json(msg="No command specified")

    # If command contains kubectl, mock it
    if 'kubectl' in cmd:
        if 'cluster-info' in cmd:
            stdout = 'Kubernetes control plane is running at https://127.0.0.1:6443'
        elif 'get configmap' in cmd and 'cluster-domain' in cmd:
            stdout = '{"data":{"DOMAIN":"test.example.com"}}'
        else:
            stdout = 'MOCK kubectl command'

        result = {
            'changed': False,
            'rc': 0,
            'stdout': stdout,
            'stderr': '',
            'cmd': cmd,
            'msg': 'MOCK shell: kubectl command'
        }
        module.exit_json(**result)

    # Otherwise, run the real command (for sed/grep/etc)
    try:
        # Check mode: don't actually run
        if module.check_mode:
            result = {
                'changed': True,
                'rc': 0,
                'stdout': f'CHECK MODE: Would run {cmd}',
                'stderr': '',
                'cmd': cmd,
            }
            module.exit_json(**result)

        proc = subprocess.run(
            cmd,
            shell=True,
            cwd=module.params.get('chdir'),
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=30,
            input=module.params.get('stdin'),
            executable=module.params.get('executable') or '/bin/sh'
        )

        result = {
            'changed': True,
            'rc': proc.returncode,
            'stdout': proc.stdout,
            'stderr': proc.stderr,
            'cmd': cmd,
        }

        if proc.returncode != 0:
            module.fail_json(msg=f"Command failed with rc={proc.returncode}", **result)
        else:
            module.exit_json(**result)

    except subprocess.TimeoutExpired:
        module.fail_json(msg=f"Command timed out after 30s: {cmd}", rc=124)
    except Exception as e:
        module.fail_json(msg=f"Command execution failed: {str(e)}", rc=1)


if __name__ == '__main__':
    main()
