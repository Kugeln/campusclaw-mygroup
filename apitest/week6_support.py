"""Week6 adapter. The inherited strict random tmpfs/role/database guards remain."""
import hashlib
import json
import os
from apitest.support import Context as Week5Context


class Context(Week5Context):
    def __init__(self):
        super().__init__()
        info=json.loads(self.subprocess.check_output(['docker','inspect',self.container]))[0]
        labels=info['Config']['Labels']
        if not labels.get('com.docker.compose.project','').startswith('campusclaw-w6-p6-') or labels.get('campusclaw.week6')!='phase6':
            raise RuntimeError('Only a newly owned Phase6 project is allowed')
        self.usernames["ta2"]="ta2"

    def network_count(self):
        value=self.control()
        return sum(value[k] for k in ["real_mcp_calls","real_mcp_initialize","real_mcp_lists"])

    def control(self,**flags):
        if 'dialogue_failure' in flags:flags={'mode':'connection_fail'}
        value=self.call('POST' if flags else 'GET','/__phase6MCP/control',value=flags if flags else None)
        value['dialogues']=value['models'];return value

    def bootstrap(self):
        super().bootstrap()
        self.mcp_servers={};self.cross_checks={};self.mcp_requests=[]

    def fingerprint(self):
        # Hash ciphertext in memory only; never serialize it in reports.
        data=[]
        for table in ['mcp_servers','mcp_tools','mcp_tool_calls','skills','assistants','conversation_messages']:
            # Binary AES-GCM envelopes are not UTF-8. Compare every column, but
            # encode binary fields in the SELECT; never print or persist them.
            columns=('id,class_id,name,base_url,connected,connected_at,created_at,updated_at,revision,op_id,operation_started_at,SHA2(token_cipher,256) AS cipher_hash,HEX(token_nonce) AS nonce_hex,key_id' if table=='mcp_servers' else '*')
            data.append(self.sql('SELECT '+columns+' FROM '+table+' ORDER BY id'))
        return hashlib.sha256(json.dumps(data,sort_keys=True,ensure_ascii=False,default=str).encode()).hexdigest()

    def safe(self,value):
        text=json.dumps(value,ensure_ascii=False)
        for secret in [os.environ['MCP_AUTH_TOKEN'],*self.credentials.values()]:
            self.check(secret not in text,'response excludes generated credential')
        self.check(all(('"'+key+'"') not in text for key in ['auth_token','token_cipher','token_nonce','key_id','revision','op_id']),'response excludes internal credential fields')

    def mcp(self,method,path,role=None,value=None,expected=200):
        result=self.call(method,path,role,value,expected=expected)
        self.safe(result)
        self.mcp_requests.append({'recorded_utc':__import__('datetime').datetime.now(__import__('datetime').timezone.utc).isoformat(),'method':method,'path':path,'role':role,'status':expected,'response':result})
        return result

    def gate(self,method,path,value=None):
        before=self.fingerprint();network=self.network_count()
        for role,code in [(None,401),('a',403),('b',403)]:self.mcp(method,path,role,value,expected=code)
        self.check(self.fingerprint()==before,'401/403 requests have zero database writes')
        self.check(self.network_count()==network,'401/403 requests have zero remote tools/call')

    def mcp_case(self,name):
        if name!='create' and not self.mcp_servers:self.case_create()
        if name in ['tools','disconnect','calls','delete'] and not getattr(self,'mcp_connected',False):self.case_connect()
        if name in ['calls','delete'] and not getattr(self,'mcp_exercised',False):self.case_disconnect()
        getattr(self,'case_'+name)()

    def case_create(self):
        path='/api/mcp/servers';payload={'name':self.namespace+'-math','base_url':'http://app:8092','auth_token':os.environ['MCP_AUTH_TOKEN']}
        self.gate('POST',path,payload)
        for group,teacher in [('a','ta'),('b','tb')]:
            server=self.mcp('POST',path,teacher,payload,201);self.mcp_servers[group]=server
            self.check(server['has_auth'] and not server['connected'] and server['tool_count']==0,'new class-scoped encrypted credential view')
            row=self.sql('SELECT token_cipher IS NOT NULL AS encrypted,token_nonce IS NOT NULL AS nonce FROM mcp_servers WHERE id=%s',(server['id'],))[0]
            self.check(row=={'encrypted':1,'nonce':1},'database stores AES-GCM envelope, not plaintext')
            self.mcp('POST',path,teacher,payload,400)

    def case_list(self):
        self.gate('GET','/api/mcp/servers')
        for group,teacher in [('a','ta'),('b','tb')]:
            rows=self.mcp('GET','/api/mcp/servers',teacher)['servers']
            self.check(any(s['id']==self.mcp_servers[group]['id'] for s in rows),'own server visible')
            self.check(all(s['id']!=self.mcp_servers['b' if group=='a' else 'a']['id'] for s in rows),'other class server invisible')

    def case_update(self):
        for group,teacher in [('a','ta'),('b','tb')]:
            server=self.mcp_servers[group];path='/api/mcp/servers/'+str(server['id'])
            value={'name':server['name'],'base_url':server['base_url'],'auth_token':''}
            self.gate('PUT',path,value)
            changed=self.mcp('PUT',path,teacher,value)
            self.check(changed['has_auth'] and not changed['connected'],'blank credential retained, update disconnected')

    def case_connect(self):
        for group,teacher in [('a','ta'),('b','tb')]:
            path='/api/mcp/servers/'+str(self.mcp_servers[group]['id'])+'/connect'
            self.gate('POST',path)
            result=self.mcp('POST',path,teacher)
            self.check(result['connected'] and {t['name'] for t in result['tools']}=={'formula_hint','unit_convert'},'real initialize/list discover both tools')
        self.mcp_connected=True
        self._isolation()

    def _isolation(self):
        before=self.fingerprint();remote=self.network_count()
        for group,foreign in [('a','tb'),('b','ta')]:
            base='/api/mcp/servers/'+str(self.mcp_servers[group]['id'])
            for method,suffix in [('PUT',''),('DELETE',''),('POST','/connect'),('POST','/disconnect'),('GET','/tools')]:
                self.mcp(method,base+suffix,foreign,{'name':'foreign','base_url':'http://app:8092'} if method=='PUT' else None,404)
        self.check(self.fingerprint()==before,'bidirectional class denial changes no rows')
        self.check(self.network_count()==remote,'cross-class management makes zero remote tools/call')
        same_class=self.mcp('GET','/api/mcp/servers','ta2')['servers']
        self.check(any(row['id']==self.mcp_servers['a']['id'] for row in same_class),'independent same-class teacher can manage class service')
        self.mcp('GET','/api/mcp/servers/'+str(self.mcp_servers['a']['id'])+'/tools','ta2')
        self.cross_checks['A_class_isolation']=True

    def case_tools(self):
        for group,teacher in [('a','ta'),('b','tb')]:
            path='/api/mcp/servers/'+str(self.mcp_servers[group]['id'])+'/tools';self.gate('GET',path)
            before=self.fingerprint();count=self.control()['real_mcp_calls']
            tools=self.mcp('GET',path,teacher)['tools']
            self.check(len(tools)==2 and self.fingerprint()==before and self.control()['real_mcp_calls']==count,'cached tools read has no writes or RPC')
        self.cross_checks['B_has_auth_no_token']=True

    def case_disconnect(self):
        self.mcp_skills={};self.mcp_agents={}
        for group,teacher,student in [('a','ta','a'),('b','tb','b')]:
            server=self.mcp_servers[group];refs='formula_hint@'+server['name']+',unit_convert@'+server['name']
            source='名称：'+self.namespace+'工具'+group+'\n适用条件：星河,海风,换算\n工具：'+refs+'\n启用：是\n\n引导理解，检查单位。'
            sk=self.call('POST','/api/skills',teacher,{'source':source},expected=201);self.mcp_skills[group]=sk['id']
            agent=self.call('POST','/api/agents',teacher,self.agent_input(group,name=self.namespace+'-mcp-'+group,skill_ids=[sk['id']]),expected=201);self.mcp_agents[group]=agent['id']
            # Binding to an other-class-only name is rejected without side effects.
            bad=source.replace(server['name'],'absent-foreign-only')
            before=self.fingerprint();self.call('POST','/api/skills',teacher,{'source':bad},expected=400);self.check(self.fingerprint()==before,'invalid tool binding is atomic')
            cid=self.create(student,agent['id']);remote=self.control()['real_mcp_calls'];events=self.stream(cid,'星河海风换算 1 m转cm',student)
            calls=events[0][1]['tool_calls'];self.check(events[-1][0]=='done' and all(c['status']=='ok' for c in calls),'tools complete before successful teaching SSE')
            self.check(any(c['text']=='1 m = 100 cm' for c in calls),'real network unit_convert 1 m = 100 cm')
            self.check(self.control()['real_mcp_calls']==remote+2,'one real RPC per declared tool, no retries')
            for other in ['a2','b' if student=='a' else 'a']:self.call('GET','/api/conversations/'+str(cid),other,expected=404)
            before=self.control()['real_mcp_calls'];self.call('GET','/api/conversations/'+str(cid),student);self.check(self.control()['real_mcp_calls']==before,'history never replays tools')
            path='/api/mcp/servers/'+str(server['id'])+'/disconnect';self.gate('POST',path);self.mcp('POST',path,teacher)
            old=self.sql('SELECT COUNT(*) AS n FROM mcp_tool_calls WHERE server_id=%s',(server['id'],))[0]['n'];remote=self.control()['real_mcp_calls']
            denied=self.stream(cid,'星河海风换算 1 m转cm',student)
            self.check(denied[-1][0]=='done' and all(c['status']=='denied' for c in denied[0][1]['tool_calls']),'disconnected existing server degrades with denied meta')
            rows=self.sql('SELECT class_id,tool_ref,status FROM mcp_tool_calls WHERE server_id=%s ORDER BY id DESC LIMIT 2',(server['id'],))
            self.check(len(rows)==2 and all(row['class_id']==(1 if group=='a' else 2) and row['status']=='denied' and row['tool_ref'].endswith('@'+server['name']) for row in rows),'denied terminal audit class/ref/status correct')
            self.check(self.sql('SELECT COUNT(*) AS n FROM mcp_tool_calls WHERE server_id=%s',(server['id'],))[0]['n']==old+2,'one denied audit per declared reference')
            self.check(self.control()['real_mcp_calls']==remote,'disconnect denial makes zero actual tool RPC')
            self.mcp('POST',path,teacher);self.mcp('POST',path.replace('/disconnect','/connect'),teacher)
        self.mcp_exercised=True
        self.cross_checks['C_disconnect_denied_audit']=True

    def case_calls(self):
        self.gate('GET','/api/mcp/tool-calls')
        for group,teacher in [('a','ta'),('b','tb')]:
            before=self.fingerprint();rows=self.mcp('GET','/api/mcp/tool-calls',teacher)['calls']
            own=self.mcp_servers[group]['name'];foreign_server=self.mcp_servers['b' if group=='a' else 'a']['id']
            foreign_ids={r['id'] for r in self.sql('SELECT id FROM mcp_tool_calls WHERE server_id=%s',(foreign_server,))}
            self.check(rows and all(r['id'] not in foreign_ids for r in rows),'own audits visible and foreign IDs excluded despite same server name')
            self.check({'ok','denied'}<={r['status'] for r in rows},'audit status matches success/disconnect results')
            self.check(self.fingerprint()==before,'audit query is read-only')
        self.cross_checks['D_class_audit_four_assertions']=True

    def case_delete(self):
        for group,teacher in [('a','ta'),('b','tb')]:
            path='/api/mcp/servers/'+str(self.mcp_servers[group]['id']);self.gate('DELETE',path)
            before=self.fingerprint();self.mcp('DELETE',path,teacher,expected=503);self.check(self.fingerprint()==before,'audited delete 503 keeps all rows')
            server=self.mcp('POST','/api/mcp/servers',teacher,{'name':self.namespace+'-empty-'+group,'base_url':'http://app:8092','auth_token':''},201)
            self.check(not server['has_auth'],'has_auth is false for an unconfigured credential')
            self.mcp('DELETE','/api/mcp/servers/'+str(server['id']),teacher,expected=204)
            self.mcp('DELETE','/api/mcp/servers/'+str(server['id']),teacher,expected=404)
        actors=self.sql('SELECT u.id,u.username,u.role,u.class_id,c.name AS class_name FROM users u JOIN classes c ON c.id=u.class_id ORDER BY u.id')
        ids=','.join(str(server['id']) for server in self.mcp_servers.values())
        audit_rows=self.sql('SELECT id,class_id,server_id,user_id,conversation_id,tool_ref,status,detail,created_at FROM mcp_tool_calls WHERE server_id IN('+ids+') ORDER BY id')
        self.safe(audit_rows)
        from pathlib import Path
        report={'utc':__import__('datetime').datetime.now(__import__('datetime').timezone.utc).isoformat(),'participation':'Codex automated simulated cross verification; no real peers','checks':self.cross_checks,'passed':len(self.cross_checks)==4 and all(self.cross_checks.values()),'roles':self.usernames,'principal_users':self.users,'requests':self.mcp_requests,'database':self.database,'actors':actors,'database_terminal_audits':audit_rows,'remote_rpc_counts':{k:self.control()[k] for k in ['real_mcp_calls','real_mcp_initialize','real_mcp_lists']},'paid_model_calls':0}
        Path(os.environ['APITEST_CROSS_RESULT']).write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
