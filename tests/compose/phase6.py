"""Only missing 7.1 evidence: proxy health faults and multi-device revocation.
Creates a new default-Compose project; never accepts existing targets or deletes volumes.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import socket
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'tests/http_contract'))
from adapters import HTTPAdapter


def main():
    name = 'campusclaw-phase6-' + secrets.token_hex(4)
    runtime = ROOT / 'runtime' / name
    runtime.mkdir(parents=True, exist_ok=False)
    empty = runtime / 'empty.env'; empty.write_text('# empty\n')
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0)); port = sock.getsockname()[1]
    assert port != 8080
    env = os.environ.copy(); password = secrets.token_hex(24)
    env.update(DB_PASSWORD=secrets.token_hex(24), MYSQL_ROOT_PASSWORD=secrets.token_hex(24),
               JWT_SIGNING_SECRET=secrets.token_hex(32), INITIALIZATION_MODE='fresh',
               DEV_HTTP_PORT=str(port), GO_IMAGE=name+':server', GO_INIT_IMAGE=name+':init')
    for key in ('TEACHER_A','STUDENT_A1','TEACHER_B','STUDENT_B1'):
        env['SEED_'+key+'_PASSWORD'] = password
    secret_values = [password, env['DB_PASSWORD'], env['MYSQL_ROOT_PASSWORD'], env['JWT_SIGNING_SECRET']]
    base = ['docker','compose','--env-file',str(empty),'-p',name,'-f',str(ROOT/'docker-compose.yml')]
    report = {'project':name,'port':port,'accepted':False,'checks':[]}
    target = HTTPAdapter('Go',f'http://127.0.0.1:{port}',env['JWT_SIGNING_SECRET'],password,runtime,None)

    def redact(value):
        for secret in secret_values:value=value.replace(secret,'[redacted]')
        return re.sub(r'eyJ[\w-]+\.[\w-]+\.[\w-]+','[redacted JWT]',value)

    def dc(*args, check=True):
        proc = subprocess.Popen(base+list(args),cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,encoding='utf-8',errors='replace')
        begin=time.monotonic()
        while True:
            try:out,err=proc.communicate(timeout=15);break
            except subprocess.TimeoutExpired:
                print('Isolated phase6: '+' '.join(args[:3]),flush=True)
                if time.monotonic()-begin>480:
                    proc.kill();proc.communicate();raise RuntimeError('command timeout')
        with (runtime/'commands.log').open('a',encoding='utf-8') as log:
            log.write(redact(json.dumps(list(args))+'\n'+out+err+'\nexit='+str(proc.returncode)+'\n'))
        if check and proc.returncode:raise RuntimeError(redact('Compose failed: '+err[-1500:]))
        return out

    def health(status):
        r=target.request('GET','/health')
        assert r.status==status and r.json()=={'status':'ok' if status==200 else 'unhealthy'}, 'health status/body mismatch'
        assert r.headers.get('content-type')=='application/json' and 'set-cookie' not in r.headers
    def step(value):report['checks'].append(value);print('PASS '+value,flush=True)
    changed_permissions=False
    try:
        dc('build','app','initialize')
        dc('up','-d','--wait','--wait-timeout','180')
        health(200)
        first=target.login();time.sleep(1.1);second=target.login()
        assert first!=second, 'two independently issued tokens required'
        for token in (first,second):assert target.request('GET','/api/auth/me',token).status==200
        assert target.request('POST','/api/auth/logout',first).status==204
        for token in (first,second):assert target.request('GET','/api/auth/me',token).status==401
        current=target.login()
        assert target.request('GET','/api/auth/me',current).status==200
        step('two distinct device tokens revoked through Nginx; re-login succeeds')
        dc('stop','mysql')
        health(503)
        step('real MySQL stopped: anonymous proxy health returns 503 JSON')
        dc('start','mysql')
        for _ in range(60):
            r=target.request('GET','/health')
            if r.status==200:break
            time.sleep(1)
        health(200)
        step('same MySQL container recovers: proxy health returns 200')
        # No second recovery/server process: only change permission on this new
        # project's empty temp directory, never a material file or Demo mount.
        dc('run','--no-deps','--entrypoint','/bin/sh','initialize','-c','chmod 500 /uploads/tmp')
        changed_permissions=True
        health(503)
        dc('run','--no-deps','--entrypoint','/bin/sh','initialize','-c','chmod 755 /uploads/tmp')
        changed_permissions=False
        health(200)
        step('private temp directory unwritable: proxy health 503; permission restored: 200')
        dc('stop','nginx','app')
        dc('start','app')
        for _ in range(45):
            text=dc('ps','--format','json','app')
            if '"healthy"' in text:break
            time.sleep(1)
        dc('start','nginx')
        health(200)
        for token in (first,second):assert target.request('GET','/api/auth/me',token).status==401
        assert target.request('GET','/api/auth/me',current).status==200
        step('same-key app restart preserves new token and does not revive either revoked token')
        report['source_hashes']={p:hashlib.sha256((ROOT/p).read_bytes()).hexdigest() for p in
            ('docker-compose.yml','Dockerfile.backend','deploy/nginx.conf','cmd/server/main.go')}
        report['accepted']=True
        return 0
    except Exception as error:
        report['failure']=redact(str(error));print('FAIL '+report['failure'],flush=True);return 1
    finally:
        if changed_permissions:dc('run','--no-deps','--entrypoint','/bin/sh','initialize','-c','chmod 755 /uploads/tmp',check=False)
        dc('stop',check=False)
        report['http']=target.trace
        (runtime/'result.json').write_text(redact(json.dumps(report,indent=2)),encoding='utf-8')
        print('Evidence retained: '+str(runtime/'result.json'),flush=True)


if __name__=='__main__':sys.exit(main())
