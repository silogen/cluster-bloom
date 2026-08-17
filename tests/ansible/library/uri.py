#!/usr/bin/python
# -*- coding: utf-8 -*-
# Mock uri module for testing
# There is no Cilium agent in the test container, so the real module would
# retry the agent healthz for its full budget. Reports healthy by default;
# write an attempt count to the state file to make it fail that many times
# first (see conftest.cilium_agent_not_ready).

from ansible.module_utils.basic import AnsibleModule

CILIUM_AGENT_STATE = '/tmp/mock_cilium_agent_not_ready'


def healthz_status():
    try:
        with open(CILIUM_AGENT_STATE) as f:
            remaining = int(f.read().strip() or 0)
    except (IOError, OSError, ValueError):
        remaining = 0

    if remaining <= 0:
        return 200

    with open(CILIUM_AGENT_STATE, 'w') as f:
        f.write(str(remaining - 1))
    # Connection refused while the agent is still starting.
    return -1


def main():
    module = AnsibleModule(
        argument_spec=dict(
            url=dict(type='str', required=True),
            headers=dict(type='dict', default={}),
            status_code=dict(type='list', default=[200]),
            use_proxy=dict(type='bool', default=True),
            timeout=dict(type='int', default=30),
            method=dict(type='str', default='GET'),
            validate_certs=dict(type='bool', default=True),
        ),
        supports_check_mode=True
    )

    url = module.params['url']

    if '9879/healthz' not in url:
        module.fail_json(msg=f"MOCK uri: unhandled url {url}", status=-1)

    status = healthz_status()
    result = {
        'changed': False,
        'status': status,
        'url': url,
        'msg': 'MOCK uri: cilium agent healthz',
    }

    if status != 200:
        module.fail_json(msg='MOCK uri: connection refused', **result)

    module.exit_json(**result)


if __name__ == '__main__':
    main()
