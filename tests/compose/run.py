"""6.3: exercise the actual default Compose file with a new project and volumes.

No externally supplied target, Demo mount, down, volume removal or data reset.
All credentials are generated in memory; evidence is redacted before writing.
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
from cases import multipart, attachment, require

SEEDS = ('SEED_TEACHER_A_PASSWORD', 'SEED_STUDENT_A1_PASSWORD',
         'SEED_TEACHER_B_PASSWORD', 'SEED_STUDENT_B1_PASSWORD')


def main():
    name = 'campusclaw-phase5-compose-' + secrets.token_hex(4)
    runtime = ROOT / 'runtime' / name
    runtime.mkdir(parents=True, exist_ok=False)
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0)); port = s.getsockname()[1]
    require(port != 8080, 'Demo port forbidden')
    env = os.environ.copy()
    password = secrets.token_hex(24)
    env.update(DB_PASSWORD=secrets.token_hex(24), MYSQL_ROOT_PASSWORD=secrets.token_hex(24),
               JWT_SIGNING_SECRET=secrets.token_hex(32), INITIALIZATION_MODE='fresh',
               DEV_HTTP_PORT=str(port), GO_IMAGE=name + ':server', GO_INIT_IMAGE=name + ':init')
    env.update({k: password for k in SEEDS})
    # Explicit env file prevents accidental local .env settings from changing this run.
    empty_env = runtime / 'empty.env'; empty_env.write_text('# intentionally empty\n')
    base = ['docker', 'compose', '--env-file', str(empty_env), '-p', name,
            '-f', str(ROOT / 'docker-compose.yml')]
    sensitive = [password, env['DB_PASSWORD'], env['MYSQL_ROOT_PASSWORD'], env['JWT_SIGNING_SECRET']]
    report = {'project': name, 'port': port, 'accepted': False, 'checks': [], 'commands': []}
    target = HTTPAdapter('Go', f'http://127.0.0.1:{port}', env['JWT_SIGNING_SECRET'], password, runtime, None)
    counter = 0

    def redact(text):
        for value in sensitive:
            text = text.replace(value, '[redacted]')
        return re.sub(r'eyJ[\w-]+\.[\w-]+\.[\w-]+', '[redacted JWT]', text)

    def run(args, check=True, stdin=None, timeout=480, record_stdout=True):
        nonlocal counter
        counter += 1
        p = subprocess.Popen(args, cwd=ROOT, env=env, stdin=subprocess.PIPE if stdin is not None else None,
                             stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, encoding='utf-8', errors='replace')
        started = time.monotonic(); first = True
        while True:
            try:
                out, err = p.communicate(input=stdin if first else None, timeout=15); break
            except subprocess.TimeoutExpired:
                first = False
                print('Isolated verification running: ' + ' '.join(args[-3:]), flush=True)
                if time.monotonic() - started > timeout:
                    p.kill(); p.communicate(); raise RuntimeError('isolated command timed out')
        text = redact((out if record_stdout else '[stdout retained only in memory]\n') + err)
        (runtime / f'command-{counter:03}.log').write_text(text, encoding='utf-8')
        report['commands'].append({'argv': args, 'exit': p.returncode, 'log': f'command-{counter:03}.log'})
        if check and p.returncode:
            raise RuntimeError('command failed: ' + ' '.join(args[-5:]) + '\n' + text[-2000:])
        return p.returncode, out

    def dc(*args, **kw): return run(base + list(args), **kw)
    def step(message):
        report['checks'].append(message); print('PASS ' + message, flush=True)
    def sql(query):
        _, out = dc('exec', '-T', 'mysql', 'sh', '-c',
                    'MYSQL_PWD="$MYSQL_PASSWORD" mysql --max-allowed-packet=67108864 -N -B -u"$MYSQL_USER" "$MYSQL_DATABASE"', stdin=query, record_stdout=False)
        return out.strip()
    def check(r, status):
        require(r.status == status, f'expected HTTP {status}, got {r.status}'); return r
    def volumes():
        _, out = run(['docker','volume','ls','--filter','label=com.docker.compose.project='+name,'--format','{{.Name}}'])
        result = {}
        for volume in out.split():
            require(volume.startswith(name+'_'), 'foreign volume')
            _, value = run(['docker','volume','inspect','--format','{{.CreatedAt}}',volume])
            result[volume] = value.strip()
        return result
    def ids():
        _, out = dc('ps','-a','-q'); return out.split()
    def fingerprint():
        # No password hash is written to evidence, only a hash of the result.
        value = sql('SELECT * FROM users ORDER BY id; SELECT * FROM materials ORDER BY id; SELECT * FROM knowledge_entries ORDER BY id;')
        return hashlib.sha256(value.encode()).hexdigest()
    def isolated_shell(command):
        return dc('run','--no-deps','--entrypoint','/bin/sh','initialize','-c',command)

    try:
        saved = env.pop('JWT_SIGNING_SECRET')
        code, _ = dc('config','--quiet',check=False)
        env['JWT_SIGNING_SECRET'] = saved
        require(code != 0, 'missing JWT accepted by Compose')
        step('missing JWT fails configuration before creating resources')
        _, config = dc('config','--format','json')
        model = json.loads(config)
        require(set(model['services']) == {'nginx','app','mysql','initialize','storage-init'}, 'unexpected service')
        require(model['services']['app']['environment']['STARTUP_MODE'] == 'application', 'health-only mode')
        for service, spec in model['services'].items():
            if service != 'nginx': require(not spec.get('ports'), 'non-ingress published port')
        dc('build','app','initialize')
        for key in SEEDS: env[key] = ''
        code, _ = dc('up','-d',check=False)
        require(code != 0, 'missing seed credentials did not block startup')
        require(sql('SELECT COUNT(*) FROM users; SELECT COUNT(*) FROM materials;') == '0\n0', 'missing seeds wrote business data')
        _, running = dc('ps','--status','running','--services')
        require('app' not in running.split() and 'nginx' not in running.split(), 'failed init opened ingress')
        step('missing seed passwords leaves empty business tables and blocks app/Nginx')
        env.update({k:password for k in SEEDS})
        dc('up','-d','--wait','--wait-timeout','180')
        dc('exec','-T','nginx','nginx','-t')
        check(target.request('GET','/health'),200)
        require(sql('SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE(); SELECT COUNT(*) FROM classes; SELECT COUNT(*) FROM users; SELECT COUNT(*) FROM materials; SELECT COUNT(*) FROM knowledge_entries; SELECT COUNT(*) FROM sessions;') == '8\n2\n4\n2\n2\n0', 'fresh table/count mismatch')
        step('actual default Compose fresh deployment: eight tables, A/B and four accounts, two complete sample materials, empty sessions')
        tokens = {u:target.login(u) for u in ('teacher_a','student_a1','teacher_b','student_b1')}
        all_materials = {}
        for user, token in tokens.items():
            identity = check(target.request('GET','/api/auth/me',token),200).json()
            require(identity['class_name'] == ('A' if '_a' in user else 'B'), 'identity class')
            require(identity['role'] == user.split('_')[0], 'identity role')
        for user in ('teacher_a','teacher_b'):
            raw = ('\ufeffDefault Compose '+user+' \u4e2d\u6587\n').encode()
            body, headers = multipart([('file','compose.txt',raw)])
            item = check(target.request('POST','/api/materials',tokens[user],body,headers),201).json()
            all_materials[item['material_id']] = raw
        for user in ('student_a1','student_b1'):
            body, headers = multipart([('file','student.txt',b'refused')])
            check(target.request('POST','/api/materials',tokens[user],body,headers),403)
        for user, token in tokens.items():
            listing = check(target.request('GET','/api/materials',token),200).json()['items']
            require(len(listing)==2, 'class list size')
            own = {v['id'] for v in listing}
            for item in listing:
                mid = item['id']
                detail = check(target.request('GET','/api/materials/'+mid,token),200).json()
                download = check(target.request('GET','/api/materials/'+mid+'/file',token),200)
                require(download.body.decode('utf-8-sig') == detail['plain_text'], 'original text mismatch')
                require(attachment(download)['kind']=='attachment' and download.headers.get('cache-control')=='private, no-store'
                        and download.headers.get('x-content-type-options')=='nosniff', 'unsafe download')
                if mid in all_materials: require(download.body==all_materials[mid], 'uploaded bytes changed')
                else: all_materials[mid] = download.body
            for mid in set(all_materials)-own:
                for suffix in ('','/file'):
                    require(check(target.request('GET','/api/materials/'+mid+suffix,token),404).json()=={'error':'Material not found'}, 'cross-class disclosure')
        # Recheck both directions once all A/B IDs are known.
        for user, token in tokens.items():
            own = {v['id'] for v in target.request('GET','/api/materials',token).json()['items']}
            for mid in set(all_materials)-own:
                for suffix in ('','/file'):check(target.request('GET','/api/materials/'+mid+suffix,token),404)
        for path in ('/login','/materials','/static/logo.png'):check(target.request('GET',path),200)
        for mid in all_materials:
            for prefix in ('/uploads/files/','/static/uploads/files/','/files/'):
                check(target.request('GET',prefix+mid),404)
        check(target.request('GET','/api/auth/me'),401)
        check(target.request('GET','/api/auth/me',headers={'Cookie':'session=fake'}),401)
        check(target.request('POST','/api/auth/logout',tokens['teacher_a']),204)
        check(target.request('GET','/api/auth/me',tokens['teacher_a']),401)
        current = target.login('teacher_a')
        check(target.request('GET','/api/auth/me',current),200)
        step('Nginx Bearer forwarding, all four identities, teacher upload/student 403, bidirectional isolation, text/bytes, logout/re-login and private paths')
        # Test exact legal file limit through Nginx, including multipart overhead.
        raw = b'x' * (20*1024*1024)
        body, headers = multipart([('file','limit.txt',raw)])
        item = check(target.request('POST','/api/materials',current,body,headers),201).json()
        all_materials[item['material_id']] = raw
        body, headers = multipart([('file','over.txt',raw+b'x')])
        check(target.request('POST','/api/materials',current,body,headers),413)
        step('Nginx permits legal 20 MiB plus multipart overhead; Go rejects 20 MiB + 1')
        before = fingerprint(); old_ids = ids(); old_volumes = volumes()
        dc('stop','nginx','app')
        # Wrong mode and unknown existing data must fail without seeding/resetting.
        env['INITIALIZATION_MODE'] = 'migrate'
        code, _ = dc('run','--no-deps','initialize',check=False)
        require(code != 0 and fingerprint()==before, 'migrate accepted fresh or changed data')
        env['INITIALIZATION_MODE'] = 'fresh'
        receipt = '/uploads/.campusclaw-initialization.json'
        isolated_shell('mv '+receipt+' '+receipt+'.saved')
        try:
            code, _ = dc('run','--no-deps','initialize',check=False)
            require(code != 0 and fingerprint()==before, 'unrecognized nonempty target was reseeded')
        finally:
            isolated_shell('mv '+receipt+'.saved '+receipt)
        saved = env['JWT_SIGNING_SECRET']; env['JWT_SIGNING_SECRET']='short'
        code, _ = dc('run','--no-deps','initialize',check=False)
        env['JWT_SIGNING_SECRET']=saved
        require(code!=0 and fingerprint()==before, 'short JWT accepted or changed data')
        step('migrate rejects fresh receipt; unknown nonempty data and invalid JWT fail without changing users/materials/body')
        # Different injected seeds must not reset existing accounts. No volume is removed.
        replacement = secrets.token_hex(24); sensitive.append(replacement)
        env.update({k:replacement for k in SEEDS})
        dc('up','-d','--force-recreate','--wait','--wait-timeout','180')
        require(not set(old_ids)&set(ids()), 'containers were not all recreated')
        require(volumes()==old_volumes and len(old_volumes)==2, 'persistent volumes replaced')
        require(fingerprint()==before, 'container recreation changed account/material data')
        check(target.request('GET','/api/auth/me',tokens['teacher_a']),401)
        check(target.request('GET','/api/auth/me',current),200)
        target.login('teacher_a')  # Original password still works.
        check(target.request('POST','/api/auth/login',body=json.dumps({'username':'teacher_a','password':replacement}).encode(),headers={'Content-Type':'application/json'}),401)
        for user in ('teacher_a','teacher_b'):
            token=target.login(user)
            for item in check(target.request('GET','/api/materials',token),200).json()['items']:
                result=check(target.request('GET','/api/materials/'+item['id']+'/file',token),200)
                require(result.body==all_materials[item['id']], 'retained file bytes changed')
        report['persistent_volumes']=old_volumes
        report['data_sha256_before_after']=before
        report['files_sha256']={k:hashlib.sha256(v).hexdigest() for k,v in all_materials.items()}
        step('all five service containers recreated retaining same two volumes; all five files byte-identical, original password/version and valid/revoked tokens preserved')
        report['containers']=[]
        for cid in ids():
            _, value=run(['docker','inspect','--format','{{json .Mounts}}',cid]); mounts=json.loads(value)
            _, value=run(['docker','inspect','--format','{{json .NetworkSettings.Ports}}',cid]); ports=json.loads(value)
            _, service=run(['docker','inspect','--format','{{index .Config.Labels "com.docker.compose.service"}}',cid]); service=service.strip()
            for mount in mounts:
                if mount['Type']=='volume':require(mount['Name'].startswith(name+'_'), 'Demo/foreign volume mounted')
                else:require(service=='nginx' and mount['Destination']=='/etc/nginx/nginx.conf' and not mount['RW'], 'unexpected bind')
            if service!='nginx':require(not any((ports or {}).values()), 'published backend port')
            report['containers'].append({'service':service,'mounts':mounts,'ports':ports})
        step('actual resource audit: only new project volumes; only Nginx publishes loopback port; no Flask/Python/SQLite/RAG service or Demo mount')
        report['accepted']=True
        return 0
    except Exception as error:
        report['failure']=redact(str(error)); print('FAIL '+report['failure'],flush=True); return 1
    finally:
        # Test resources are stopped, never deleted. The live Demo is never contacted.
        dc('stop',check=False)
        report['http']=target.trace
        report['http_observations']=len(target.trace)
        (runtime/'result.json').write_text(redact(json.dumps(report,ensure_ascii=False,indent=2)),encoding='utf-8')
        print('Evidence retained: '+str(runtime/'result.json'),flush=True)


if __name__=='__main__':sys.exit(main())
