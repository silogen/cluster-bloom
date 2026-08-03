#!/usr/bin/python
# -*- coding: utf-8 -*-
# Mock systemd module for testing
# Returns success without actually operating on systemd services

from ansible.module_utils.basic import AnsibleModule


def main():
    module = AnsibleModule(
        argument_spec=dict(
            name=dict(type='str', required=True),
            state=dict(
                type='str',
                choices=['started', 'stopped', 'restarted', 'reloaded']
            ),
            enabled=dict(type='bool'),
            daemon_reload=dict(type='bool', default=False),
            masked=dict(type='bool'),
            no_block=dict(type='bool', default=False),
        ),
        supports_check_mode=True
    )

    service_name = module.params['name']
    state = module.params.get('state')

    # Determine if this would cause a change
    changed = state in ['started', 'stopped', 'restarted', 'reloaded']

    result = {
        'changed': changed,
        'name': service_name,
        'state': state or 'unknown',
        'status': {
            'ActiveState': 'active',
            'SubState': 'running',
            'LoadState': 'loaded',
        },
        'msg': f"MOCK systemd: Would {state or 'manage'} service '{service_name}'"
    }

    module.exit_json(**result)


if __name__ == '__main__':
    main()
