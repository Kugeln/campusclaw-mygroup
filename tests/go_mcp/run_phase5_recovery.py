"""Phase 5 transport recovery only; a fresh random project, no repeated broad suites."""
import base64
import datetime
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import time
import traceback
import urllib.request


def main():
    root=Path(__file__).resolve().parents[2]
    run_id=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ-')+secrets.token_hex(4)
    evidence=root/'runtime'/('week6-phase5-recovery-'+run_id); evidence.mkdir(parents=True,exist_ok=False)
    project='campusclaw-w6-p5-'+run_id.lower(); fresh=project+'-fresh'
    password,token,browser_password=secrets.token_hex(32),secrets.token_hex(32),secrets.token_hex(32)
    master=base64.b64encode(secrets.token_bytes(32)).decode(); signing=secrets.token_hex(32)
    secret_values=(password,token,browser_password,master,signing)
    env=dict(os.environ); env.update(PYTHONIOENCODING='utf-8',COMPOSE_DISABLE_ENV_FILE='1',MCP_TEST_PROJECT=project,MCP_TEST_MYSQL=project+'-mysql',
        MCP_TEST_PASSWORD=password,MCP_AUTH_TOKEN=token,MCP_TEST_EVIDENCE_HOST=str(evidence),W6_BROWSER_PASSWORD=browser_password,
        MCP_TEST_LEGACY_ALIAS='campusclaw-align-storage-'+secrets.token_hex(4),MCP_TEST_QDRANT='campusclaw-w6-p5-qdrant-'+secrets.token_hex(4))
    env['MCP_TEST_LEGACY_DSN']='root:'+password+'@tcp('+env['MCP_TEST_LEGACY_ALIAS']+':3306)/?charset=utf8mb4'
    commands,failures=[],[]
    report=dict(run_id=run_id,project=project,fresh_project=fresh,commands=commands,failures=failures,paid_model_calls=0,existing_database_connections=0)
    compose=['docker','compose','-p',project,'-f','tests/go_mcp/compose.phase5.yml']
    fresh_compose=['docker','compose','-p',fresh,'-f','docker-compose.yml','-f',str(evidence/'fresh.override.json'),'--profile','rag','--profile','mcp']
    def clean(text):
        for value in secret_values: text=text.replace(value,'<redacted>')
        return text
    def persist(command,label,process):
        out,err=process.stdout.decode('utf-8','replace'),process.stderr.decode('utf-8','replace')
        if label not in ('config','fresh-config') and any(v in out or v in err for v in secret_values): failures.append(label+': credential output detected')
        (evidence/(label+'.stdout')).write_text(clean(out),encoding='utf-8'); (evidence/(label+'.stderr')).write_text(clean(err),encoding='utf-8')
        commands.append(dict(name=label,command=command,exit_code=process.returncode)); print(label+': '+str(process.returncode),flush=True)
        if process.returncode: raise RuntimeError(label+' failed')
        return out
    def run(command,label,timeout=300):
        try: p=subprocess.run(command,cwd=root,env=env,capture_output=True,timeout=timeout)
        except subprocess.TimeoutExpired as e: p=subprocess.CompletedProcess(command,124,e.stdout or b'',e.stderr or b'')
        return persist(command,label,p)
    def output(command): return subprocess.check_output(command,cwd=root,env=env).decode().strip()
    owned=[]
    before_ids=output(['docker','ps','-aq']).split()
    template='{{.Id}} {{.State.Status}} {{.State.StartedAt}} {{.State.FinishedAt}}'
    before=output(['docker','inspect','--format',template]+before_ids) if before_ids else ''
    (evidence/'existing-containers-before.txt').write_text(before,encoding='utf-8')
    old_networks=set(output(['docker','network','ls','-q']).split()); old_volumes=set(output(['docker','volume','ls','-q']).split())
    source_files=[f for d in ('cmd','internal','mcp-server','tests/go_mcp') for f in (root/d).rglob('*') if f.is_file() and '__pycache__' not in f.parts]
    report['source_sha256']={f.relative_to(root).as_posix():hashlib.sha256(f.read_bytes()).hexdigest() for f in source_files}
    try:
        networks=json.loads(output(['docker','network','inspect']+list(old_networks))) if old_networks else []
        occupied=[ipaddress.ip_network(c['Subnet']) for n in networks for c in (n.get('IPAM',{}).get('Config') or []) if c.get('Subnet')]
        free=[]
        for i in range(1,255):
            candidate=ipaddress.ip_network(f'10.243.{i}.0/24')
            if not any(n.version==4 and n.overlaps(candidate) for n in occupied): free.append(str(candidate))
            if len(free)==6: break
        assert len(free)==6
        env['MCP_TEST_SUBNET']=free[0]
        env['MCP_TEST_FRONTEND_SUBNET']=free[1]
        env['MCP_ALLOWED_TARGETS']=json.dumps([dict(scheme='http',host='mcp-demo',port=8090,cidrs=[free[0]])])
        config=json.loads(run(compose+['config','--format','json'],'config'))
        assert set(config['services'])=={'mysql','qdrant','mcp-demo','app','nginx'}
        assert not config.get('volumes') and config['networks']['isolated']['internal']
        assert all(not s.get('ports') for name,s in config['services'].items() if name!='nginx')
        assert not output(['docker','ps','-aq','--filter','label=com.docker.compose.project='+project])
        owned.append((project,compose)); run(compose+['build','--pull=false'],'build',600)
        run(compose+['up','-d','--wait'],'up',240)
        for _ in range(100):
            if (evidence/'ready.json').exists(): break
            time.sleep(.1)
        assert (evidence/'ready.json').exists(), 'ready file missing'
        address=output(compose+['port','nginx','8080']); report['browser_address']=address
        assert address.startswith('127.0.0.1:'), 'unexpected published address '+address
        env.update(W6_BROWSER_ORIGIN='http://'+address,W6_BROWSER_RESULT=str(evidence/'browser-result.json'))
        command=[str(root/'.venv/Scripts/python.exe'),'tests/go_mcp/check_phase5_recovery.py']
        # Spool output to files, avoiding a pipe deadlock while waiting for the
        # browser's requests to stop/start only this run's MCP container.
        with (evidence/'browser.raw-out').open('wb') as stdout,(evidence/'browser.raw-err').open('wb') as stderr:
            process=subprocess.Popen(command,cwd=root,env=env,stdout=stdout,stderr=stderr)
            deadline=time.monotonic()+600
            stopped=started=False
            while process.poll() is None:
                if (evidence/'request-mcp-stop').exists() and not stopped:
                    run(compose+['stop','mcp-demo'],'stop-owned-mcp',60); (evidence/'mcp-stopped').touch(); stopped=True
                if (evidence/'request-mcp-start').exists() and not started:
                    run(compose+['up','-d','--wait','mcp-demo'],'restart-owned-mcp',60); (evidence/'mcp-started').touch(); started=True
                if time.monotonic()>deadline: process.kill(); break
                time.sleep(.15)
            process.wait()
        persist(command,'browser',subprocess.CompletedProcess(command,process.returncode,(evidence/'browser.raw-out').read_bytes(),(evidence/'browser.raw-err').read_bytes()))
        # Remove only raw spool files created by this process after redacted copies
        # have been saved. Failure evidence remains in browser.stdout/stderr.
        (evidence/'browser.raw-out').unlink(); (evidence/'browser.raw-err').unlink()
        (evidence/'stop').touch(); run(compose+['wait','app'],'fixture-exit',60)
        assert output(['docker','inspect','--format','{{.State.ExitCode}}',output(compose+['ps','-aq','app'])])=='0'
    except Exception as e:
        failures.append(clean(type(e).__name__+': '+str(e)))
        (evidence/'exception.txt').write_text(clean(traceback.format_exc()),encoding='utf-8')
    finally:
        for name,command in reversed(owned):
            try:
                run(command+['logs','--no-color'],name+'-logs',60)
                run(command+['down','--volumes','--remove-orphans'],name+'-cleanup',90)
                assert not output(['docker','ps','-aq','--filter','label=com.docker.compose.project='+name])
                assert not output(['docker','network','ls','-q','--filter','label=com.docker.compose.project='+name])
                assert not output(['docker','volume','ls','-q','--filter','label=com.docker.compose.project='+name])
            except Exception as e: failures.append('cleanup '+name+': '+clean(str(e)))
        after=output(['docker','inspect','--format',template]+before_ids) if before_ids else ''
        (evidence/'existing-containers-after.txt').write_text(after,encoding='utf-8')
        if before!=after: failures.append('existing container lifecycle changed')
        if set(output(['docker','network','ls','-q']).split())!=old_networks: failures.append('network inventory differs')
        if set(output(['docker','volume','ls','-q']).split())!=old_volumes: failures.append('volume inventory differs')
        report['existing_containers_unchanged']=len(before_ids) if before==after else False
        report['owned_resources_removed']=all(not output(['docker','ps','-aq','--filter','label=com.docker.compose.project='+name]) for name,_ in owned)
        for path,sha in report['source_sha256'].items():
            if hashlib.sha256((root/path).read_bytes()).hexdigest()!=sha: failures.append('source changed during run: '+path)
        # Never leave an unredacted spool even after a failed browser command.
        for path in (evidence/'browser.raw-out',evidence/'browser.raw-err'):
            if path.exists(): path.write_text(clean(path.read_text(encoding='utf-8',errors='replace')),encoding='utf-8')
        report['passed']=not failures
        (evidence/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
        print(json.dumps(dict(evidence=str(evidence),passed=report['passed'],failures=failures)),flush=True)
    return 0 if not failures else 1


if __name__=='__main__':
    sys.exit(main())
