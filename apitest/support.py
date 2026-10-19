import json
import os
import re
import time
import uuid
from urllib import request,error,parse



class NoRedirect(request.HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs):return None


class Context:
    def __init__(self):
        import subprocess
        self.subprocess=subprocess
        required=['APITEST_BASE_URL','APITEST_PASSWORD','APITEST_READY','APITEST_DB_CONTAINER','APITEST_GO_EVIDENCE','APITEST_SWAGGER_EVIDENCE']
        if any(not os.environ.get(k) for k in required):raise RuntimeError('Missing isolated current acceptance configuration')
        self.base=os.environ['APITEST_BASE_URL'].rstrip('/')
        host=parse.urlparse(self.base)
        if host.scheme!='http' or host.hostname!='127.0.0.1' or host.port in [None,80,443,8080]:raise RuntimeError('Isolated random loopback proxy required')
        self.ready=json.loads(__import__('pathlib').Path(os.environ['APITEST_READY']).read_text(encoding='utf-8-sig'))
        self.database=self.ready['database'];self.container=os.environ['APITEST_DB_CONTAINER']
        if not re.fullmatch(r'align_w5_[a-f0-9]{16}',self.database) or not re.fullmatch(r'campusclaw-align-storage-[a-f0-9]{8}',self.container):raise RuntimeError('Fresh isolated target required')
        info=json.loads(subprocess.check_output(['docker','inspect',self.container]))[0]
        if not info['State']['Running'] or info['Config']['Labels'].get('campusclaw.phase')!='align-storage' or '/var/lib/mysql' not in info['HostConfig']['Tmpfs'] or any(m.get('Type')!='tmpfs' for m in info['Mounts']):raise RuntimeError('Independent tmpfs database required: '+json.dumps({'running':info['State']['Running'],'label':info['Config']['Labels'].get('campusclaw.phase'),'tmpfs':list(info['HostConfig']['Tmpfs']),'mounts':[{'type':m.get('Type'),'destination':m.get('Destination')} for m in info['Mounts']]}))
        self.namespace='align_'+uuid.uuid4().hex[:8]
        self.password=os.environ['APITEST_PASSWORD'];self.opener=request.build_opener(NoRedirect)
        self.credentials={};self.users={};self.assertions=[];self.timings={}
        self.usernames={'ta':'ta','tb':'tb','a':'sa','a2':'sa2','b':'sb'}

    def check(self,condition,label):
        if not condition:raise AssertionError(label)
        self.assertions.append(label)

    def sql(self,query,args=()):
        if not query.lstrip().upper().startswith('SELECT') or ';' in query:raise ValueError('Assertions use SELECT only')
        # SQL is SELECT-only and restricted to this run's ready.json database.
        for value in args:
            literal=str(value) if isinstance(value,int) else "'"+str(value).replace("\\","\\\\").replace("'","''")+"'"
            query=query.replace('%s',literal,1)
        command=['docker','exec',self.container,'sh','-c','MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot --default-character-set=utf8mb4 --batch "$1" -e "$2"','_',self.database,query]
        output=self.subprocess.check_output(command).decode('utf-8').splitlines()
        if not output:return []
        keys=output[0].split('\t');rows=[]
        for line in output[1:]:
            values=line.split('\t');rows.append({k:(None if v=='NULL' else int(v) if re.fullmatch(r'-?\d+',v) else re.sub(r'\\([ntr0\\])',lambda m:{'n':'\n','t':'\t','r':'\r','0':'\0','\\':'\\'}[m[1]],v)) for k,v in zip(keys,values)})
        return rows

    def raw(self,method,path,role=None,value=None,body=None,content_type=None,base=None):
        headers={}
        if role:
            headers['Authorization']='Bearer '+self.credentials.get(role,role)
        if value is not None:body=json.dumps(value,ensure_ascii=False).encode();content_type='application/json'
        if content_type:headers['Content-Type']=content_type
        req=request.Request((base or self.base)+parse.quote(path,safe='/?=&%'),data=body,headers=headers,method=method)
        try:return self.opener.open(req,timeout=20)
        except error.HTTPError as failure:return failure

    def call(self,method,path,role=None,value=None,expected=200,**kwargs):
        with self.raw(method,path,role,value,**kwargs) as response:
            code=response.status;body=response.read();headers=response.headers
        self.check(code==expected,f'{method} {path.split("?")[0]} HTTP {expected}')
        if expected>=400:
            if path.startswith('/api/'):
                data=json.loads(body);self.check(isinstance(data.get('error'),str),'error:string');return data
            return body
        if not body:return None
        if 'json' in headers.get('Content-Type',''):return json.loads(body)
        return body

    def control(self,**flags):
        if 'dialogue_failure' in flags:flags={'mode':'connection_fail'}
        value=self.call('POST' if flags else 'GET','/__phase4/control',value=flags if flags else None)
        value['dialogues']=value['models'];return value

    def reset(self):self.control(mode='normal')

    def login(self,role):
        result=self.call('POST','/api/auth/login',value={'username':self.usernames[role],'password':self.password})
        self.credentials[role]=result['access_token'];return result

    def upload(self,role,text):
        boundary='fixture-'+uuid.uuid4().hex
        body=(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{self.namespace}-{role}.md"\r\nContent-Type: text/plain\r\n\r\n'.encode()+text.encode()+f'\r\n--{boundary}--\r\n'.encode())
        return self.call('POST','/api/materials',role,expected=201,body=body,content_type='multipart/form-data; boundary='+boundary)

    def skill_source(self,group='a',name=None):
        return '\u540d\u79f0:'+ (name or self.namespace+'-'+group+'-\u89e3\u9898\u5f15\u5bfc')+'\n\u9002\u7528\u6761\u4ef6:'+('\u661f\u6cb3,\u65b9\u7a0b' if group=='a' else '\u6d77\u98ce')+'\n\u542f\u7528:\u662f\n\n\u5e2e\u52a9\u5b66\u751f\u7406\u89e3\u95ee\u9898\u3002'

    def agent_input(self,group='a',**changes):
        data={'name':self.namespace+'-'+group,'prompt':'教师原则：耐心分析已知和目标。','enabled':True,'skill_ids':[self.skills[group]],'material_ids':[self.materials[group]]}
        data.update(changes);return data

    def create(self,role='a',agent=None):
        return self.call('POST','/api/conversations',role,{'agent_id':agent or self.agents['b' if role=='b' else 'a']},expected=201)['id']

    def bootstrap(self):
        self.reset()
        for role in self.usernames:
            self.login(role);self.users[role]=self.call('GET','/api/auth/me',role)['user_id']
        self.skills={};self.materials={};self.agents={};self.texts={'a':('星河 '+self.namespace+' 教材：等式变换先分析已知与目标。')*80,'b':('海风 '+self.namespace+' 教材：实验操作先核对条件。')*80}
        for group,teacher in [('a','ta'),('b','tb')]:
            skill=self.call('POST','/api/skills',teacher,{'source':self.skill_source(group)},expected=201);self.skills[group]=skill['id']
            self.call('PUT','/api/skills/'+str(skill['id']),teacher,{'enabled':True})
            uploaded=self.upload(teacher,self.texts[group]);self.check(uploaded['index_status']=='ready','fixture index ready')
            self.materials[group]=uploaded['material_id']
            self.agents[group]=self.call('POST','/api/agents',teacher,self.agent_input(group),expected=201)['id']
        self.conversations={role:self.create(role) for role in ['a','a2','b']}
        self.assertions=[]

    def stream(self,conversation,content,role='a',first=None,disconnect=False):
        start=time.monotonic();events=[];data=[];kind='';first_at=None
        response=self.raw('POST',f'/api/conversations/{conversation}/messages',role,{'content':content})
        self.check(response.status==200,'SSE HTTP200')
        self.check(response.headers.get('Content-Type','').startswith('text/event-stream'),'SSE content-type')
        try:
            for line in response:
                line=line.decode('utf-8').rstrip('\r\n')
                if line.startswith('event:'):kind=line[6:].strip()
                elif line.startswith('data:'):data.append(line[5:].lstrip())
                elif not line and data:
                    payload=json.loads('\n'.join(data));events.append((kind,payload));data=[]
                    if kind=='delta' and first_at is None:
                        first_at=time.monotonic()-start
                        if first:first(payload,events)
                        if disconnect:break
        finally:response.close()
        self.timings.setdefault('streams',[]).append({'conversation_id':conversation,'first_delta_seconds':first_at,'end_seconds':time.monotonic()-start,'terminal':events[-1][0] if events else 'none'})
        return events

    def assistant(self,conversation):
        return self.sql('SELECT * FROM conversation_messages WHERE conversation_id=%s AND role="assistant" ORDER BY sequence_number DESC',(conversation,))[0]

    def wait_state(self,conversation,state):
        until=time.monotonic()+5
        while time.monotonic()<until:
            result=self.assistant(conversation)
            if result['status']==state:return result
            time.sleep(.1)
        raise AssertionError('message state did not settle to '+state)

    def close(self):pass


def standalone(run):
    context_type=Context
    if os.environ.get('APITEST_SUITE')=='week6':
        from apitest.week6_support import Context as context_type
    context=context_type()
    try:context.bootstrap();run(context);print('PASS',len(context.assertions),'assertions')
    finally:context.reset();context.close()
