"""Real Edge -> unchanged Nginx -> Go/MySQL/Qdrant -> independent MCP.

Only random isolated accounts/resources are accepted. Credentials and request
bodies are deliberately excluded from persisted network evidence.
"""
import json
import os
from pathlib import Path
import time
import traceback
from urllib.parse import urlparse
import urllib.request

from playwright.sync_api import sync_playwright, expect


def main():
    root = Path(os.environ['W6_BROWSER_RESULT']).parent
    result = Path(os.environ['W6_BROWSER_RESULT'])
    ready = json.loads((root/'ready.json').read_text(encoding='utf-8'))
    origin = os.environ['W6_BROWSER_ORIGIN']
    parsed = urlparse(origin)
    assert parsed.hostname == '127.0.0.1' and parsed.port not in (80,443,8080,8081)
    password, token = os.environ['W6_BROWSER_PASSWORD'], os.environ['MCP_AUTH_TOKEN']
    checks, requests, errors, timings = [], [], [], []
    report = dict(checks=checks, requests=requests, page_errors=errors, timings=timings,
                  origin=origin, database=ready['database'], collection=ready['collection'],
                  real_paid_model_calls=0, browser='Edge', passed=False)

    def save():
        content = json.dumps(report, ensure_ascii=False, indent=2)
        assert password not in content and token not in content
        result.write_text(content,encoding='utf-8')

    def record(name):
        checks.append(dict(name=name,status='passed')); save()
        print('PASS '+name,flush=True)

    def control(data=None):
        req = urllib.request.Request(origin+'/__phase5MCP/control',data=None if data is None else json.dumps(data).encode(),headers={'Content-Type':'application/json'})
        with urllib.request.urlopen(req,timeout=8) as response:
            return json.load(response)

    try:
      with sync_playwright() as pw:
        browser = pw.chromium.launch(channel='msedge',headless=True)
        report['browser_version'] = browser.version

        def login(who):
            context = browser.new_context(viewport=dict(width=1440,height=1000),accept_downloads=True)
            page = context.new_page()
            page.on('pageerror',lambda error: errors.append(str(error).replace(token,'<redacted>').replace(password,'<redacted>')))
            def note(response):
                if '/api/' in response.url:
                    req = response.request
                    headers = req.all_headers()
                    requests.append(dict(method=req.method,path=urlparse(req.url).path,status=response.status,
                                         bearer=headers.get('authorization','').startswith('Bearer '),cookie='cookie' in headers))
            page.on('response',note)
            page.goto(origin+'/login')
            expect(page.locator('.login-logo img')).to_be_visible()
            page.locator('[name=username]').fill(ready['users'][who])
            page.locator('[name=password]').fill(password)
            page.get_by_role('button',name='登录',exact=True).click()
            page.wait_for_url('**/materials')
            return context,page

        def api(page,method,path,data=None):
            return page.evaluate('''async ([method,path,data]) => {
              const response=await CampusAPI.request(path,{method,...(data===null?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(data)})});
              return {status:response.status,data:response.status===204?null:await response.json()};
            }''',[method,path,data])

        tc,t = login('teacher')
        t.goto(origin+'/tutor'); expect(t.locator('#teacher')).to_be_visible()
        expect(t.locator('#agent-materials input')).to_have_count(16)
        t.locator('#mcp-tab').click()
        expect(t.locator('#mcp-servers')).to_contain_text('暂无 MCP 服务')
        expect(t.locator('#mcp-audits')).to_contain_text('暂无工具调用记录')
        record('teacher MCP tab, empty lists, existing assistant/material bindings')

        def create(name,credential=token,base='http://mcp-demo:8090'):
            t.locator('#mcp-new').click()
            t.locator('#mcp-name').fill(name); t.locator('#mcp-url').fill(base); t.locator('#mcp-token').fill(credential)
            with t.expect_response(lambda r:urlparse(r.url).path=='/api/mcp/servers' and r.request.method=='POST') as event:
                t.locator('#mcp-save').click()
            response=event.value
            assert response.status==201, 'create did not succeed'
            value=response.json()
            assert token not in json.dumps(value) and set(value)=={'id','name','base_url','connected','has_auth','tool_count'}
            expect(t.locator('#mcp-editor')).not_to_be_visible()
            expect(t.locator('#mcp-token')).to_have_value('')
            return value['id']

        sid=create('math-demo')
        card=t.locator(f'[data-server-id="{sid}"]')
        expect(card).to_contain_text('已配置凭据')
        card.get_by_role('button',name='编辑',exact=True).click()
        expect(t.locator('#mcp-token')).to_have_value('')
        t.locator('#mcp-name').fill('math-renamed'); t.locator('#mcp-save').click()
        expect(t.locator('#mcp-editor')).not_to_be_visible(); expect(card).to_contain_text('math-renamed')
        card.get_by_role('button',name='连接',exact=True).click()
        expect(card).to_contain_text('已连接 · 工具 2')
        card.get_by_role('button',name='查看工具').click()
        expect(card.locator('summary')).to_have_count(2)
        expect(card).to_contain_text('formula_hint'); expect(card).to_contain_text('unit_convert')
        record('create, rename, blank-token retention verified by real MCP connect/discovery')
        t.screenshot(path=str(root/'teacher-connected-1440.png'),full_page=True)

        disposable=create('discard-me')
        t.once('dialog',lambda dialog:dialog.accept())
        t.locator(f'[data-server-id="{disposable}"]').get_by_role('button',name='删除',exact=True).click()
        expect(t.locator(f'[data-server-id="{disposable}"]')).to_have_count(0)
        bad=create('bad-auth','deliberately-wrong-test-credential')
        with t.expect_response(lambda r:urlparse(r.url).path==f'/api/mcp/servers/{bad}/connect' and r.request.method=='POST') as bad_connection:
            t.locator(f'[data-server-id="{bad}"]').get_by_role('button',name='连接',exact=True).click()
        expect(t.locator('#mcp-notice')).to_contain_text('连接工具服务失败')
        assert bad_connection.value.status==502
        assert 'deliberately-wrong-test-credential' not in t.locator('body').inner_text()
        record('unaudited delete 204; real remote Bearer rejection 502 safely displayed')

        # Browser transport fault injection is for UI branches only, never claimed
        # as real backend failure evidence; actual 502/503 are tested separately.
        for code in (400,403,404,503):
            t.route('**/api/mcp/servers',lambda route,request,code=code:route.fulfill(status=code,json={'error':'INTERNAL-ADDRESS-TOKEN-MUST-NOT-BE-SHOWN'}))
            t.locator('#mcp-refresh').click()
            expect(t.locator('#mcp-notice')).not_to_have_text('正在读取服务…')
            assert 'INTERNAL-ADDRESS' not in t.locator('#mcp-notice').inner_text()
            t.unroute('**/api/mcp/servers')
        t.route('**/api/mcp/servers',lambda route:route.abort())
        t.locator('#mcp-refresh').click(); expect(t.locator('#mcp-notice')).to_contain_text('网络不可用')
        t.unroute('**/api/mcp/servers')
        t.route('**/api/mcp/servers',lambda route:route.fulfill(status=200,body='not-json'))
        t.locator('#mcp-refresh').click(); expect(t.locator('#mcp-notice')).to_contain_text('网络不可用')
        t.unroute('**/api/mcp/servers')
        # Exercise the real 15s browser timeout once; keep requests pending.
        t.route('**/api/mcp/servers',lambda route:None)
        t.locator('#mcp-refresh').click(); expect(t.locator('#mcp-notice')).to_contain_text('请求超时',timeout=20000)
        t.unroute('**/api/mcp/servers')
        t.locator('#mcp-refresh').click(); expect(card).to_be_visible()
        record('UI 400/403/404/503, network, malformed-response and 15-second timeout branches')

        # Required fields and credential clearing on a failed write.
        t.locator('#mcp-new').click(); t.locator('#mcp-name').fill('invalid name')
        assert not t.locator('#mcp-name').evaluate('(e)=>e.checkValidity()')
        t.locator('#mcp-name').fill('bad-url'); t.locator('#mcp-url').fill('http://user:pass@mcp-demo:8090')
        t.locator('#mcp-save').click(); expect(t.locator('#mcp-form-error')).to_contain_text('请检查名称及 URL')
        t.locator('#mcp-url').fill('http://mcp-demo:8090'); t.locator('#mcp-token').fill(token)
        t.route('**/api/mcp/servers',lambda route:route.fulfill(status=503,json={'error':'secret-remote-error'}))
        t.locator('#mcp-save').click(); expect(t.locator('#mcp-form-error')).to_contain_text('服务暂不可用')
        expect(t.locator('#mcp-token')).to_have_value(''); t.unroute('**/api/mcp/servers'); t.locator('#mcp-cancel').click()
        record('form validation; password never refilled and cleared after failed submission')

        source='名称：数学工具\n适用条件：星河,方程,换算\n工具：formula_hint@math-renamed、unit_convert@math-renamed\n启用：是\n\n帮助学生理解下一步。'
        t.locator('#config-tab').click(); t.locator('#new-skill').click(); t.locator('#skill-source').fill(source); t.locator('#save-skill').click()
        expect(t.locator('#notice')).to_contain_text('技能已保存')
        sk=next(s for s in api(t,'GET','/api/skills')['data']['skills'] if s['name']=='数学工具')
        t.locator('#edit-agent').select_option(str(ready['agent_id']))
        [checkbox.uncheck() for checkbox in t.locator('#agent-skills input').all()]; t.locator(f'#agent-skills input[value="{sk["id"]}"]').check()
        t.locator('#config button').click(); expect(t.locator('#notice')).to_contain_text('助手已保存')
        toggle=t.locator(f'[data-skill-toggle="{sk["id"]}"]')
        toggle.uncheck(); expect(toggle).not_to_be_checked(); expect(t.locator('#notice')).to_contain_text('技能状态已保存')
        toggle=t.locator(f'[data-skill-toggle="{sk["id"]}"]'); toggle.check(); expect(t.locator('#notice')).to_contain_text('技能状态已保存')
        record('existing skill source CRUD/toggle and assistant binding via teacher UI')

        sc,s=login('student'); s.goto(origin+'/tutor'); expect(s.locator('#student')).to_be_visible(); expect(s.locator('#teacher')).not_to_be_visible()
        assert api(s,'GET','/api/mcp/servers')['status']==403
        s.locator('#agent').select_option(str(ready['agent_id'])); s.locator('#create').click(); expect(s.locator('#send-button')).to_be_enabled()
        current=api(s,'GET','/api/conversations')['data']['conversations'][0]['id']

        def send(question,expected,tools=2,mode='normal'):
            control({'mode':mode})
            before=control()['tool_runs']
            s.locator('#question').fill(question); started=time.monotonic(); s.locator('#send-button').click()
            if mode=='hold_open':
                expect(s.locator('.message').last.locator('.tool-call')).to_have_count(tools)
                expect(s.locator('.message').last.locator(':scope > p')).to_have_text('')
                timings.append({'browser_tool_meta_visible_ms':round((time.monotonic()-started)*1000,2),'model_gate_still_closed':True})
                control({'release_open':True,'release_tail':True})
            expect(s.locator('#notice')).to_have_text('回答已保存。',timeout=20000)
            expect(s.locator('#send-button')).to_be_enabled()
            last=s.locator('.message').last
            if tools:
                expect(last.locator('.tool-call')).to_have_count(tools)
                expect(last.locator('.tool-call').first).to_have_attribute('data-status',expected)
            else: expect(last.locator('.tool-calls')).not_to_be_visible()
            state=control()
            if tools:
                assert 0<state['tool_started_ns']<=state['tool_finished_ns']<=state['meta_flushed_ns']<state['open_invoked_ns']<=state['first_delta_ns']<=state['done_ns']
            timings.append({k:state[k] for k in ('tool_started_ns','tool_finished_ns','meta_flushed_ns','open_invoked_ns','first_delta_ns','done_ns')})
            return last,state['tool_runs']-before

        last,runs=send('星河方程换算：2 m to cm','ok',mode='hold_open'); assert runs==1
        expect(last.locator('.actual-sources')).to_contain_text('[1]')
        assert not last.locator('.tool-calls button').count()
        record('real two-tool SSE, tool finish before meta/model/delta/done; actual RAG citations separate')
        s.screenshot(path=str(root/'student-tools-1440.png'),full_page=True)
        history=api(s,'GET',f'/api/conversations/{current}')['data']['messages']
        snapshot=history[-1]['configuration_snapshot']['tool_calls']; assert len(snapshot)==2
        n=control()['tool_runs']; s.reload(); s.locator('#conversations button').first.click()
        expect(s.locator('.message').last.locator('.tool-call')).to_have_count(2)
        assert control()['tool_runs']==n
        record('history reload restores safe snapshot with zero repeated MCP execution')

        t.locator('#mcp-tab').click(); t.locator('#mcp-refresh').click(); expect(card).to_contain_text('已连接')
        card.get_by_role('button',name='断开',exact=True).click(); expect(card).to_contain_text('已断开 · 工具 0')
        send('星河方程换算：3 m to cm','denied')
        t.locator('#mcp-audit-refresh').click(); expect(t.locator('#mcp-audits')).to_contain_text('已拒绝')
        t.once('dialog',lambda dialog:dialog.accept())
        with t.expect_response(lambda r:urlparse(r.url).path==f'/api/mcp/servers/{sid}' and r.request.method=='DELETE') as audited_deletion:
            card.get_by_role('button',name='删除',exact=True).click()
        expect(t.locator('#mcp-notice')).to_contain_text('已有审计'); expect(card).to_be_visible()
        assert audited_deletion.value.status==503
        t.locator('#mcp-limit').select_option('100'); expect(t.locator('#mcp-audits')).to_contain_text('formula_hint@math-renamed')
        record('disconnect real service; denied audit; audited delete 503 retains card; audit 50/100')

        # Wrong credentials still allow valid discovery only after restoring;
        # to obtain tools/call error, keep discovered cache and stop MCP below
        # via the runner-owned control file, never an existing Docker service.
        card.get_by_role('button',name='连接',exact=True).click(); expect(card).to_contain_text('已连接 · 工具 2')
        (root/'request-mcp-stop').write_text('owned MCP only',encoding='utf-8')
        for _ in range(100):
            if (root/'mcp-stopped').exists(): break
            time.sleep(.1)
        assert (root/'mcp-stopped').exists()
        send('星河方程换算：4 m to cm','error')
        record('stopped owned MCP -> error display; model stream still completes without clearing JWT')
        (root/'request-mcp-start').write_text('owned MCP only',encoding='utf-8')
        for _ in range(150):
            if (root/'mcp-started').exists(): break
            time.sleep(.1)
        assert (root/'mcp-started').exists()

        s.locator('#agent').select_option(str(ready['empty_agent_id'])); s.locator('#create').click(); expect(s.locator('#send-button')).to_be_enabled()
        send('没有绑定工具的问题','',0)
        expect(s.locator('.message').last.locator('.tool-calls')).not_to_be_visible()
        # Legacy history has no tool snapshot; the UI must not invent one.
        api(s,'GET','/api/conversations/40')
        s.reload(); s.locator('#conversations button').filter(has_text='old title').click()
        expect(s.locator('.tool-calls:visible')).to_have_count(0)
        record('unbound and legacy history omit empty tool region and never replay tools')

        # Test renderer as an untrusted-data boundary, then persist a browser-only
        # intercepted history response to cover old null/missing snapshots too.
        for snapshot in (None,{}, {'tool_calls':None},{'tool_calls':[]}):
            rendered=s.evaluate('''snapshot=>{const n=document.createElement('div');CampusMCP.renderCalls(n,snapshot?.tool_calls);return {hidden:n.hidden,children:n.childElementCount};}''',snapshot)
            assert rendered=={'hidden':True,'children':0}
        malicious='<img src=x onerror="window.pwned=true">𠮷'+('长文本'*180)
        rendered=s.evaluate('''text=>{const n=document.createElement('div');CampusMCP.renderCalls(n,[{ref:'formula_hint@math',status:'ok',text},{ref:'unit_convert@math',status:'denied',text:'不可用'},{ref:'x@y',status:'error',text:'安全失败'}]);document.querySelector('#messages').append(n);return {images:n.querySelectorAll('img').length,text:n.textContent,statuses:[...n.querySelectorAll('.tool-call')].map(n=>n.dataset.status)};}''',malicious)
        assert rendered['images']==0 and malicious in rendered['text'] and rendered['statuses']==['ok','denied','error']
        assert not s.evaluate('Boolean(window.pwned)')
        record('pure-text HTML injection defense; missing/null/empty legacy tool snapshots')

        # Long data remains real server configuration/audit responses; no fake UI.
        longid=create('long-service-'+('x'*51))
        # A long-URL view is an explicit browser fixture: outbound policy correctly
        # rejects arbitrary paths and hosts, so do not weaken it for layout tests.
        views=api(t,'GET','/api/mcp/servers')['data']
        next(v for v in views['servers'] if v['id']==longid)['base_url']='https://'+('longhostname.'*16)+'example.test:8090'
        t.route('**/api/mcp/servers',lambda route:route.fulfill(status=200,json=views))
        t.locator('#mcp-refresh').click(); expect(t.locator(f'[data-server-id="{longid}"]')).to_contain_text('longhostname')
        long_audit={'id':999,'tool_ref':'formula_hint@'+('a'*32),'status':'ok','created_at':'2026-10-19T00:00:00Z','user_id':1,'conversation_id':1,'detail':'长审计文本'*40+'<img src=x onerror=alert(1)>'}
        t.route('**/api/mcp/tool-calls*',lambda route:route.fulfill(status=200,json={'calls':[long_audit]}))
        t.locator('#mcp-audit-refresh').click(); expect(t.locator('#mcp-audits')).to_contain_text('长审计文本')
        assert t.locator('#mcp-audits img').count()==0
        t.locator(f'[data-server-id="{longid}"]').get_by_role('button',name='编辑',exact=True).click()
        for width in (1440,1024,768,390):
            t.set_viewport_size(dict(width=width,height=950))
            assert t.evaluate('document.documentElement.scrollWidth<=innerWidth'), f'teacher overflow {width}'
            box=t.locator('#mcp-editor').bounding_box(); assert box['x']>=0 and box['x']+box['width']<=width+1
            t.screenshot(path=str(root/f'teacher-dialog-{width}.png'),full_page=True)
            s.set_viewport_size(dict(width=width,height=950))
            assert s.evaluate('document.documentElement.scrollWidth<=innerWidth'), f'student overflow {width}'
            if width==390: assert not s.locator('#conversation-disclosure').evaluate('(e)=>e.open')
            s.screenshot(path=str(root/f'student-wrap-{width}.png'),full_page=True)
        t.locator('#mcp-cancel').click(); t.unroute('**/api/mcp/tool-calls*'); t.unroute('**/api/mcp/servers')
        record('1440/1024/768/390 no horizontal overflow, long URL/name/tool text, dialog and mobile collapse')

        # Real fifth-lesson knowledge UI, pagination, all three retrieval modes.
        s.goto(origin+'/knowledge'); expect(s.locator('#lecture-cards > button')).to_have_count(9)
        s.locator('#next-page').click(); expect(s.locator('#page-status')).to_contain_text('第 2 / 2 页')
        for mode in ('keyword','vector','hybrid'):
            s.locator('#mode').select_option(mode); s.locator('#question').fill('星河方程')
            s.locator('#search-button').click(); expect(s.locator('#search-button')).to_be_enabled()
            expect(s.locator('#sources')).to_contain_text('星河方程')
            expect(s.locator('#sources .knowledge-result').first).to_be_visible()
        t.goto(origin+'/materials'); expect(t.locator('#materials')).to_be_visible()
        s.screenshot(path=str(root/'knowledge-mobile.png'),full_page=True)
        record('Week5 material UI and knowledge pagination + keyword/vector/hybrid real Qdrant retrieval')

        fc,f=login('foreign_teacher'); f.goto(origin+'/tutor'); expect(f.locator('#teacher')).to_be_visible(); f.locator('#mcp-tab').click()
        expect(f.locator('#mcp-servers')).to_contain_text('暂无 MCP 服务')
        owner_before=api(t,'GET','/api/mcp/servers')['data']
        audits_before=api(t,'GET','/api/mcp/tool-calls')['data']
        for method,suffix in [('PUT',''),('DELETE',''),('POST','/connect'),('POST','/disconnect'),('GET','/tools')]:
            assert api(f,method,f'/api/mcp/servers/{sid}'+suffix,{'name':'foreign','base_url':'http://mcp-demo:8090'} if method=='PUT' else None)['status']==404
        assert api(f,'GET','/api/mcp/tool-calls')['data']['calls']==[]
        assert api(t,'GET','/api/mcp/servers')['data']==owner_before
        assert api(t,'GET','/api/mcp/tool-calls')['data']==audits_before
        anonymous=browser.new_context(); anon=anonymous.new_page(); anon.goto(origin+'/tutor'); anon.wait_for_url('**/login')
        response=anonymous.request.get(origin+'/api/mcp/servers'); assert response.status==401
        record('anonymous 401, student 403, foreign class lists/audits empty and resources 404')

        # Swagger UI Authorize and real Execute for connect/tools/skill-save.
        t.goto(origin+'/tutor'); expect(t.locator('#teacher')).to_be_visible()
        jwt=t.evaluate("sessionStorage.getItem('campusclaw.token')")
        doc=tc.new_page(); doc.goto(origin+'/swagger/'); expect(doc.locator('.swagger-ui')).to_be_visible()
        doc.locator('button.authorize').first.click(); doc.locator('.auth-container input').fill(jwt)
        doc.locator('.auth-container button.authorize').click(); doc.get_by_role('button',name='Close',exact=True).click()
        doc.locator('input').evaluate_all("nodes=>nodes.forEach(n=>{if(n.type==='text'&&n.value.startsWith('eyJ'))n.value=''})")
        def execute(path,method,body=None):
            op=doc.locator('.opblock').filter(has=doc.locator(f'.opblock-summary-path[data-path="{path}"]')).filter(has=doc.locator('.opblock-summary-method',has_text=method.upper()))
            expect(op).to_have_count(1)
            op.locator('.opblock-summary').click(); op.get_by_role('button',name='Try it out',exact=True).click()
            if '{id}' in path: op.locator('input[placeholder="id"]').fill(str(sid))
            if body is not None: op.locator('textarea').fill(json.dumps(body,ensure_ascii=False))
            op.get_by_role('button',name='Execute',exact=True).click()
            expect(op.locator('.live-responses-table tbody .response-col_status').first).to_have_text('200',timeout=20000)
            op.locator('.opblock-summary').click()
        execute('/api/mcp/servers/{id}/connect','POST')
        execute('/api/mcp/servers/{id}/tools','GET')
        # Skill creation is the official POST /api/skills operation (201).
        op=doc.locator('.opblock-post').filter(has=doc.locator('.opblock-summary-path[data-path="/api/skills"]'))
        op.locator('.opblock-summary').click(); op.get_by_role('button',name='Try it out',exact=True).click()
        op.locator('textarea').fill(json.dumps({'source':source.replace('数学工具','Swagger 工具技能')},ensure_ascii=False))
        op.get_by_role('button',name='Execute',exact=True).click(); expect(op.locator('.live-responses-table tbody .response-col_status').first).to_have_text('201',timeout=15000)
        record('actual Swagger Authorize and Execute connect/tools/skill-save')
        student_jwt=s.evaluate("sessionStorage.getItem('campusclaw.token')")
        doc.evaluate('(token)=>CampusSwagger.preauthorizeApiKey("bearerAuth",token)',student_jwt)
        fresh_conversation=api(s,'POST','/api/conversations',{'agent_id':ready['agent_id']})['data']['id']
        doc.locator('#conversation-id').fill(str(fresh_conversation)); doc.locator('#stream-content').fill('星河方程换算：5 m to cm'); doc.locator('#stream-send').click()
        expect(doc.locator('#stream-status')).to_have_text('已完成',timeout=15000)
        expect(doc.locator('#stream-tools-region')).to_be_visible(); expect(doc.locator('#stream-tools')).to_contain_text('formula_hint@math-renamed')
        debug_calls=json.loads(doc.locator('#stream-tools').inner_text()); assert all(set(call)=={'ref','status','text'} for call in debug_calls)
        record('Swagger real SSE panel shows safe optional tool_calls alongside original delta/citations')
        # Swagger renders an authorized curl command with a JWT; logout and clear
        # operation responses before a screenshot, never persist JWT screenshots.
        doc.evaluate("CampusSwagger.authActions.logout(['bearerAuth']);document.querySelectorAll('.responses-wrapper').forEach(n=>n.remove())")
        doc.screenshot(path=str(root/'swagger-after-execute.png'),full_page=True)
        assert token not in t.locator('body').inner_text()
        assert token not in json.dumps(t.evaluate('({...sessionStorage})'))
        assert token not in t.evaluate("[...document.querySelectorAll('input')].map(n=>n.value+' '+n.outerHTML).join('')")
        assert all(not r['cookie'] for r in requests)
        # A 401 response must clear UI state through the shared API wrapper.
        f.route('**/api/mcp/servers',lambda route:route.fulfill(status=401,json={'error':'expired'}))
        f.locator('#mcp-refresh').click(); f.wait_for_url('**/login')
        assert not f.evaluate("sessionStorage.getItem('campusclaw.token')")
        record('credential scan, same-origin Bearer requests/no cookies, shared 401 cleanup')
        assert not errors, 'browser JavaScript errors'
        report['passed']=True; save(); browser.close()
    except Exception as error:
        detail=(str(error)+'\n'+traceback.format_exc()).replace(password,'<redacted>').replace(token,'<redacted>')
        checks.append(dict(name='acceptance aborted',status='failed',detail=detail)); save()
        raise RuntimeError(detail) from None


if __name__=='__main__':
    main()
