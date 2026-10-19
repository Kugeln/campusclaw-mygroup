"""Phase 1 evidence runner. No database/model calls; only new owned Docker resources.

Run from any directory: python mcp-server/run_phase1.py [--docker]
The Docker option requires engine/build access. Never dumps environment/credentials.
"""
import argparse
import datetime
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--docker", action="store_true")
    args = parser.parse_args()
    module = Path(__file__).resolve().parent
    root = module.parent
    run_id = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + secrets.token_hex(4)
    evidence = root / "runtime" / ("week6-phase1-" + run_id)
    evidence.mkdir(parents=True, exist_ok=False)
    env = dict(os.environ)
    # Discard inherited service/test settings; never use an existing credential.
    for key in list(env):
        if key.startswith("MCP_"):
            del env[key]
    env["GOCACHE"] = str(root / "runtime" / "week6-go-cache")
    env["MCP_TEST_EVIDENCE"] = str(evidence / "host-network.json")
    token = secrets.token_hex(32)
    project = "campusclaw-w6-" + run_id.lower()
    env["MCP_AUTH_TOKEN"] = token
    env["MCP_TEST_PROJECT"] = project
    env["COMPOSE_DISABLE_ENV_FILE"] = "1"
    commands = []
    docker_approved = False
    failures = []
    summary = {"run_id": run_id, "docker_project": project, "commands": commands,
               "failures": failures, "database_operations": 0, "model_calls": 0}

    def run(command, label, expected=0, timeout=180):
        process = subprocess.run(command, cwd=module, env=env, stdout=subprocess.PIPE,
                                 stderr=subprocess.PIPE, timeout=timeout)
        stdout = process.stdout.decode("utf-8", "replace")
        stderr = process.stderr.decode("utf-8", "replace")
        # Redact before any persistence. A leak is still a failed check.
        leaked = token in stdout or token in stderr
        if leaked and label != "compose-config":
            failures.append(label + ": credential appeared in output")
        safe_out = stdout.replace(token, "<redacted-ephemeral-service-credential>")
        safe_err = stderr.replace(token, "<redacted-ephemeral-service-credential>")
        (evidence / (label + ".stdout")).write_text(safe_out, encoding="utf-8")
        (evidence / (label + ".stderr")).write_text(safe_err, encoding="utf-8")
        commands.append({"name": label, "command": command, "exit_code": process.returncode,
                         "expected_exit_code": expected})
        if process.returncode != expected:
            failures.append(label + ": unexpected exit code")
            raise RuntimeError(label + " failed; see redacted evidence")
        return stdout

    compose = ["docker", "compose", "-p", project, "-f", "compose.test.yml"]
    try:
        run(["go", "test", "./...", "-count=1", "-json"], "go-test", timeout=240)
        run(["go", "vet", "./..."], "go-vet")
        if args.docker:
            existing = run(["docker", "ps", "-aq"], "docker-existing").split()
            before = {}
            # Only nonsecret lifecycle fields, so the snapshot never includes Config.Env.
            template = '{{.Id}} {{json .State}}'
            def lifecycle(raw):
                state = json.loads(raw)
                # Health probe timestamps naturally change while we only read.
                return {key: state.get(key) for key in ["Status", "Running", "Paused", "Restarting",
                        "OOMKilled", "Dead", "Pid", "ExitCode", "StartedAt", "FinishedAt"]}
            if existing:
                states = subprocess.check_output(["docker", "inspect", "--format", template] + existing, env=env).decode()
                for line in states.splitlines():
                    container, state = line.split(" ", 1)
                    before[container] = lifecycle(state)
            (evidence / "existing-container-state.json").write_text(json.dumps(before, indent=2), encoding="utf-8")
            network_ids = subprocess.check_output(["docker", "network", "ls", "-q"], env=env).decode().split()
            occupied = []
            networks = json.loads(subprocess.check_output(["docker", "network", "inspect"] + network_ids, env=env)) if network_ids else []
            for network in networks:
                for entry in network.get("IPAM", {}).get("Config", []) or []:
                    if entry.get("Subnet"):
                        occupied.append(ipaddress.ip_network(entry["Subnet"]))
            candidates = [ipaddress.ip_network("10.%d.%d.0/24" % (second, third))
                          for second in range(240, 255) for third in range(1, 255)]
            candidate = next((network for network in candidates
                              if not any(network.version == old.version and network.overlaps(old) for old in occupied)), None)
            assert candidate is not None, "no unused test subnet found; existing networks must be preserved"
            env["MCP_TEST_SUBNET"] = str(candidate)
            summary["docker_test_subnet"] = str(candidate)
            summary["existing_network_subnets"] = [str(network) for network in occupied]
            raw = run(compose + ["config", "--format", "json"], "compose-config")
            config = json.loads(raw)
            assert config["name"] == project
            assert set(config["services"]) == {"mcp-demo", "probe"}
            assert not config.get("volumes")
            assert len(config["networks"]) == 1
            for network in config["networks"].values():
                assert network["internal"] and network["name"].startswith(project + "_")
                assert not network.get("external")
                assert network["ipam"]["config"][0]["subnet"] == str(candidate)
            for service in config["services"].values():
                assert not service.get("ports") and not service.get("volumes")
                assert service["image"].startswith(project + "-")
                assert not service.get("container_name")
            # Refuse collisions before any mutation.
            assert not subprocess.check_output(["docker", "ps", "-aq", "--filter", "label=com.docker.compose.project=" + project], env=env).strip()
            assert not subprocess.check_output(["docker", "network", "ls", "-q", "--filter", "name=" + project], env=env).strip()
            docker_approved = True
            run(compose + ["build", "--pull=false"], "docker-build", timeout=300)
            run(["docker", "image", "inspect", "--format", "{{json .Config}}",
                 project + "-server:phase1", project + "-probe:phase1"], "docker-image-config")
            run(["docker", "history", "--no-trunc", "--format", "{{.CreatedBy}}",
                 project + "-server:phase1"], "docker-image-history")
            run(compose + ["up", "-d", "--wait", "mcp-demo"], "docker-up", timeout=90)
            raw = run(compose + ["run", "--rm", "--no-deps", "probe"], "docker-network", timeout=90)
            results = json.loads(raw)
            assert results and all(row["passed"] for row in results)
            summary["docker_network_checks"] = len(results)
            run(compose + ["logs", "--no-color", "mcp-demo"], "docker-service-logs")
            run(compose + ["stop", "mcp-demo"], "docker-stop")
            raw = run(compose + ["run", "--rm", "--no-deps", "probe", "offline"], "docker-offline")
            assert json.loads(raw)["passed"]
            summary["docker_stopped_service_unavailable"] = True
            if before:
                states = subprocess.check_output(["docker", "inspect", "--format", template] + list(before), env=env).decode()
                for line in states.splitlines():
                    container, raw_state = line.split(" ", 1)
                    assert lifecycle(raw_state) == before[container], "existing container lifecycle changed"
            summary["existing_containers_unchanged"] = len(before)
        else:
            summary["docker_status"] = "not requested / not executed"
    except Exception as error:
        failures.append(type(error).__name__ + ": " + str(error).replace(token, "<redacted>"))
    finally:
        if docker_approved:
            try:
                # The project was created in this invocation after proving no collision.
                run(compose + ["down", "--remove-orphans"], "docker-cleanup")
                assert not subprocess.check_output(["docker", "ps", "-aq", "--filter",
                    "label=com.docker.compose.project=" + project], env=env).strip()
                assert not subprocess.check_output(["docker", "network", "ls", "-q", "--filter",
                    "label=com.docker.compose.project=" + project], env=env).strip()
                summary["owned_containers_networks_removed"] = True
            except Exception as error:
                failures.append("owned project cleanup failed: " + type(error).__name__)
        host_path = evidence / "host-network.json"
        if host_path.exists():
            rows = json.loads(host_path.read_text(encoding="utf-8"))
            summary["host_network_checks"] = len(rows)
            summary["host_network_failed"] = sum(not row["passed"] for row in rows)
        summary["passed"] = not failures
        summary["source_sha256"] = {str(path.relative_to(module)): hashlib.sha256(path.read_bytes()).hexdigest()
                                    for path in module.rglob("*") if path.is_file() and "__pycache__" not in path.parts}
        (evidence / "result.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        print(json.dumps({"evidence": str(evidence), "passed": summary["passed"], "failures": failures}))
    return 0 if not failures else 1


if __name__ == "__main__":
    sys.exit(main())
