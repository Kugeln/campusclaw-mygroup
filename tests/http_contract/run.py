"""Run shared HTTP cases against disposable fixtures; never target a supplied URL.

Resources are retained, not deleted. Exit 1 means an unresolved contract result;
exit 2 means infrastructure/setup failed and acceptance is blocked.
"""
from collections import Counter
from datetime import datetime, timezone
import argparse
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT))
from adapters import start_flask, start_go
from cases import suite
from accepted_differences import RULES
import pymysql


def select_cases(cases, names):
    missing = set(names) - {case.name for case in cases}
    if missing:
        raise ValueError('unknown contract case: ' + ', '.join(sorted(missing)))
    return [case for case in cases if not names or case.name in names]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--case', action='append', default=[], help='exact shared case ID; repeat to select multiple cases')
    args = parser.parse_args()
    fixture_path = Path(__file__).with_name('fixtures.json')
    fixture = json.loads(fixture_path.read_text(encoding='utf-8'))
    all_cases = suite(fixture)
    try:
        selected = select_cases(all_cases, args.case)
    except ValueError as error:
        parser.error(str(error))
    suffix = secrets.token_hex(4)
    name = 'campusclaw-phase5-contract-' + suffix
    runtime = ROOT / 'runtime' / name
    runtime.mkdir(parents=True, exist_ok=False)
    env = os.environ.copy()
    env.update(DB_HOST='127.0.0.1', DB_NAME='phase5_contract_' + suffix,
               DB_USER='phase5_contract', DB_PASSWORD=secrets.token_hex(24),
               MYSQL_ROOT_PASSWORD=secrets.token_hex(24), JWT_SIGNING_SECRET=secrets.token_hex(32),
               CONTRACT_PASSWORD=secrets.token_hex(24), CONTRACT_FIXTURES=str(fixture_path),
               UPLOAD_ROOT=str(runtime / 'go-uploads'), PHASE5_CONTRACT_ISOLATED='1',
               MATERIAL_BACKEND='sqlite', LISTEN_ADDR='127.0.0.1:0')
    env.update(MYSQL_DATABASE=env['DB_NAME'], MYSQL_USER=env['DB_USER'], MYSQL_PASSWORD=env['DB_PASSWORD'])
    for key in ('SECRET_KEY', 'MAX_UPLOAD_BYTES', 'STARTUP_MODE'):
        env.pop(key, None)
    flask_secret = secrets.token_hex(32)
    sensitive = [env[k] for k in ('DB_PASSWORD','MYSQL_ROOT_PASSWORD','JWT_SIGNING_SECRET','CONTRACT_PASSWORD')] + [flask_secret]

    def sanitized(value):
        for secret in sensitive:
            value = value.replace(secret, '[redacted]')
        return re.sub(r'eyJ[\w-]+\.[\w-]+\.[\w-]+', '[redacted JWT]', value)

    def run(args, check=True, timeout=180):
        process = subprocess.Popen(args, cwd=ROOT, env=env, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, text=True, encoding='utf-8', errors='replace')
        started = time.monotonic()
        while True:
            try:
                out, err = process.communicate(timeout=15)
                break
            except subprocess.TimeoutExpired:
                print('Waiting for isolated ' + args[0], flush=True)
                if time.monotonic() - started > timeout:
                    process.kill(); process.communicate()
                    raise RuntimeError('isolated command timeout; resources retained')
        output = sanitized(out + err)
        if check and process.returncode:
            print(output, flush=True)
            raise RuntimeError('isolated command failed: ' + args[0])
        return process.returncode, output

    report = {'schema_version':1, 'started_at':datetime.now(timezone.utc).isoformat(),
              'resources':{'name':name,'volume':name+'-data','runtime':str(runtime),
                           'mysql_image':'mysql:8.4.4','database':env['DB_NAME']},
              'scope':'base authentication and materials; no RAG, no cutover',
              'full_suite':not args.case, 'selected_cases':[case.name for case in selected],
              'not_executed_count':len(all_cases)-len(selected),
              'results':[], 'complete':False}
    created = False
    stops = []
    print('Independent resources: ' + name, flush=True)
    try:
        run(['docker','network','create',name])
        run(['docker','volume','create',name+'-data'])
        run(['docker','run','-d','--name',name,'--network',name,'-p','127.0.0.1::3306',
             '--mount','type=volume,source='+name+'-data,target=/var/lib/mysql',
             '-e','MYSQL_DATABASE','-e','MYSQL_USER','-e','MYSQL_PASSWORD','-e','MYSQL_ROOT_PASSWORD',
             'mysql:8.4.4','--max-allowed-packet=67108864'])
        created = True
        _, endpoint = run(['docker','port',name,'3306/tcp'])
        if not endpoint.strip().startswith('127.0.0.1:'):
            raise RuntimeError('refuse non-loopback database')
        env['DB_PORT'] = endpoint.strip().rsplit(':',1)[1]
        for attempt in range(90):
            code, _ = run(['docker','exec',name,'sh','-c',
                          'MYSQL_PWD="$MYSQL_PASSWORD" mysql --protocol=TCP -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -e "SELECT 1"'],check=False)
            if code == 0:
                break
            if attempt % 15 == 0:
                print('Waiting for new isolated MySQL', flush=True)
            time.sleep(1)
        else:
            raise RuntimeError('isolated MySQL unavailable')
        executable = runtime / ('server.exe' if os.name=='nt' else 'server')
        run(['go','build','-mod=readonly','-o',str(executable),'./cmd/server'])
        run(['go','run','-mod=readonly','./tests/http_contract/fixture'])
        # Explicitly constrain Flask's environment as well as its app config.
        os.environ['MATERIAL_BACKEND'] = 'sqlite'
        os.environ.pop('SECRET_KEY', None)
        os.environ.pop('MAX_UPLOAD_BYTES', None)
        flask, stop = start_flask(ROOT,runtime,fixture,flask_secret,env['CONTRACT_PASSWORD'])
        stops.append(stop)
        database = lambda: pymysql.connect(host='127.0.0.1',port=int(env['DB_PORT']),
            user=env['DB_USER'],password=env['DB_PASSWORD'],database=env['DB_NAME'])
        go, stop = start_go(executable,env,database)
        stops.append(stop)
        report['origins'] = {'Flask':flask.origin,'Go':go.origin}
        flask.other_token = lambda: go.login()
        go.other_token = lambda: flask.login()
        print('PASS independent Flask/SQLite and Go/MySQL HTTP startup', flush=True)
        adapters = (flask,go)
        for case in selected:
            row = {'case':case.name,'category':case.category,'backends':{}}
            for adapter in adapters:
                adapter.trace = []
                try:
                    observation = case.run(adapter)
                    result = {'status':'pass','observed':observation}
                except AssertionError as error:
                    result = {'status':'fail','reason':sanitized(str(error))}
                except Exception as error:
                    result = {'status':'fail','reason':'unexpected '+type(error).__name__}
                result['http'] = adapter.trace
                row['backends'][adapter.name] = result
            left,right = row['backends']['Flask'],row['backends']['Go']
            both = left['status']==right['status']=='pass'
            if both and left['observed']==right['observed']:
                row['comparison']='match'
            elif both and case.category=='accepted-cache' and left['observed']=={'cache_control':'no-store'} and right['observed']=={'cache_control':'private, no-store'}:
                row['comparison']='accepted'
                row['accepted_rule']='download-cache'
            elif both and case.name in RULES:
                row['comparison']='accepted'
                row['accepted_rule']=RULES[case.name]
            else:
                row['comparison']='pending' if case.category=='pending' or both else 'incompatible'
            report['results'].append(row)
            print(f"{case.name}: Flask={left['status']} Go={right['status']} comparison={row['comparison']}",flush=True)
        report['complete']=True
        report['counts']={a.name:{s:sum(r['backends'][a.name]['status']==s for r in report['results']) for s in ('pass','fail','skip')} for a in adapters}
        report['comparisons']=dict(Counter(r['comparison'] for r in report['results']))
        report['selection_accepted']=all(r['comparison'] in ('match','accepted') for r in report['results'])
        report['accepted']=report['full_suite'] and report['selection_accepted']
        print(json.dumps({'counts':report['counts'],'comparisons':report['comparisons'],'accepted':report['accepted']}),flush=True)
        return 0 if report['selection_accepted'] else 1
    except Exception as error:
        report['blocked']=sanitized(str(error))
        print('BLOCKED: '+report['blocked'],flush=True)
        return 2
    finally:
        for stop in reversed(stops):
            stop()
        if created:
            run(['docker','stop',name],check=False)
        (runtime/'report.json').write_text(sanitized(json.dumps(report,ensure_ascii=False,indent=2)),encoding='utf-8')
        print('Report and isolated resources retained: '+str(runtime/'report.json'),flush=True)


if __name__=='__main__':
    sys.exit(main())
