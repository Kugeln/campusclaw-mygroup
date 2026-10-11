"""Task 6.2: new Compose resources, stopped synthetic source, fail-closed ingress.

No source/Demo URL or database can be supplied. No down/rm/delete command exists.
"""
from contextlib import closing
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import sqlite3
import subprocess
import sys
import time
import urllib.request

ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT))
sys.path.insert(0,str(ROOT/'tests/http_contract'))
from adapters import start_flask, HTTPAdapter
from cases import multipart, attachment
from gate import allow_cutover

TABLES=('classes','users','materials','knowledge_entries','assignments','assistants','skills')
MAINTENANCE='events {}\nhttp { access_log off; server { listen 8080; location / { return 503; } } }\n'
ACTIVE='''events {}
http {
 access_log off;
 server {
  listen 8080;
  client_max_body_size 0;
  location / {
   proxy_pass http://app:8080;
   proxy_set_header Host $http_host;
   proxy_set_header Authorization $http_authorization;
   proxy_set_header X-Forwarded-Proto $scheme;
   proxy_set_header X-Forwarded-For $remote_addr;
   proxy_set_header X-User-Id "";
   proxy_set_header X-Role "";
   proxy_set_header X-Class-Id "";
   proxy_next_upstream off;
   proxy_read_timeout 120s;
  }
 }
}
'''


def require(value,message):
    if not value: raise AssertionError(message)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def inventory(root):
    return {p.relative_to(root).as_posix():digest(p) for p in sorted(root.rglob('*')) if p.is_file()}


def compose_config(runtime,image):
    db={'DB_HOST':'mysql','DB_PORT':'3306','DB_NAME':'cutover','DB_USER':'cutover',
        'DB_PASSWORD':'${CUTOVER_DB_PASSWORD:?required}','UPLOAD_ROOT':'/uploads'}
    return {'services':{
        'mysql':{'image':'mysql:8.4.4','environment':{'MYSQL_DATABASE':'cutover','MYSQL_USER':'cutover',
            'MYSQL_PASSWORD':'${CUTOVER_DB_PASSWORD:?required}','MYSQL_ROOT_PASSWORD':'${CUTOVER_ROOT_PASSWORD:?required}'},
            'command':['--max-allowed-packet=67108864','--log-bin-trust-function-creators=1'],
            'volumes':['mysql-data:/var/lib/mysql'],'networks':['backend']},
        'tool':{'image':image,'build':{'context':str(ROOT),'dockerfile':'tests/cutover/Dockerfile.backend',
                                   'args':{'GOPROXY':os.environ.get('CUTOVER_GOPROXY','https://proxy.golang.org,direct')}},
            'profiles':['offline'],'user':'10001:10001','environment':db,
            'volumes':['target-files:/uploads',str(runtime/'snapshot')+':/snapshot:ro',
                       str(runtime/'fault-snapshot')+':/fault:ro',str(runtime/'reports')+':/reports'],
            'networks':['backend']},
        'storage-init':{'image':'alpine:3.23.3','profiles':['offline'],'network_mode':'none',
            'volumes':['target-files:/uploads'],
            'command':['sh','-c','mkdir -p /uploads/tmp /uploads/files && chown 10001:10001 /uploads /uploads/tmp /uploads/files']},
        'app':{'image':image,'profiles':['target'],'user':'10001:10001',
            'environment':dict(db,JWT_SIGNING_SECRET='${CUTOVER_JWT_SECRET:?required}',LISTEN_ADDR=':8080'),
            'volumes':['target-files:/uploads'],'networks':['backend'],
            'healthcheck':{'test':['CMD','/server','healthcheck'],'interval':'2s','timeout':'6s','retries':15}},
        'nginx':{'image':'nginx:1.28.0-alpine','ports':['127.0.0.1::8080'],
            'volumes':[str(runtime/'nginx')+':/rehearsal:ro'],
            'command':['nginx','-c','/rehearsal/nginx.conf','-g','daemon off;'],
            'networks':['frontend','backend']},
    },'networks':{'frontend':{},'backend':{'internal':True}},
       'volumes':{'mysql-data':{},'target-files':{}}}


def main():
    name='campusclaw-phase5-cutover-'+secrets.token_hex(4)
    runtime=ROOT/'runtime'/name
    runtime.mkdir(parents=True,exist_ok=False)
    for d in ('source','reports','nginx'):(runtime/d).mkdir()
    fixture=json.loads((ROOT/'tests/http_contract/fixtures.json').read_text(encoding='utf-8'))
    env=os.environ.copy()
    env.update(CUTOVER_DB_PASSWORD=secrets.token_hex(24),CUTOVER_ROOT_PASSWORD=secrets.token_hex(24),
               CUTOVER_JWT_SECRET=secrets.token_hex(32),MATERIAL_BACKEND='sqlite')
    password=secrets.token_hex(24); source_secret=secrets.token_hex(32)
    sensitive=[password,source_secret,env['CUTOVER_DB_PASSWORD'],env['CUTOVER_ROOT_PASSWORD'],env['CUTOVER_JWT_SECRET']]
    report={'project':name,'started_at':datetime.now(timezone.utc).isoformat(),'accepted':False,'checks':[],
            'runtime':str(runtime),'image':name+':rehearsal','demo':{}}
    compose=runtime/'compose.json'
    base=['docker','compose','--project-name',name,'--file',str(compose)]
    source_stop=None; started=False; counter=0
    def redact(s):
        for v in sensitive:s=s.replace(v,'[redacted]')
        return re.sub(r'eyJ[\w-]+\.[\w-]+\.[\w-]+','[redacted JWT]',s)
    def run(args,check=True,timeout=180,stdin=None):
        p=subprocess.Popen(args,cwd=ROOT,env=env,stdin=subprocess.PIPE if stdin is not None else None,
                           stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,encoding='utf-8',errors='replace')
        begin=time.monotonic();first=True
        while True:
            try:
                out,err=p.communicate(input=stdin if first else None,timeout=15);break
            except subprocess.TimeoutExpired:
                first=False
                print('Waiting for isolated '+args[0]+' operation',flush=True)
                if time.monotonic()-begin>timeout:
                    p.kill();p.communicate();raise RuntimeError('isolated command deadline exceeded')
        text=redact(out+err)
        if check and p.returncode:
            print(text,flush=True);raise RuntimeError('isolated command failed')
        return p.returncode,text
    def dc(*args,**kwargs):return run(base+list(args),**kwargs)
    def sql(statement,check=True):
        return dc('exec','-T','mysql','sh','-c',
                  'MYSQL_PWD="$MYSQL_PASSWORD" exec mysql --protocol=TCP -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" --batch --skip-column-names',
                  stdin=statement,check=check)
    def tool(executable,args,check=True):
        nonlocal counter
        counter+=1
        return dc('run','--no-deps','--name',name+'-tool-'+str(counter),'-T','--entrypoint',executable,'tool',*args,check=check)
    def step(label):
        report['checks'].append(label);print('PASS '+label,flush=True)
    def migrate(mode,label,source='/snapshot',check=True):
        path=runtime/'reports'/(label+'.json')
        require(not path.exists(),'report overwrite refused')
        code,output=tool('/migrate',['-mode',mode,'-source',source+'/source.db','-source-uploads',source+'/uploads',
                                 '-report','/reports/'+path.name],check=check)
        if code:
            report.setdefault('migration_failures',{})[label]={'exit_code':code,'output':output}
        return code,json.loads(path.read_text(encoding='utf-8')) if path.exists() else None
    def demo_probe():
        result={}
        for path in ('/health','/login'):
            try:
                with urllib.request.urlopen('http://127.0.0.1:8080'+path,timeout=8) as r:
                    data=r.read();result[path]={'status':r.status,'content_ok':json.loads(data)=={'status':'ok'} if path=='/health' else b'CampusClaw' in data}
            except Exception as e:result[path]={'error':type(e).__name__}
        return result
    def stopped_app():
        _,cid=dc('ps','-a','-q','app')
        if not cid.strip():return True
        _,state=run(['docker','inspect','--format','{{.State.Running}}',cid.strip()])
        return state.strip()=='false'
    try:
        report['demo']['before']=demo_probe()
        os.environ['MATERIAL_BACKEND']='sqlite';os.environ.pop('SECRET_KEY',None);os.environ.pop('MAX_UPLOAD_BYTES',None)
        source,source_stop=start_flask(ROOT,runtime/'source',fixture,source_secret,password)
        dbpath=runtime/'source/flask.db'
        with closing(sqlite3.connect(dbpath)) as db:
            with db:
                db.execute('UPDATE users SET session_version=id+6')
                for i,table in enumerate(('assignments','assistants','skills'),101):
                    db.execute('INSERT INTO '+table+'(id,name) VALUES (?,?)',(i,'cutover-'+table))
                db.execute('INSERT INTO sessions(sid_hash,data,expires_at) VALUES (?,?,?)',
                           (hashlib.sha256(b'isolated-legacy-sid').hexdigest(),'{}',4102444800))
        # Real source authentication, revoke once so migration cannot quietly reset versions.
        revoked=source.login()
        require(source.request('POST','/api/auth/logout',revoked).status==204,'source logout failed')
        old=source.login()
        require(source.request('GET','/api/auth/me',old).status==200,'source bearer failed')
        require(source.request('GET','/api/auth/me',revoked).status==401,'source revocation failed')
        report['source_origin']=source.origin
        source_stop();source_stop=None
        try:source.request('GET','/health');raise AssertionError('source still serving')
        except OSError:pass
        step('source HTTP stopped; no source writer during snapshot')
        snapshot=runtime/'snapshot';snapshot.mkdir()
        with closing(sqlite3.connect(dbpath)) as db,closing(sqlite3.connect(snapshot/'source.db')) as backup:
            db.execute('PRAGMA wal_checkpoint(FULL)');db.backup(backup)
            report['source_counts']={t:db.execute('SELECT COUNT(*) FROM '+t).fetchone()[0] for t in TABLES+('sessions',)}
            versions=dict(db.execute('SELECT id,session_version FROM users'))
        shutil.copytree(runtime/'source/flask-uploads',snapshot/'uploads')
        require(not any((snapshot/('source.db'+s)).exists() for s in ('-wal','-shm','-journal')),'snapshot sidecars')
        snapshot_hashes=inventory(snapshot);report['source_versions']=versions
        shutil.copytree(snapshot,runtime/'fault-snapshot')
        bad=runtime/'fault-snapshot/uploads/files'/fixture['materials'][0]['id']
        bad.write_bytes(b'corrupt-isolated-copy')
        (runtime/'nginx/nginx.conf').write_text(MAINTENANCE,encoding='utf-8')
        compose.write_text(json.dumps(compose_config(runtime,report['image']),indent=2),encoding='utf-8')
        # Never print rendered config; interpolation contains generated secrets.
        dc('config','--quiet')
        dc('build','tool',timeout=900)
        started=True
        dc('up','-d','mysql','nginx')
        for attempt in range(100):
            code,_=sql('SELECT 1;',check=False)
            if code==0:break
            if attempt%15==0:print('Waiting for new Compose MySQL',flush=True)
            time.sleep(1)
        else:raise RuntimeError('new Compose MySQL unavailable')
        _,endpoint=dc('port','nginx','8080')
        require(endpoint.strip().startswith('127.0.0.1:') and not endpoint.strip().endswith(':8080'),'unsafe ingress port')
        target=HTTPAdapter('Go','http://'+endpoint.strip(),env['CUTOVER_JWT_SECRET'],password,Path('unused'),None)
        report['target_origin']=target.origin
        require(target.request('GET','/health').status==503,'maintenance gate not closed')
        dc('run','--no-deps','--name',name+'-storage-init','-T','storage-init')
        tool('/admin',['-mode','schema'])
        _,tables=sql('SHOW TABLES;')
        require(set(tables.split())==set(TABLES+('sessions',)),'eight target tables differ')
        step('new project networks/volumes; only Nginx port published; eight-table schema ready')
        code,bad_report=migrate('preflight','bad-source','/fault',check=False)
        require(code!=0 and bad_report is None,'corrupt snapshot falsely verified')
        require(not allow_cutover(bad_report,'x','y',True,False) and target.request('GET','/health').status==503,'bad source opened ingress')
        step('corrupt source preflight fails; no success report; Nginx remains 503')
        _,preflight=migrate('preflight','preflight')
        require(preflight['Verified'] is False,'preflight incorrectly authorizes cutover')
        sql("CREATE TRIGGER reject_cutover_body BEFORE INSERT ON knowledge_entries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='isolated cutover fault';")
        code,failed=migrate('import','failed-import',check=False)
        require(code!=0 and failed is None,'injected transaction error falsely succeeded')
        require('insert failed in knowledge_entries' in report['migration_failures']['failed-import']['output'],'fault did not exercise insert transaction')
        for table in TABLES+('sessions',):
            _,value=sql('SELECT COUNT(*) FROM '+table+';');require(value.strip()=='0','failed import left rows')
        tool('/bin/sh',['-c','test ! -e /uploads/.campusclaw-initialization.json && test -z "$(ls -A /uploads/files)" && test -z "$(ls -A /uploads/tmp)"'])
        dc('up','-d','--no-deps','app')
        for _ in range(25):
            if stopped_app():break
            time.sleep(1)
        require(stopped_app(),'unverified app did not refuse startup')
        require(not allow_cutover(failed,'x','y',True,False) and target.request('GET','/health').status==503,'failed import switched ingress')
        step('transaction injection rolls back all rows; no success receipt/files; app refuses startup; ingress stays 503')
        sql('DROP TRIGGER reject_cutover_body;') # our own temporary trigger only
        require(stopped_app(),'offline importer cannot share writable volume with server')
        _,imported=migrate('import','import')
        _,verified=migrate('verify','verify')
        require(verified['Verified'] and imported['Fingerprint']==verified['Fingerprint']==preflight['Fingerprint'],'migration fingerprint mismatch')
        require(verified['Tables']==preflight['Tables'] and verified['FilesSHA256']==preflight['FilesSHA256'] and verified['BodiesSHA256']==preflight['BodiesSHA256'],'data/hash evidence differs')
        require(verified['SourceSHA256']==digest(snapshot/'source.db'),'snapshot association mismatch')
        _,version_rows=sql('SELECT id,session_version FROM users ORDER BY id;')
        actual_versions={int(line.split()[0]):int(line.split()[1]) for line in version_rows.strip().splitlines()}
        require(actual_versions==versions,'session versions reset')
        _,sessions=sql('SELECT COUNT(*) FROM sessions;');require(sessions.strip()=='0','old sessions imported')
        report['migration']={'tables':verified['Tables'],'files':verified['FilesSHA256'],'bodies':verified['BodiesSHA256'],
                             'fingerprint':verified['Fingerprint'],'source_sha256':verified['SourceSHA256'],'target_versions':actual_versions,'target_sessions':0}
        step('seven tables IDs/fields/counts and body/file SHA-256 match; nonzero versions preserved; sessions empty')
        # Successful-looking but stale or corrupt verification must not open ingress.
        sql("UPDATE knowledge_entries SET plain_text='isolated corruption' WHERE id=1;")
        code,wrong=migrate('verify','failed-verification',check=False)
        require(code!=0 and wrong is None and target.request('GET','/health').status==503,'verification failure did not block cutover')
        original=fixture['materials'][0]['raw'].removeprefix('\ufeff').encode().hex()
        sql("UPDATE knowledge_entries SET plain_text=CONVERT(UNHEX('"+original+"') USING utf8mb4) WHERE id=1;")
        _,verified=migrate('verify','reverified')
        step('target-body corruption blocks verification and cutover; repaired isolated copy reverified')
        require(inventory(snapshot)==snapshot_hashes,'read-only backup changed')
        dc('up','-d','--no-deps','app')
        healthy=False
        for _ in range(45):
            code,_=dc('exec','-T','app','/server','healthcheck',check=False)
            if code==0:healthy=True;break
            time.sleep(1)
        require(allow_cutover(verified,digest(snapshot/'source.db'),preflight['Fingerprint'],source_stop is None,healthy),'cutover gate refused')
        (runtime/'nginx/nginx.conf').write_text(ACTIVE,encoding='utf-8')
        dc('exec','-T','nginx','nginx','-t','-c','/rehearsal/nginx.conf')
        dc('exec','-T','nginx','nginx','-s','reload','-c','/rehearsal/nginx.conf')
        for _ in range(30):
            if target.request('GET','/health').status==200:break
            time.sleep(.2)
        require(target.request('GET','/health').json()=={'status':'ok'},'proxy did not switch')
        step('validated app healthy before Nginx config test/reload; same isolated port now proxies Go')
        require(target.request('GET','/api/auth/me',old).status==401,'old signing-key token accepted')
        require(target.request('GET','/api/auth/me',headers={'Cookie':'session=isolated-legacy-sid; sid=isolated-legacy-sid'}).status==401,'old SID accepted')
        require(target.request('GET','/api/auth/me',headers={'Cookie':'access_token='+old}).status==401,'cookie token fallback')
        require(target.request('GET','/api/auth/me').status==401,'anonymous allowed')
        for user in fixture['users']:
            token=target.login(user['username'])
            me=target.request('GET','/api/auth/me',token)
            require(me.status==200 and me.json()=={'user_id':user['id'],'role':user['role'],'class_id':user['class_id'],
                    'class_name':next(c['name'] for c in fixture['classes'] if c['id']==user['class_id'])},'proxy trusted identity mismatch')
            listing=target.request('GET','/api/materials',token)
            own=[m for m in fixture['materials'] if m['class_id']==user['class_id']]
            require(listing.status==200 and {m['id'] for m in listing.json()['items']}=={m['id'] for m in own},'scoped list mismatch')
            for m in fixture['materials']:
                detail=target.request('GET','/api/materials/'+m['id'],token)
                file=target.request('GET','/api/materials/'+m['id']+'/file',token)
                if m in own:
                    require(detail.status==200 and detail.json()['plain_text']==m['raw'].removeprefix('\ufeff'),'proxy detail body mismatch')
                    require(file.status==200 and file.body==m['raw'].encode() and attachment(file)=={'kind':'attachment','filename':m['name']},'proxy attachment mismatch')
                    require(file.headers.get('cache-control')=='private, no-store' and file.headers.get('x-content-type-options')=='nosniff','proxy safety headers')
                else:
                    require(detail.status==file.status==404 and detail.json()==file.json()=={'error':'Material not found'},'cross-class disclosure')
            require(target.request('GET','/api/materials/absent',token).status==404,'missing material status')
            body,headers=multipart([('role',None,b'teacher'),('file','forged.txt',b'denied')])
            require(target.request('POST','/api/materials',token,body,headers).status==(403 if user['role']=='student' else 400),'forged upload authorization')
        for path in ('/login','/materials','/static/logo.png'):
            require(target.request('GET',path).status==200,'proxy page/static failed')
        for path in ('/uploads/files/'+fixture['materials'][0]['id'],'/static/uploads/files/'+fixture['materials'][0]['id']):
            require(target.request('GET',path).status==404,'private upload exposed')
        step('new-key re-login, Authorization forwarding, A/B both roles, original details/bytes, SID/token rejection, private URL denial')
        # Stop the sole target writer before re-running the offline verifier.
        dc('stop','app');require(stopped_app(),'app did not stop before final offline verification')
        _,final=migrate('verify','after-http')
        require(final['Fingerprint']==verified['Fingerprint'],'read/denied HTTP requests changed migrated data')
        require(inventory(snapshot)==snapshot_hashes,'source backup modified')
        step('post-HTTP offline re-verification matches all seven tables and files; backups unchanged')
        report['http']=target.trace
        report['source_backup_hashes']=snapshot_hashes
        # Inspect only safe metadata, never container Env (generated secrets).
        _,ids=dc('--profile','target','--profile','offline','ps','-a','-q')
        report['containers']=[]
        for cid in ids.split():
            _,mount_text=run(['docker','inspect','--format','{{json .Mounts}}',cid])
            _,port_text=run(['docker','inspect','--format','{{json .NetworkSettings.Ports}}',cid])
            _,container_name=run(['docker','inspect','--format','{{.Name}}',cid])
            mounts=json.loads(mount_text);ports=json.loads(port_text)
            for mount in mounts:
                if mount['Type']=='volume':require(mount['Name'].startswith(name+'_'),'foreign volume detected')
                elif mount['Type']=='bind':require(name in mount['Source'],'foreign host mount detected')
                else:raise AssertionError('unexpected mount type')
                if 'nginx' in container_name:require(mount['Destination']=='/rehearsal','Nginx received data volume')
            if 'nginx' not in container_name:require(not any((ports or {}).values()),'non-ingress host port detected')
            report['containers'].append({'name':container_name.strip(),'mounts':mounts,'ports':ports})
        step('actual container mounts belong only to new project; Nginx has no uploads; app/MySQL have no published ports')
        report['accepted']=True
        return 0
    except Exception as error:
        report['failure']=redact(str(error));print('BLOCKED/FAIL '+report['failure'],flush=True)
        return 1
    finally:
        if source_stop:source_stop()
        if started:
            # Close the isolated ingress first, including any failed post-switch checks.
            (runtime/'nginx/nginx.conf').write_text(MAINTENANCE,encoding='utf-8')
            dc('exec','-T','nginx','nginx','-s','reload','-c','/rehearsal/nginx.conf',check=False)
            dc('--profile','target','--profile','offline','stop',check=False)
        report['demo']['after']=demo_probe()
        (runtime/'result.json').write_text(redact(json.dumps(report,ensure_ascii=False,indent=2)),encoding='utf-8')
        print('Evidence and isolated resources retained: '+str(runtime/'result.json'),flush=True)


if __name__=='__main__':sys.exit(main())
