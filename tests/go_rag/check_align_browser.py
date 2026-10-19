"""Phase 4 only: real Edge -> new Nginx -> Go -> new MySQL/Qdrant fixtures."""
import json
import os
import re
import time
import platform
import subprocess
from datetime import datetime, timezone
import urllib.request
from pathlib import Path
from urllib.parse import urlparse

from playwright.sync_api import sync_playwright, expect


def main():
    origin = os.environ['ALIGN_BROWSER_ORIGIN']
    parsed = urlparse(origin)
    assert parsed.hostname == '127.0.0.1' and parsed.port not in (80, 443, 8080)
    fixture = json.loads(Path(os.environ['ALIGN_BROWSER_READY']).read_text(encoding='utf-8-sig'))
    password = os.environ['ALIGN_BROWSER_PASSWORD']
    result = Path(os.environ['ALIGN_BROWSER_RESULT'])
    checks, requests, errors, timings, dod = [], [], [], {}, {}
    environment = {'origin':origin,'database':fixture['database'],'collection':fixture['collection'],
                   'head':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),
                   'python':platform.python_version(),'model':'controlled local HTTP only'}

    def record(check):
        checks.append(check)
        result.write_text(json.dumps({'checks': checks, 'timings': timings, 'requests': requests,
                                      'page_errors': errors, 'real_model': False,'utc':datetime.now(timezone.utc).isoformat(),
                                      'environment':environment,'dod':dod}, ensure_ascii=False, indent=2), encoding='utf-8')

    def acceptance(key, inputs, outputs, evidence):
        dod[key]={'passed':True,'utc':datetime.now(timezone.utc).isoformat(),'inputs':inputs,
                  'outputs':outputs,'evidence':evidence,'environment':environment.copy()}

    def control(data=None):
        request = urllib.request.Request(origin+'/__phase4/control',
                                         data=None if data is None else json.dumps(data).encode(),
                                         headers={'Content-Type': 'application/json'})
        with urllib.request.urlopen(request, timeout=5) as response:
            return json.load(response)

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(channel='msedge', headless=True)
        environment['browser']=browser.version

        def login(who):
            context = browser.new_context(accept_downloads=True, viewport={'width': 1280, 'height': 900})
            page = context.new_page()
            page.on('pageerror', lambda error: errors.append(type(error).__name__))
            def note(request):
                if '/api/' in request.url:
                    headers = request.all_headers()
                    requests.append({'method': request.method, 'path': urlparse(request.url).path,
                                     'bearer': headers.get('authorization', '').startswith('Bearer '),
                                     'cookie': 'cookie' in headers})
            page.on('request', note)
            page.goto(origin+'/login')
            page.locator('[name=username]').fill(fixture['users'][who])
            page.locator('[name=password]').fill(password)
            page.get_by_role('button', name='登录', exact=True).click()
            page.wait_for_url('**/materials')
            return context, page

        def api(page, method, path, data=None):
            return page.evaluate("""async ([method,path,data]) => {
              const response=await CampusAPI.request(path,{method,...(data===null?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(data)})});
              return {status:response.status,data:response.status===204?null:await response.json()};
            }""", [method, path, data])

        teacher_context, teacher = login('teacher')
        teacher.goto(origin+'/tutor')
        expect(teacher.locator('#teacher')).to_be_visible()
        expect(teacher.locator('#agent-materials input')).to_have_count(16)
        teacher.locator('#new-skill').click()
        source = '# file comment\r\n名称：浏览器解题引导\r\n适用条件：星河,方程\r\n工具：knowledge_search\r\n启用：是\r\n\r\n# 正文保留\r\n只提示下一步。'
        teacher.locator('#skill-file').set_input_files({'name': 'skill.txt', 'mimeType': 'text/plain', 'buffer': source.encode()})
        expect(teacher.locator('#notice')).to_contain_text('已读取文件')
        teacher.locator('#save-skill').click()
        expect(teacher.locator('#notice')).to_contain_text('技能已保存')
        skills = api(teacher, 'GET', '/api/skills')['data']['skills']
        first = next(s for s in skills if s['name'] == '浏览器解题引导')
        assert first['source'] == source, 'unchanged file source lost CRLF'
        teacher.reload()
        teacher.locator(f'button[data-skill-id="{first["id"]}"]').click()
        expect(teacher.locator('#skill-source')).to_have_value(source.replace('\r\n', '\n'))
        edited = source.replace('只提示下一步。', '只提示下一步并检查理解。')
        teacher.locator('#skill-source').fill(edited)
        teacher.locator('#save-skill').click()
        expect(teacher.locator('#notice')).to_contain_text('技能已保存')
        assert '检查理解' in api(teacher, 'GET', f'/api/skills/{first["id"]}')['data']['source']
        toggle = teacher.locator(f'input[data-skill-toggle="{first["id"]}"]')
        toggle.uncheck()
        expect(teacher.locator('#notice')).to_contain_text('技能状态已保存')
        toggled = api(teacher, 'GET', f'/api/skills/{first["id"]}')['data']
        assert not toggled['enabled'] and '启用：否' in toggled['source']
        toggle.check()
        expect(teacher.locator('#notice')).to_contain_text('技能状态已保存')
        teacher.locator('#new-skill').click()
        teacher.locator('#skill-source').fill('名称：浏览器复习\n适用条件：复习\n启用：是\n\n复习说明。')
        teacher.locator('#save-skill').click()
        expect(teacher.locator('#notice')).to_contain_text('技能已保存')
        second = next(s for s in api(teacher, 'GET', '/api/skills')['data']['skills'] if s['name'] == '浏览器复习')
        teacher.locator('#edit-agent').select_option(str(fixture['agent_id']))
        teacher.locator('#prompt').fill('')
        for item in teacher.locator('#agent-skills input').all():
            item.set_checked(int(item.get_attribute('value')) in (first['id'], second['id']))
        for item in teacher.locator('#agent-materials input').all():
            item.set_checked(item.get_attribute('value') in fixture['materials'][:2])
        teacher.get_by_role('button', name='保存助手', exact=True).click()
        expect(teacher.locator('#notice')).to_contain_text('助手已保存')
        teacher.reload()
        teacher.locator('#edit-agent').select_option(str(fixture['agent_id']))
        expect(teacher.locator('#prompt')).to_have_value('')
        expect(teacher.locator('#agent-skills input:checked')).to_have_count(2)
        expect(teacher.locator('#agent-materials input:checked')).to_have_count(2)
        expect(teacher.locator('#empty-material-note')).to_contain_text('不检索任何材料')
        teacher.locator('#edit-agent').select_option('')
        teacher.locator('#agent-name').fill('页面空材料助手')
        teacher.get_by_role('button', name='保存助手', exact=True).click()
        expect(teacher.locator('#notice')).to_contain_text('助手已保存')
        empty = next(a for a in api(teacher, 'GET', '/api/agents')['data']['agents'] if a['name'] == '页面空材料助手')
        assert empty['prompt'] == '' and empty['skill_ids'] == [] and empty['material_ids'] == []
        fixture['empty_agent_id'] = empty['id']
        teacher.locator('#new-skill').click()
        teacher.locator('#skill-source').fill('缺少元数据的说明')
        teacher.locator('#save-skill').click()
        expect(teacher.locator('#notice')).not_to_contain_text('已保存')
        expect(teacher.locator('#notice')).not_to_be_empty()
        teacher.locator(f'button[data-skill-id="{second["id"]}"]').click()
        teacher.once('dialog', lambda dialog: dialog.accept())
        teacher.locator('#delete-skill').click()
        expect(teacher.locator('#notice')).to_contain_text('技能已删除')
        saved = api(teacher, 'GET', f'/api/agents/{fixture["agent_id"]}')['data']
        assert saved['skill_ids'] == [first['id']] and saved['prompt'] == ''
        teacher.set_viewport_size({'width': 390, 'height': 844})
        assert teacher.evaluate('document.documentElement.scrollWidth <= innerWidth')
        teacher.screenshot(path=str(result.parent/'teacher-mobile.png'), full_page=True)
        teacher.set_viewport_size({'width': 1280, 'height': 900})
        record('4.1 teacher file/source CRUD, CRLF round trip, edit, delete/unbind, old toggle, two bindings, empty role, reload and mobile')

        student_context, page = login('student')
        page.goto(origin+'/tutor')
        expect(page.locator('#teacher')).to_be_hidden()
        expect(page.locator('#student')).to_be_visible()
        assert api(page, 'POST', '/api/skills', {'source': source})['status'] == 403
        assert api(page, 'PUT', f'/api/agents/{fixture["agent_id"]}', {})['status'] == 403
        assert api(page, 'GET', f'/api/agents/{fixture["foreign_agent_id"]}')['status'] == 404
        assert api(page, 'GET', f'/api/materials/{fixture["foreign_material_id"]}')['status'] == 404

        def create(agent=None):
            page.locator('#agent').select_option(str(agent or fixture['agent_id']))
            page.locator('#notice').evaluate("e=>e.textContent=''")
            with page.expect_response(lambda r: r.request.method == 'POST' and urlparse(r.url).path == '/api/conversations') as response:
                page.locator('#create').click()
            expect(page.locator('#notice')).to_have_text('会话已创建。')
            return response.value.json()['id']

        def send(question='星河只引用第二份'):
            page.locator('#question').fill(question)
            page.locator('#send-button').click()

        cid = create()
        before = control({'mode': 'hold_open'})
        send()
        draft = page.locator('.message[data-state=generating]').last
        expect(draft.locator('.candidate-sources button')).to_have_count(4)
        expect(draft.locator('.state')).to_contain_text('浏览器解题引导')
        expect(draft.locator('p')).to_be_empty()
        gated = control()
        assert gated['models'] == before['models'] and 0 < gated['meta_flushed_ns'] <= gated['open_invoked_ns']
        control({'release_open': True})
        expect(draft.locator('p')).to_have_text('先分析𠮷条件，')
        assert control()['completed'] == before['completed']
        page.screenshot(path=str(result.parent/'student-incremental.png'), full_page=True)
        control({'release_tail': True})
        completed = page.locator('.message[data-state=completed]').last
        expect(completed.locator('.actual-sources button')).to_have_count(2)
        expect(completed.locator('.actual-sources button').first).to_contain_text('[2]')
        history = api(page, 'GET', f'/api/conversations/{cid}')['data']['messages']
        assert history[-1]['content'] == completed.locator('p').inner_text() and history[-1]['citations'][0]['number'] == 2
        timings.update({k: control()[k] for k in ('meta_flushed_ns', 'open_invoked_ns', 'first_delta_ns', 'done_ns')})
        assert timings['meta_flushed_ns'] <= timings['open_invoked_ns'] < timings['first_delta_ns'] < timings['done_ns']
        completed.locator('.actual-sources button').first.click()
        expect(page.locator('#source-text')).to_contain_text('星河方程')
        with page.expect_download() as download:
            completed.locator('.actual-sources').get_by_role('button', name='下载材料', exact=True).click()
        filename = download.value.suggested_filename
        assert filename == history[-1]['citations'][0]['title'], ascii(filename)
        page.set_viewport_size({'width': 390, 'height': 844})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
        page.screenshot(path=str(result.parent/'student-mobile.png'), full_page=True)
        page.set_viewport_size({'width': 1280, 'height': 900})
        record('4.2/4.4 proxy meta before model connection, first delta before completion, candidate skill/sources, final number=2, DB equality and attachment download')

        # Teacher controls affect the next round, including the existing conversation.
        control({'mode': 'normal'})
        teacher.goto(origin+'/tutor')
        teacher.locator('#edit-agent').select_option(str(fixture['agent_id']))
        teacher.locator('#agent-enabled').uncheck()
        teacher.get_by_role('button', name='保存助手', exact=True).click()
        expect(teacher.locator('#notice')).to_contain_text('助手已保存')
        assert api(page, 'POST', '/api/conversations', {'agent_id': fixture['agent_id']})['status'] == 404
        expect(page.locator('#send-button')).to_be_enabled()
        send('星河停用后继续')
        expect(page.locator('.message[data-state=completed] .actual-sources')).to_have_count(2)
        teacher.locator('#agent-enabled').check()
        for item in teacher.locator('#agent-materials input').all():
            item.uncheck()
        teacher.get_by_role('button', name='保存助手', exact=True).click()
        expect(teacher.locator('#notice')).to_contain_text('助手已保存')
        expect(page.locator('#send-button')).to_be_enabled()
        before_unbound = control()
        send('星河解绑最后材料')
        expect(page.locator('.message[data-state=completed] .actual-sources')).to_have_count(3)
        expect(page.locator('.message[data-state=completed] .candidate-sources').last).to_contain_text('无候选来源')
        assert control()['models'] == before_unbound['models']+1 and control()['embeddings'] == before_unbound['embeddings']
        for item in teacher.locator('#agent-materials input').all():
            item.set_checked(item.get_attribute('value') in fixture['materials'][:2])
        teacher.get_by_role('button', name='保存助手', exact=True).click()
        expect(teacher.locator('#notice')).to_contain_text('助手已保存')
        record('4.1 browser disable blocks new conversations but existing continues; last-material unbind produces zero retrieval next turn; explicit rebind restored')

        control({'mode': 'normal'})
        create(fixture['empty_agent_id'])
        before = control()
        send('无材料如何理解')
        expect(page.locator('.message[data-state=completed] .actual-sources')).to_contain_text('无引用来源')
        expect(page.locator('.candidate-sources')).to_contain_text('无候选来源')
        assert '[1]' not in page.locator('.message[data-state=completed] p').last.inner_text()
        assert control()['models'] == before['models']+1 and control()['embeddings'] == before['embeddings']
        for mode, expected_state in [('error', 'failed'), ('connection_fail', 'failed'), ('cancel', 'interrupted'), ('disconnect', 'interrupted'), ('lost_done', 'completed')]:
            control({'mode': mode})
            cid = create()
            send('星河如何分析')
            if mode == 'cancel':
                expect(page.locator('.message[data-state=generating] p')).to_contain_text('先分析')
                page.locator('#cancel').click()
            terminal = page.locator(f'.message[data-state={expected_state}]').last
            expect(terminal).to_be_visible(timeout=10000)
            expect(page.locator('#send-button')).to_be_enabled()
            last = api(page, 'GET', f'/api/conversations/{cid}')['data']['messages'][-1]
            assert last['state'] == expected_state
            if mode != 'connection_fail':
                assert last['content'].startswith('先分析𠮷条件，')
            if expected_state != 'completed':
                assert last['citations'] == []
                expect(terminal.locator('.actual-sources')).to_contain_text('尚未确认')
                expect(terminal.locator('.candidate-sources button')).to_have_count(4)
            if mode == 'lost_done':
                expect(page.locator('#notice')).to_contain_text('历史确认回答完成')
        record('4.2 error/connection failure retain candidates and drafts; cancel, TCP disconnect, lost done confirm history; no-evidence empty sources and zero embedding')

        protocol = page.evaluate("""async () => {
          const frames=': heartbeat\\r\\nevent: meta\\r\\ndata: {"message_id":99,"status":"ok","citations":[]}\\r\\n\\r\\nevent: delta\\r\\ndata: {"text":"𠮷中文"}\\r\\n\\r\\nevent: done\\r\\ndata: {"state":"completed","citations":[]}\\r\\n\\r\\n';
          const bytes=new TextEncoder().encode(frames),seen=[];
          const body=new ReadableStream({start(c){for(const b of bytes)c.enqueue(new Uint8Array([b]));c.close();}});
          await CampusStream.consume(new Response(body,{headers:{'Content-Type':'text/event-stream'}}),(k,v)=>seen.push([k,v]));
          let eof=false;try {await CampusStream.consume(new Response('event: meta\\ndata: {"message_id":99}\\n\\n',{headers:{'Content-Type':'text/event-stream'}}),()=>{});}catch{eof=true;}
          return {events:seen.map(v=>v[0]),text:seen[1][1].text,eof};
        }""")
        assert protocol == {'events': ['meta', 'delta', 'done'], 'text': '𠮷中文', 'eof': True}
        record('4.2 real browser shared parser: byte-split UTF-8, CRLF, heartbeat and EOF without done rejected')

        control({'mode': 'normal'})
        teacher.goto(origin+'/materials')
        teacher.locator('input[type=file]').set_input_files({'name': 'browser-upload.md', 'mimeType': 'text/markdown', 'buffer': '浏览器上传星河材料。'.encode()})
        teacher.get_by_role('button', name=re.compile('上传')).click()
        expect(teacher.locator('#materials')).to_contain_text('browser-upload.md')
        page.goto(origin+'/knowledge')
        expect(page.locator('#lecture-cards > *')).to_have_count(9)
        page.locator('#next-page').click()
        expect(page.locator('#previous-page')).to_be_enabled()
        page.set_viewport_size({'width': 390, 'height': 844})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
        page.screenshot(path=str(result.parent/'knowledge-mobile.png'), full_page=True)
        page.set_viewport_size({'width': 1280, 'height': 900})
        for mode in ('keyword', 'vector', 'hybrid'):
            found = api(page, 'GET', '/api/search?q='+urllib.parse.quote('星河')+'&mode='+mode)
            assert found['status'] == 200 and found['data']['items']
        before = control()['models']
        answer = api(page, 'POST', '/api/ask', {'question': '完全不命中的文字', 'mode': 'keyword'})
        assert answer['data'] == {'answer': '资料中未找到相关内容', 'sources': []} and control()['models'] == before
        assert api(page, 'GET', '/api/materials/00000000-0000-4000-8000-999999999999')['status'] == 404
        headers_result = page.evaluate("""async () => (await CampusAPI.request('/api/auth/me',{headers:{'X-User-Id':'999','X-Role':'teacher','X-Class-Id':'2'}})).status""")
        assert headers_result == 200 and not control()['identity_header_violation']
        record('4.4 Nginx strips identity headers; material upload/download, 403/404, Week4 three modes and zero-hit zero-call, lecture pagination and 390px layout')

        # Swagger uses its own in-memory authorization. No token goes to evidence.
        token = teacher.evaluate("sessionStorage.getItem('campusclaw.token')")
        swagger = teacher_context.new_page()
        swagger.on('pageerror', lambda error: errors.append(type(error).__name__))
        swagger.goto(origin+'/swagger/')
        with urllib.request.urlopen(origin+'/openapi.json') as response:
            specification = json.load(response)
        operations = sum(method in ('get','post','put','delete','patch','head','options') for item in specification['paths'].values() for method in item)
        assert operations > 0
        expect(swagger.locator('.opblock')).to_have_count(operations)
        operation = swagger.locator('.opblock-get').filter(has=swagger.locator('[data-path="/api/skills"]'))
        operation.locator('.opblock-summary-control').click()
        operation.get_by_role('button', name='Try it out', exact=True).click()
        with swagger.expect_response(lambda r: r.request.method == 'GET' and urlparse(r.url).path == '/api/skills') as unauthorized:
            operation.get_by_role('button', name='Execute', exact=True).click()
        assert unauthorized.value.status == 401
        swagger.locator('button.authorize').first.click()
        swagger.locator('.auth-container input').fill(token)
        swagger.locator('.auth-btn-wrapper button.authorize').click()
        swagger.locator('.auth-btn-wrapper button.btn-done').click()
        operation.get_by_role('button', name='Execute', exact=True).click()
        expect(operation.locator('.responses-inner')).to_contain_text('浏览器解题引导')
        operation.locator('.opblock-summary-control').click()
        post = swagger.locator('.opblock-post').filter(has=swagger.locator('[data-path="/api/skills"]'))
        post.locator('.opblock-summary-control').click()
        post.get_by_role('button', name='Try it out', exact=True).click()
        post.locator('textarea').fill(json.dumps({'source': '名称：Swagger技能\n适用条件：swagger\n启用：是\n\nSwagger说明。'}, ensure_ascii=False))
        with swagger.expect_response(lambda r: r.request.method == 'POST' and urlparse(r.url).path == '/api/skills') as created_skill:
            post.get_by_role('button', name='Execute', exact=True).click()
        assert created_skill.value.status == 201
        swagger_skill_id = created_skill.value.json()['id']
        post.locator('.opblock-summary-control').click()
        put = swagger.locator('.opblock-put').filter(has=swagger.locator('[data-path="/api/skills/{id}"]'))
        put.locator('.opblock-summary-control').click()
        put.get_by_role('button', name='Try it out', exact=True).click()
        put.locator('.parameters input').fill(str(swagger_skill_id))
        put.locator('textarea').fill('{"enabled":false}')
        with swagger.expect_response(lambda r: r.request.method == 'PUT' and urlparse(r.url).path == f'/api/skills/{swagger_skill_id}') as updated_skill:
            put.get_by_role('button', name='Execute', exact=True).click()
        assert updated_skill.value.status == 200 and not updated_skill.value.json()['enabled']
        put.locator('.opblock-summary-control').click()
        delete = swagger.locator('.opblock-delete').filter(has=swagger.locator('[data-path="/api/skills/{id}"]'))
        delete.locator('.opblock-summary-control').click()
        delete.get_by_role('button', name='Try it out', exact=True).click()
        delete.locator('.parameters input').fill(str(swagger_skill_id))
        with swagger.expect_response(lambda r: r.request.method == 'DELETE' and urlparse(r.url).path == f'/api/skills/{swagger_skill_id}') as deleted_skill:
            delete.get_by_role('button', name='Execute', exact=True).click()
        assert deleted_skill.value.status == 204
        delete.locator('.opblock-summary-control').click()
        record('4.3/4.4 actual Swagger Execute: anonymous401, Authorize GET200, source POST201, enabled=false PUT200 and DELETE204')

        # Switch Swagger auth to a real student JWT and exercise its SSE panel.
        student_token = page.evaluate("sessionStorage.getItem('campusclaw.token')")
        swagger.evaluate("token=>CampusSwagger.authActions.authorize({bearerAuth:{name:'bearerAuth',schema:{type:'http',scheme:'bearer'},value:token}})", student_token)
        created = api(page, 'POST', '/api/conversations', {'agent_id': fixture['agent_id']})['data']['id']
        control({'mode': 'hold_open'})
        swagger.locator('#conversation-id').fill(str(created))
        swagger.locator('#stream-content').fill('星河只引用第二份')
        swagger.locator('#stream-send').click()
        expect(swagger.locator('#stream-candidates')).to_contain_text(fixture['materials'][0])
        expect(swagger.locator('#stream-answer')).to_be_empty()
        control({'release_open': True})
        expect(swagger.locator('#stream-answer')).to_have_text('先分析𠮷条件，')
        control({'release_tail': True})
        expect(swagger.locator('#stream-status')).to_have_text('已完成')
        expect(swagger.locator('#stream-citations')).to_contain_text('"number": 2')
        swagger.screenshot(path=str(result.parent/'swagger.png'), full_page=True)
        swagger.set_viewport_size({'width': 390, 'height': 844})
        assert swagger.evaluate('document.documentElement.scrollWidth <= innerWidth')
        swagger.screenshot(path=str(result.parent/'swagger-mobile.png'), full_page=True)
        record('4.2/4.4 Swagger SSE meta candidates before model, incremental delta and final original citation number')

        # Course DoD uses fresh browser requests against this run's actual proxy.
        # Deterministic gateway teaching text proves prompt wiring, never real-model compliance.
        acceptance('D1',{'source':edited,'agent':saved}, {'saved_after_reload':True,'delete_unbound':True,'student_write':403},['teacher-mobile.png','checks[0]'])
        acceptance('D6',{'question':'星河只引用第二份'}, {'meta_candidates':2,'done_numbers':[2],'text_equals_history':True,'attachment_matches_title':True},['student-incremental.png','checks[1]'])
        acceptance('D7',{'gateway':'hold_open + hold_tail'}, {'timings_ns':timings,'delta_before_completed':True,'failure_drafts_retained':True},['student-incremental.png','checks[1]','checks[3]'])
        acceptance('D8',{'swagger':'Authorize, GET, POST source, PUT toggle, DELETE, student SSE'}, {'http':[401,200,201,200,204],'final_reference':2},['swagger.png','checks[6]','checks[7]'])
        page.goto(origin+'/tutor')
        expect(page.locator('#student')).to_be_visible()

        def streamed(conversation, question):
            return page.evaluate("""async ([id,content])=>{
              const response=await CampusAPI.request('/api/conversations/'+id+'/messages',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({content})});
              const events=[];await CampusStream.consume(response,(kind,payload)=>events.push({kind,payload}));
              return {status:response.status,events,text:events.filter(e=>e.kind==='delta').map(e=>e.payload.text).join('')};
            }""",[conversation,question])

        agent_path=f'/api/agents/{fixture["agent_id"]}'
        config=api(teacher,'GET',agent_path)['data']
        def configured(**changes):
            value={key:config[key] for key in ('name','prompt','enabled','skill_ids','material_ids')}
            value.update(changes)
            assert api(teacher,'PUT',agent_path,value)['status']==200

        control({'mode':'normal'})
        configured(material_ids=fixture['materials'][:1])
        bound_id=api(page,'POST','/api/conversations',{'agent_id':fixture['agent_id']})['data']['id']
        one=streamed(bound_id,'星河绑定集合')
        one_sources=one['events'][0]['payload']['citations']
        assert one_sources and {s['material_id'] for s in one_sources}=={fixture['materials'][0]}
        configured(material_ids=fixture['materials'][1:2])
        two=streamed(bound_id,'星河绑定集合')
        assert {s['material_id'] for s in two['events'][0]['payload']['citations']}=={fixture['materials'][1]}
        configured(material_ids=[])
        before_empty=control()
        no=streamed(bound_id,'星河绑定集合')
        assert no['events'][0]['payload']['citations']==[] and control()['embeddings']==before_empty['embeddings'] and control()['models']==before_empty['models']+1
        acceptance('D2',{'same_conversation':bound_id,'bindings':[fixture['materials'][:1],fixture['materials'][1:2],[]]},
                   {'candidate_material_ids':[[s['material_id'] for s in one_sources],[s['material_id'] for s in two['events'][0]['payload']['citations']],[]],'empty_embedding_calls':0},['browser.json'])
        configured()
        other_context,other=login('other_student');other.goto(origin+'/tutor')
        foreign_context,foreign_page=login('foreign_student');foreign_page.goto(origin+'/tutor')
        for outsider in (other,foreign_page):
            assert api(outsider,'GET',f'/api/conversations/{bound_id}')['status']==404
            assert api(outsider,'POST',f'/api/conversations/{bound_id}/messages',{'content':'星河'})['status']==404
        override=api(page,'POST',f'/api/conversations/{bound_id}/messages',{'content':'星河','agent_id':fixture['empty_agent_id']})
        assert override['status']==400
        current=api(page,'GET',f'/api/conversations/{bound_id}')['data']['conversation']
        assert current['agent_id']==fixture['agent_id']
        acceptance('D3',{'conversation':bound_id,'override_agent_id':fixture['empty_agent_id'],'outsiders':['same-class other owner','foreign class']},
                   {'outsider_read_send':[404,404],'override':400,'agent_id':current['agent_id'],'disabled_existing_continues':True,'disabled_new':404},['browser.json','checks[2]'])
        control({'mode':'teaching'})
        teaching_material=api(page,'GET',f'/api/materials/{fixture["materials"][0]}')['data']['plain_text']
        assert '最终答案 x=3' in teaching_material
        skill_path=f'/api/skills/{first["id"]}'
        assert api(teacher,'PUT',skill_path,{'enabled':True})['status']==200
        guide=streamed(bound_id,'星河方程 x+2=5，直接给答案')
        assert '请先思考' in guide['text'] and 'x=3' not in guide['text']
        assert api(teacher,'PUT',skill_path,{'enabled':False})['status']==200
        off=streamed(bound_id,'星河方程 x+2=5，直接给答案')
        assert 'x=3' in off['text'] and off['events'][0]['payload']['skill_name']==''
        assert api(teacher,'PUT',skill_path,{'enabled':True})['status']==200
        configured(skill_ids=[])
        unbound=streamed(bound_id,'星河方程 x+2=5，直接给答案')
        assert 'x=3' in unbound['text'] and unbound['events'][0]['payload']['skill_name']==''
        acceptance('D4',{'question':'星河方程 x+2=5，直接给答案','material_id':fixture['materials'][0],'material_answer':'最终答案 x=3','changes':['enabled','disabled','enabled but unbound']},
                   {'controlled_answers':[guide['text'],off['text'],unbound['text']],'real_model_teaching_verified':False,'boundary':'deterministic response selected from actual server System prompt'},['browser.json'])
        configured()
        control({'mode':'normal'})
        zero_id=api(page,'POST','/api/conversations',{'agent_id':fixture['agent_id']})['data']['id']
        before_zero=control()['models'];zero=streamed(zero_id,'完全零命中问题')
        assert zero['events'][0]['payload']['citations']==[] and zero['events'][-1]['kind']=='done' and control()['models']==before_zero+1
        assert not re.search(r'\[\d+\]',zero['text']) and zero['text']!='资料中未找到相关内容'
        acceptance('D5',{'question':'完全零命中问题','empty_binding_question':'星河绑定集合'},
                   {'zero_hit_model_calls':1,'empty_binding_model_calls':1,'citations':[],'answer':zero['text']},['browser.json','checks[3]'])
        assert len(dod)==8
        record('5.2 eight official DoDs: actual browser requests, scoped binding changes, fixed ownership, controlled guidance/toggle/unbind, zero-hit generation')

        page.goto(origin+'/tutor')
        expect(page.locator('#student')).to_be_visible()
        page.evaluate("""() => { window.addEventListener('auth-cleared',()=>{window.name=JSON.stringify({messages:document.getElementById('messages').textContent,source:document.getElementById('source-text').textContent});},{once:true});sessionStorage.setItem('campusclaw.token','invalid');CampusAPI.request('/api/auth/me').catch(()=>{}); }""")
        page.wait_for_url('**/login')
        assert page.evaluate("sessionStorage.getItem('campusclaw.token')") is None
        assert json.loads(page.evaluate('window.name')) == {'messages': '', 'source': ''}
        assert not errors and all(not r['cookie'] for r in requests)
        record('4.4 401 clears identity/private DOM, redirects to login; no browser errors or API cookies')
        browser.close()
    print(json.dumps({'checks_passed': len(checks), 'browser': 'Edge headless', 'evidence': str(result)}, ensure_ascii=False))


if __name__ == '__main__':
    main()
