"""Identical case functions drive both services. No backend business branches."""
from dataclasses import dataclass
from email.message import Message
from email.utils import collapse_rfc2231_value, formatdate
import hashlib
import json
import re
import time
from uuid import UUID

import jwt
from accepted_differences import expected_status, assert_response, expected_allow


@dataclass
class Case:
    name: str
    run: object
    category: str = 'required'


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def status(response, expected):
    require(response.status == expected, f'HTTP expected {expected}, observed {response.status}')


def multipart(parts, boundary='contract-boundary', close=True):
    body = bytearray()
    for field, name, data in parts:
        disposition = f'form-data; name="{field}"'
        if name is not None:
            disposition += f'; filename="{name}"'
        body += ('--' + boundary + '\r\nContent-Disposition: ' + disposition + '\r\n\r\n').encode()
        body += data + b'\r\n'
    if close:
        body += ('--' + boundary + '--\r\n').encode()
    return bytes(body), {'Content-Type': 'multipart/form-data; boundary=' + boundary}


def no_effect(a, action):
    before = a.state()
    try:
        return action()
    finally:
        require(a.state() == before, 'denied/read request changed material tables or private files')


def attachment(r):
    msg = Message(); msg['Content-Disposition'] = r.headers.get('content-disposition', '')
    # RFC 6266: filename* takes precedence over its ASCII filename fallback.
    extended = [v for k,v in msg.get_params(header='content-disposition',failobj=[]) if k=='filename' and isinstance(v,tuple)]
    filename = collapse_rfc2231_value(extended[0]) if extended else msg.get_filename()
    return {'kind': msg.get_content_disposition(), 'filename': filename}


def suite(fixture):
    cases = []
    def add(name, fn, category='required'):
        cases.append(Case(name, fn, category))
    materials = fixture['materials']
    a_id, b_id, unicode_id = [m['id'] for m in materials]
    def health(a):
        r=a.request('GET','/health');status(r,200)
        require(r.json()=={'status':'ok'} and 'set-cookie' not in r.headers,'anonymous health')
        return r.summary()
    add('health.anonymous',health)
    for value in (None,'','   ','short'):
        def configuration(a,value=value):
            require(a.reject_secret(value),'invalid JWT secret accepted or SECRET_KEY fallback')
            return {'startup_rejected':True,'secret_key_fallback':False}
        add('config.jwt_secret.'+('missing' if value is None else 'empty' if value=='' else 'whitespace' if value.isspace() else 'short'),configuration)
    def other_stack(a):
        r=a.request('GET','/api/auth/me',a.other_token());status(r,401)
        require(r.json()=={'error':'Authentication required'},'foreign signing key accepted')
        r=a.request('GET','/api/auth/me',a.login());status(r,200)
        return {'foreign_token':401,'local_relogin':200}
    add('signing_key.foreign_token_and_relogin',other_stack)
    def login_case(a):
        before = a.version()
        body = json.dumps({'username':'teacher_a','password':a.password}).encode()
        r = a.request('POST','/api/auth/login',body=body,headers={'Content-Type':'application/json'})
        status(r,200);v=r.json()
        require(set(v)=={'access_token','token_type','expires_in','redirect'},'login field set')
        require(v['token_type']=='Bearer' and v['expires_in']==28800 and v['redirect']=='/materials','login values')
        require(r.headers.get('cache-control')=='no-store' and 'set-cookie' not in r.headers,'login caching/cookie')
        claims=jwt.decode(v['access_token'],a.secret,algorithms=['HS256'])
        require(jwt.get_unverified_header(v['access_token'])['alg']=='HS256','JWT algorithm')
        require(set(claims)=={'sub','ver','iat','exp'} and claims['sub']=='1' and claims['ver']==before,'JWT identity/claims')
        require(type(claims['iat']) is int and type(claims['exp']) is int and claims['exp']-claims['iat']==28800,'JWT lifetime')
        require(a.version()==before,'login changed version')
        return {'status':r.status,'fields':sorted(v),'type':v['token_type'],'expiry':v['expires_in'],'redirect':v['redirect'],'claims':sorted(claims),'algorithm':'HS256'}
    add('login.valid_and_jwt',login_case)
    for label,payload,kind in [
        ('wrong_password',{'username':'teacher_a','password':'deliberately-wrong'},'application/json'),
        ('unknown_user',{'username':'missing','password':'deliberately-wrong'},'application/json'),
        ('array',[],'application/json'),('null',None,'application/json'),
        ('numeric_username',{'username':1,'password':'x'},'application/json'),
        ('numeric_password',{'username':'teacher_a','password':1},'application/json'),
        ('missing_password',{'username':'teacher_a'},'application/json'),
        ('malformed',b'{broken','application/json')]:
        def invalid(a,payload=payload,kind=kind):
            body=payload if isinstance(payload,bytes) else json.dumps(payload).encode()
            r=a.request('POST','/api/auth/login',body=body,headers={'Content-Type':kind});status(r,401)
            require(r.json()=={'error':'Invalid credentials'},'login error JSON');return r.summary()
        add('login.'+label,invalid)
    for name in ('teacher_a','student_a1','teacher_b','student_b1','empty_student'):
        def me(a,name=name):
            token=a.login(name,{'role':'teacher','class_id':999,'uploader_id':999})
            r=a.request('GET','/api/auth/me',token);status(r,200)
            expected=next(u for u in fixture['users'] if u['username']==name)
            cls=next(c for c in fixture['classes'] if c['id']==expected['class_id'])
            require(r.json()=={'user_id':expected['id'],'role':expected['role'],'class_id':expected['class_id'],'class_name':cls['name']},'trusted me identity')
            return r.summary()
        add('identity.'+name,me)
    for label in ('missing','malformed','tampered','expired','wrong_signature','wrong_algorithm','wrong_claim_type','missing_claim','bad_subject','cookie_only','ambiguous_header'):
        def rejected(a,label=label):
            valid=a.token();token=valid;headers={};now=int(time.time());ver=a.version()
            claims={'sub':'1','ver':ver,'iat':now,'exp':now+28800}
            if label=='missing':token=None
            elif label=='malformed':token='a.b.c'
            elif label=='tampered':token=valid+'tampered'
            elif label=='expired':token=jwt.encode(dict(claims,iat=now-28800,exp=now),a.secret,algorithm='HS256')
            elif label=='wrong_signature':token=jwt.encode(claims,'unrelated-synthetic-signing-secret-at-least-32bytes',algorithm='HS256')
            elif label=='wrong_algorithm':token=jwt.encode(claims,a.secret,algorithm='HS384')
            elif label=='wrong_claim_type':token=jwt.encode(dict(claims,ver=True),a.secret,algorithm='HS256')
            elif label=='missing_claim':token=jwt.encode({k:v for k,v in claims.items() if k!='ver'},a.secret,algorithm='HS256')
            elif label=='bad_subject':token=jwt.encode(dict(claims,sub='not-an-account'),a.secret,algorithm='HS256')
            elif label=='cookie_only':token=None;headers={'Cookie':'access_token='+valid+'; role=teacher; class_id=1'}
            elif label=='ambiguous_header':token=valid+', Bearer '+valid
            output=[]
            def action():
                for method,path in [('GET','/api/auth/me'),('GET','/api/materials'),('GET','/api/materials/'+a_id),('GET','/api/materials/'+a_id+'/file'),('POST','/api/materials')]:
                    r=a.request(method,path,token,headers=headers);output.append(r.summary());status(r,401)
                    require(r.json()=={'error':'Authentication required'} and r.headers.get('www-authenticate')=='Bearer','Bearer failure semantics')
            no_effect(a,action);return output
        add('bearer.'+label,rejected)
    def revoke(a):
        t1=a.login();t2=a.login();before=a.version();r=a.request('POST','/api/auth/logout',t1);status(r,204)
        require(r.body==b'' and a.version()==before+1,'logout commit/version')
        old=[a.request('GET','/api/auth/me',t).status for t in (t1,t2)]
        require(old==[401,401],'old token accepted')
        new=a.login();r=a.request('GET','/api/auth/me',new);status(r,200)
        return {'logout':204,'old':old,'new':200,'version_delta':1}
    add('logout.all_devices_relogin',revoke)
    for name in ('teacher_a','student_a1','teacher_b','student_b1'):
        group=1 if '_a' in name else 2
        def reads(a,name=name,group=group):
            token=a.token(name);own=[m for m in materials if m['class_id']==group];foreign=b_id if group==1 else a_id
            out={}
            def action():
                r=a.request('GET','/api/materials',token);status(r,200);items=r.json()['items']
                expected=[{'id':m['id'],'original_name':m['name'],'size':len(m['raw'].encode()),'created_at':fixture['created_at'],'uploader':'teacher_a' if group==1 else 'teacher_b'} for m in sorted(own,key=lambda m:m['id'],reverse=True)]
                require(items==expected,'scoped list fields/order');out['list']=r.json()
                for m in own:
                    r=a.request('GET','/api/materials/'+m['id'],token);status(r,200)
                    expect=next(i for i in expected if i['id']==m['id'])
                    require(r.json()==dict(expect,plain_text=m['raw'].removeprefix('\ufeff')),'detail original text/fields')
                    r=a.request('GET','/api/materials/'+m['id']+'/file',token);status(r,200)
                    require(r.body==m['raw'].encode(),'download original bytes')
                    require(r.headers.get('content-type','').split(';')[0]=='text/plain','attachment media type')
                    require(attachment(r)=={'kind':'attachment','filename':m['name']},'decoded attachment name')
                    require(r.headers.get('x-content-type-options')=='nosniff','download nosniff')
                for suffix in ('','/file'):
                    r=a.request('GET','/api/materials/'+foreign+suffix+'?role=teacher&class_id=999&uploader_id=999',token,headers={'X-Class-Id':'999','X-Role':'teacher','Cookie':'role=teacher; class_id=999'})
                    missing=a.request('GET','/api/materials/absent'+suffix,token)
                    status(r,404);status(missing,404);require(r.json()==missing.json()=={'error':'Material not found'},'foreign/missing distinction')
            no_effect(a,action);return out
        add('material.read_isolation.'+name,reads)
    def empty(a):
        r=a.request('GET','/api/materials',a.token('empty_student'));status(r,200);require(r.json()=={'items':[]},'empty array');return r.summary()
    add('material.empty_list',empty)
    for name in ('teacher_a','student_a1','teacher_b','student_b1'):
        for field in ('role','class_id','uploader_id'):
            def forged(a,name=name,field=field):
                body,headers=multipart([(field,None,b'999'),('file','x.txt',b'text')])
                r=no_effect(a,lambda:a.request('POST','/api/materials',a.token(name),body,headers))
                code=403 if name.startswith('student') else 400;status(r,code)
                require(r.json()=={'error':'Teacher role required' if code==403 else 'Client identity fields are not accepted'},'forged identity error');return r.summary()
            add('upload.forged.'+name+'.'+field,forged)
    for query in ('role=teacher','class_id=999','uploader_id=999','q=anything','&'):
        def query_case(a,query=query):
            r=no_effect(a,lambda:a.request('GET','/api/materials?'+query,a.token()));status(r,expected_status('query.'+query,a.name,400))
            if query=='&':
                assert_response('query.&',a.name,r)
                if a.name=='Flask':
                    expected=[{'id':m['id'],'original_name':m['name'],'size':len(m['raw'].encode()),'created_at':fixture['created_at'],'uploader':'teacher_a'} for m in sorted(materials,key=lambda m:m['id'],reverse=True) if m['class_id']==1]
                    require(r.json()=={'items':expected},'empty separator leaked/changed scoped list')
            return r.summary()
        add('query.'+query,query_case,'pending' if query=='&' else 'required')
    # Each rejection captures state so malformed requests cannot hide persistence.
    variants=[
        ('missing_file',[],400),('two_files',[('file','a.txt',b'x'),('file','b.md',b'y')],400),
        ('empty',[('file','a.txt',b'')],400),('unsupported',[('file','a.pdf',b'text')],400),
        ('invalid_utf8',[('file','a.txt',b'\xff')],400),('nul',[('file','a.txt',b'x\0y')],400),
        ('over_limit',[('file','a.txt',b'x'*(20*1024*1024+1))],413),
        ('bad_extension_then_identity',[('file','a.pdf',b'x'),('role',None,b'teacher')],400),
        ('invalid_text_then_second_file',[('file','a.txt',b'\xff'),('file','b.txt',b'x')],400),
        ('oversize_then_identity',[('file','a.txt',b'x'*(20*1024*1024+1)),('class_id',None,b'2')],400),
        ('wrong_field_and_empty',[('other','a.txt',b'')],400)]
    for label,parts,code in variants:
        def invalid_upload(a,parts=parts,code=code,label=label):
            body,headers=multipart(parts);r=no_effect(a,lambda:a.request('POST','/api/materials',a.token(),body,headers));status(r,expected_status('upload.'+label,a.name,code))
            if label=='oversize_then_identity':assert_response('upload.'+label,a.name,r)
            return r.summary()
        add('upload.'+label,invalid_upload,'pending' if 'then_' in label else 'required')
    def truncated(a):
        body,headers=multipart([('file','a.txt',b'partial')],close=False)
        r=no_effect(a,lambda:a.request('POST','/api/materials',a.token(),body,headers));status(r,400);return r.summary()
    add('multipart.truncated',truncated)
    def missing_boundary(a):
        r=no_effect(a,lambda:a.request('POST','/api/materials',a.token(),b'not-multipart',{'Content-Type':'multipart/form-data'}));status(r,400);return r.summary()
    add('multipart.missing_boundary',missing_boundary)
    def overhead(a):
        body,headers=multipart([('role',None,b'x'*(1024*1024+1)),('file','a.txt',b'x')])
        r=no_effect(a,lambda:a.request('POST','/api/materials',a.token(),body,headers));status(r,expected_status('multipart.large_form_field',a.name,413));assert_response('multipart.large_form_field',a.name,r);return r.summary()
    add('multipart.large_form_field',overhead,'pending')
    for method in ('PUT','PATCH','DELETE','OPTIONS'):
        for path in ('/api/materials','/api/materials/'+a_id,'/api/materials/'+a_id+'/file','/api/auth/login','/login'):
            def methods(a,method=method,path=path):
                r=no_effect(a,lambda:a.request(method,path,a.token()));status(r,expected_status('method.'+method+'.'+path,a.name,200 if method=='OPTIONS' else 405))
                allow=sorted(v.strip() for v in r.headers.get('allow','').split(',') if v.strip())
                require(allow==expected_allow(path,a.name),'Allow does not reflect registered methods')
                require(r.headers.get('content-type','').split(';')[0]==('text/html' if a.name=='Flask' else 'text/plain'),'method response media changed')
                if path.startswith('/api/'):
                    require(r.headers.get('cache-control')=='no-store','method API cache changed')
                return {'status':r.status,'allow':allow}
            add('method.'+method+'.'+path,methods,'pending')
    for target in ('/api/materials/'+unicode_id+'/file','/static/logo.png'):
        def validators(a,target=target):
            r=a.request('GET',target,a.token());status(r,200)
            require(('etag' in r.headers)==(a.name=='Flask') and ('last-modified' in r.headers)==(a.name=='Flask'),'validator policy changed')
            if a.name=='Go' and not target.startswith('/static/'):
                require(r.headers.get('cache-control')=='private, no-store','download cache changed')
            return {'etag_present':'etag' in r.headers,'last_modified':r.headers.get('last-modified')}
        add('validator.headers.'+target,validators,'pending')
        for header in ('If-None-Match','If-Modified-Since'):
            def conditional(a,target=target,header=header):
                first=a.request('GET',target,a.token());status(first,200)
                value=first.headers.get('etag','"absent-validator"') if header=='If-None-Match' else formatdate(fixture['mtime'],usegmt=True)
                # Static source mtime is different from the fixed attachment mtime.
                if header=='If-Modified-Since' and target.startswith('/static/'):
                    value=first.headers.get('last-modified',formatdate(fixture['mtime'],usegmt=True))
                r=a.request('GET',target,a.token(),headers={header:value});status(r,expected_status('validator.'+header+'.'+target,a.name,304))
                if a.name=='Flask':require(r.body==b'','304 body')
                else:
                    require(r.body==first.body and len(r.body)>0,'conditional 200 lost original bytes')
                    require('etag' not in r.headers and 'last-modified' not in r.headers,'unconfirmed validator introduced')
                    require(r.headers.get('x-content-type-options')=='nosniff','download nosniff changed')
                    if not target.startswith('/static/'):
                        require(r.body==next(m['raw'].encode() for m in materials if m['id']==unicode_id),'conditional original attachment changed')
                        require(r.headers.get('cache-control')=='private, no-store' and attachment(r)=={'kind':'attachment','filename':next(m['name'] for m in materials if m['id']==unicode_id)},'conditional download headers changed')
                return {'status':r.status,'empty_body':not r.body}
            add('validator.'+header+'.'+target,conditional,'pending')
    def cache(a):
        r=a.request('GET','/api/materials/'+unicode_id+'/file',a.token());status(r,200)
        require(r.headers.get('cache-control') in ('private, no-store','no-store'),'download cache value')
        return {'cache_control':r.headers.get('cache-control')}
    add('download.accepted_cache_difference',cache,'accepted-cache')
    for path in ('/login','/materials'):
        def page(a,path=path):
            r=a.request('GET',path);status(r,200);s=r.body.decode('utf-8')
            require('lang="zh-CN"' in s and 'CampusClaw' in s,'Chinese page shell')
            ids=('id="login"',) if path=='/login' else ('id="materials"','id="upload"','id="logout"','id="text"')
            require(all(value in s for value in ids),'page controls');require('access_token' not in s and a.password not in s and a_id not in s,'public page embeds secrets/materials')
            require(r.headers.get('cache-control')=='no-store' and 'set-cookie' not in r.headers,'public shell cache/cookie')
            return {'status':r.status,'controls':list(ids),'csp':r.headers.get('content-security-policy')}
        add('page.'+path,page)
    for filename in ('api.js','login.js','styles.css','logo.png'):
        def asset(a,filename=filename):
            r=a.request('GET','/static/'+filename);status(r,200);return {'sha256':hashlib.sha256(r.body).hexdigest(),'media':r.headers.get('content-type','').split(';')[0]}
        add('static.'+filename,asset)
    def materials_js(a):
        r=a.request('GET','/static/materials.js');status(r,200);s=r.body.decode()
        require('textContent = material.plain_text' in s and 'innerHTML' not in s and 'URL.revokeObjectURL' in s,'safe text/blob flow')
        return {'status':r.status,'textContent':True,'blob_revoke':True}
    add('static.materials_behavior',materials_js)
    for path in ('/uploads/files/'+a_id,'/static/uploads/files/'+a_id,'/files/'+a_id):
        def private(a,path=path):
            r=a.request('GET',path);status(r,404);return {'status':r.status}
        add('private.'+path,private)
    # Upload success comes after fixture-exact list tests, with semantic checks
    # on generated IDs/timestamps rather than rewriting unrelated response data.
    for label,name,raw,role in [('txt','new.txt',b'plain','teacher_a'),('md','new.MD','\ufeff中文'.encode(),'teacher_b'),('chinese','中文材料.md','中文原文'.encode(),'teacher_a'),('quoted_boundary','quoted.txt',b'quoted','teacher_a'),('student','denied.txt',b'text','student_a1')]:
        def success(a,name=name,raw=raw,role=role,label=label):
            body,headers=multipart([('file',name,raw)])
            if label=='quoted_boundary':headers['Content-Type']='multipart/form-data; boundary="contract-boundary"'
            before=a.state();r=a.request('POST','/api/materials',a.token(role),body,headers)
            if role.startswith('student'):
                status(r,403);require(a.state()==before,'student side effects');return r.summary()
            status(r,201);v=r.json();require(set(v)=={'material_id'} and str(UUID(v['material_id']))==v['material_id'],'upload ID schema')
            detail=a.request('GET','/api/materials/'+v['material_id'],a.token(role));status(detail,200);d=detail.json()
            require(set(d)=={'id','original_name','size','created_at','uploader','plain_text'},'uploaded detail fields')
            require(d['id']==v['material_id'] and d['plain_text']==raw.decode('utf-8-sig') and d['size']==len(raw) and d['uploader']==role,'upload data')
            require(bool(re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z',d['created_at'])),'timestamp format')
            file=a.request('GET','/api/materials/'+v['material_id']+'/file',a.token(role));status(file,200);require(file.body==raw,'uploaded raw bytes')
            require(attachment(file)=={'kind':'attachment','filename':d['original_name']},'uploaded filename')
            return {'status':201,'original_name':d['original_name'],'plain_text':d['plain_text'],'size':d['size'],'uploader':d['uploader'],'uuid_valid':True,'timestamp_valid':True,'raw_sha256':hashlib.sha256(raw).hexdigest()}
        add('upload.success.'+label,success)
    return cases
