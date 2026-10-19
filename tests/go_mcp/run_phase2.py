"""Isolated Phase 2 runner: no existing database, container, volume or model access."""
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
    root = Path(__file__).resolve().parents[2]
    run_id = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + secrets.token_hex(4)
    evidence = root / "runtime" / ("week6-phase2-" + run_id)
    evidence.mkdir(parents=True, exist_ok=False)
    project = "campusclaw-w6-p2-" + run_id.lower()
    env = dict(os.environ)
    password, token = secrets.token_hex(32), secrets.token_hex(32)
    env.update(MCP_TEST_PROJECT=project, MCP_TEST_MYSQL=project+"-mysql", MCP_TEST_PASSWORD=password,
               MCP_AUTH_TOKEN=token, MCP_TEST_EVIDENCE_HOST=str(evidence), COMPOSE_DISABLE_ENV_FILE="1",
               GOCACHE=str(root / "runtime/week6-go-cache"))
    env["MCP_TEST_DSN"] = "root:"+password+"@tcp("+env["MCP_TEST_MYSQL"]+":3306)/?charset=utf8mb4"
    commands, failures = [], []
    report = {"run_id": run_id, "project": project, "commands": commands, "failures": failures,
              "paid_model_calls": 0, "existing_database_connections": 0}
    compose = ["docker", "compose", "-p", project, "-f", "tests/go_mcp/compose.test.yml"]
    def run(command, label, timeout=300):
        process = subprocess.run(command, cwd=root, env=env, capture_output=True, timeout=timeout)
        out, err = process.stdout.decode("utf-8", "replace"), process.stderr.decode("utf-8", "replace")
        for secret in (password, token):
            if label != "config" and (secret in out or secret in err):
                failures.append(label+": credential in output")
            out, err = out.replace(secret, "<redacted>"), err.replace(secret, "<redacted>")
        (evidence/(label+".stdout")).write_text(out, encoding="utf-8")
        (evidence/(label+".stderr")).write_text(err, encoding="utf-8")
        commands.append({"name":label,"command":command,"exit_code":process.returncode})
        print(label+": "+str(process.returncode), flush=True)
        if process.returncode:
            raise RuntimeError(label+" failed")
        return out
    owned = False
    try:
        ids = subprocess.check_output(["docker", "ps", "-aq"]).decode().split()
        template = '{{.Id}} {{.State.Status}} {{.State.StartedAt}} {{.State.FinishedAt}}'
        before = subprocess.check_output(["docker", "inspect", "--format", template]+ids).decode() if ids else ""
        (evidence/"existing-containers-before.txt").write_text(before,encoding="utf-8")
        network_ids = subprocess.check_output(["docker","network","ls","-q"]).decode().split()
        networks = json.loads(subprocess.check_output(["docker","network","inspect"]+network_ids)) if network_ids else []
        occupied = [ipaddress.ip_network(item["Subnet"]) for n in networks for item in (n.get("IPAM",{}).get("Config") or []) if item.get("Subnet")]
        subnet = next(ipaddress.ip_network("10.241.%d.0/24"%i) for i in range(1,255) if not any(p.version==4 and ipaddress.ip_network("10.241.%d.0/24"%i).overlaps(p) for p in occupied))
        env["MCP_TEST_SUBNET"] = str(subnet)
        env["MCP_ALLOWED_TARGETS"] = json.dumps([{"scheme":"http","host":"mcp-demo","port":8090,"cidrs":[str(subnet)]}])
        config = json.loads(run(compose+["config","--format","json"],"config"))
        assert config["name"] == project and set(config["services"]) == {"mysql","mcp-demo","tests"}
        assert not config.get("volumes")
        assert all(not service.get("ports") for service in config["services"].values())
        assert all(network["internal"] and network["name"].startswith(project+"_") and not network.get("external") for network in config["networks"].values())
        assert not subprocess.check_output(["docker","ps","-aq","--filter","label=com.docker.compose.project="+project]).strip()
        assert not subprocess.check_output(["docker","network","ls","-q","--filter","name="+project]).strip()
        owned = True
        run(compose+["build","--pull=false"],"build",600)
        run(compose+["up","-d","--wait","mysql","mcp-demo"],"up",180)
        testbase = compose+["run","--rm","--no-deps","tests"]
        run(compose+["run","--rm","--no-deps","-e","MCP_CAPTURE=true","tests","go","test","-tags","mcpstorage","./internal/repository/mysql","-run","^TestMCPCapture$","-v","-count=1"],"capture",300)
        captured=json.loads((evidence/"schema_mcp_steps.json").read_text(encoding="utf-8"))
        statements=[s.strip() for s in (root/"internal/repository/mysql/schema_mcp_v4.sql").read_text(encoding="utf-8").split(";") if s.strip()]
        assert len(captured)==len(statements)==4
        assert all(step["sha256"]==hashlib.sha256(sql.encode()).hexdigest() for step,sql in zip(captured,statements))
        artifact=root/"internal/repository/mysql/schema_mcp_steps.json"
        prior=json.loads(artifact.read_text(encoding="utf-8"))
        if prior:
            assert prior==captured, "approved catalog differs; refuse replacement"
        else:
            artifact.write_text(json.dumps(captured,indent=2)+"\n",encoding="utf-8")
        report["catalog_reference_capture"]={"steps":len(captured),"source":"new isolated reference database, not a migration target","sha256":hashlib.sha256(artifact.read_bytes()).hexdigest()}
        run(testbase+["go","test","-tags","mcpstorage","./internal/mcp","./internal/repository/mysql","./internal/transfer","-run","Test(Vault|Policy|MCP(Migration|Interrupted|Drift|Store|Backup|Client|Lock|SchemaReceipt))","-count=1","-json","-timeout","12m"],"integration",900)
        run(testbase+["go","test","./cmd/...","./internal/...","-count=1","-json","-timeout","5m"],"compatibility",420)
        run(testbase+["go","vet","./cmd/...","./internal/..."],"vet",300)
        after = subprocess.check_output(["docker","inspect","--format",template]+ids).decode() if ids else ""
        assert before == after, "existing container lifecycle changed"
        report["existing_containers_unchanged"]=len(ids)
        report["subnet"]=str(subnet)
    except Exception as error:
        failures.append(type(error).__name__+": "+str(error))
    finally:
        if owned:
            try:
                run(compose+["logs","--no-color","mysql","mcp-demo"],"service-logs",60)
                run(compose+["down","--remove-orphans"],"cleanup",90)
                assert not subprocess.check_output(["docker","ps","-aq","--filter","label=com.docker.compose.project="+project]).strip()
                assert not subprocess.check_output(["docker","network","ls","-q","--filter","label=com.docker.compose.project="+project]).strip()
                report["owned_resources_removed"]=True
            except Exception as error:
                failures.append("cleanup: "+str(error))
        report["passed"]=not failures
        (evidence/"result.json").write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding="utf-8")
        print(json.dumps({"evidence":str(evidence),"passed":report["passed"],"failures":failures}),flush=True)
    return 0 if not failures else 1

if __name__=="__main__":
    sys.exit(main())
