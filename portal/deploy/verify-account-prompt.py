"""Run opt-in Portal adapter smoke tests against the selected live accounts.

Uses the existing Core admin key via its official API. Does not mint sessions,
change Core data, or bypass step-up rejection. Secrets only travel over stdin.
The production request path instead uses the signed-in administrator's JWT.
"""
import json
import subprocess


def inspect(container):
    return json.loads(subprocess.check_output(["docker", "inspect", container], text=True))[0]


portal = inspect("portal-portal-api-1")
core = inspect("sub2api")
env = dict(item.split("=", 1) for item in portal["Config"]["Env"] if "=" in item)
key = env.get("CORE_ADMIN_API_KEY", "")
assert key, "Existing Core administrator key is not configured"
networks = set(portal["NetworkSettings"]["Networks"]) & set(core["NetworkSettings"]["Networks"])
assert networks, "Portal and Core have no shared network"
subprocess.run([
    "docker", "run", "--rm", "-i", "--network", sorted(networks)[0],
    "--env", "PORTAL_ACCOUNT_PROMPT_LIVE=1", "portal-account-prompt-validation:20261005",
], input=json.dumps({"BaseURL": "http://sub2api:8080", "AdminKey": key}), text=True, check=True)
