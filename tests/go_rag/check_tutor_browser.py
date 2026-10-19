"""Phase 4: real browser -> isolated Nginx -> Go -> MySQL/Qdrant."""
import json
import os
from pathlib import Path
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright, expect


def main():
    origin = os.environ['WEEK5_BROWSER_ORIGIN']
    parsed = urlparse(origin)
    assert parsed.hostname == '127.0.0.1' and parsed.port not in (80, 443, 8080)
    fixture = json.loads(Path(os.environ['WEEK5_BROWSER_READY']).read_text(encoding='utf-8-sig'))
    password = os.environ['WEEK5_ACCOUNT_PASSWORD']
    checks, requests, errors = [], [], []
    evidence_dir = Path(os.environ['WEEK5_BROWSER_RESULT']).parent
    with sync_playwright() as p:
        browser = p.chromium.launch(channel='msedge', headless=True)
        def context_for(user):
            context = browser.new_context(accept_downloads=True)
            page = context.new_page()
            page.on('pageerror', lambda error: errors.append(type(error).__name__))
            def record(request):
                if '/api/' in request.url:
                    headers = request.all_headers()
                    requests.append({'path':urlparse(request.url).path,'query':urlparse(request.url).query,'bearer':headers.get('authorization','').startswith('Bearer '),'cookie':'cookie' in headers})
            page.on('request',record)
            page.goto(origin+'/login')
            page.locator('[name=username]').fill(user)
            page.locator('[name=password]').fill(password)
            page.get_by_role('button',name='登录',exact=True).click()
            page.wait_for_url('**/materials')
            return context,page
        teacher_context, teacher = context_for('teacher_a')
        teacher.goto(origin+'/tutor')
        expect(teacher.locator('#teacher')).to_be_visible()
        expect(teacher.locator('#student')).to_be_hidden()
        teacher.locator('#edit-agent').select_option(str(fixture['agent_id']))
        teacher.locator('#prompt').fill('浏览器保存的教学原则：一步一步分析。')
        teacher.get_by_role('button',name='保存助手',exact=True).click()
        expect(teacher.locator('#notice')).to_contain_text('助手已保存')
        teacher.reload()
        teacher.locator('#edit-agent').select_option(str(fixture['agent_id']))
        expect(teacher.locator('#prompt')).to_have_value('浏览器保存的教学原则：一步一步分析。')
        toggle = teacher.locator('#skills input')
        toggle.check()
        expect(toggle).to_be_checked()
        toggle.uncheck()
        expect(teacher.locator('#notice')).to_contain_text('技能状态已保存')
        toggle.check()
        expect(teacher.locator('#notice')).to_contain_text('技能状态已保存')
        checks.append('teacher prompt persistence and skill toggles')
        teacher.screenshot(path=str(evidence_dir/'teacher.png'),full_page=True)
        student_context, page = context_for('student_a1')
        page.goto(origin+'/tutor')
        expect(page.locator('#teacher')).to_be_hidden()
        expect(page.locator('#student')).to_be_visible()
        if os.environ.get('WEEK5_BROWSER_PREVIOUS_READY'):
            previous=json.loads(Path(os.environ['WEEK5_BROWSER_PREVIOUS_READY']).read_text(encoding='utf-8-sig'))
            rows=page.evaluate("async () => (await (await CampusAPI.request('/api/conversations')).json()).conversations")
            older=next(c for c in rows if c['agent_id']==previous['agent_id'] and c['title'].startswith('星河'))
            history=page.evaluate("async id => (await (await CampusAPI.request('/api/conversations/'+id)).json())",older['id'])
            assert any(m['state']=='completed' and m['citations'] for m in history['messages'] if m['role']=='assistant')
            page.locator('#conversations button').nth(next(i for i,c in enumerate(rows) if c['id']==older['id'])).click()
            expect(page.locator('.message')).to_have_count(len(history['messages']))
            checks.append('new Go service instance reads previous instance durable history and citations')
        page.locator('#agent').select_option(str(fixture['agent_id']))
        page.locator('#create').click()
        expect(page.locator('#notice')).to_have_text('会话已创建。')
        page.locator('#question').fill('星河如何解题')
        page.locator('#send-button').click()
        expect(page.locator('.message[data-state=generating] p')).to_have_text('先分析条件，',timeout=2000)
        expect(page.locator('#send-button')).to_be_disabled()
        # A second submit while streaming is ignored by the UI.
        page.evaluate("document.getElementById('send').requestSubmit()")
        expect(page.locator('.message[data-state=completed] .state').last).to_contain_text('已完成',timeout=8000)
        expect(page.locator('.message')).to_have_count(2)
        expect(page.locator('.message .sources').get_by_role('button',name='下载材料')).to_be_visible()
        page.locator('.sources button').first.click()
        expect(page.locator('#source-text')).to_contain_text('星河解题先分析')
        with page.expect_download() as download:
            page.get_by_role('button',name='下载材料',exact=True).click()
        assert download.value.suggested_filename == 'browser-source.md'
        page.screenshot(path=str(evidence_dir/'student.png'),full_page=True)
        page.set_viewport_size({'width':390,'height':844})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
        page.screenshot(path=str(evidence_dir/'student-mobile.png'),full_page=True)
        page.set_viewport_size({'width':1280,'height':720})
        page.reload()
        # New conversation is first in the persisted list.
        page.locator('#conversations button').first.click()
        expect(page.locator('.message')).to_have_count(2)
        checks.append('proxy first delta before completion, duplicate click suppressed, citations and reload')
        page.locator('#question').fill('星河失败')
        page.locator('#send-button').click()
        expect(page.locator('.message[data-state=failed]')).to_have_count(1,timeout=8000)
        page.locator('#question').fill('星河继续解题')
        page.locator('#send-button').click()
        expect(page.locator('.message[data-state=generating]')).to_have_count(1,timeout=2000)
        page.locator('#cancel').click()
        expect(page.locator('.message[data-state=interrupted]')).to_have_count(1,timeout=5000)
        checks.append('model failure and browser abort preserve honest states')
        # Deliberately remove the transport done after the real server committed.
        # This fault path is separate from the real incremental timing test above.
        def lose_done(route):
            result = route.fetch(timeout=15000)
            body = result.body().decode('utf-8')
            assert 'event: done' in body
            route.fulfill(status=result.status,headers=result.headers,body=body.split('event: done')[0])
        page.route('**/api/conversations/*/messages',lose_done)
        page.locator('#question').fill('星河完成事件丢失')
        page.locator('#send-button').click()
        expect(page.locator('#notice')).to_have_text('已从历史确认回答完成。',timeout=10000)
        expect(page.locator('.message').last).to_have_attribute('data-state','completed')
        page.unroute('**/api/conversations/*/messages',lose_done)
        checks.append('lost done confirms committed completion from real history')
        owned = page.evaluate("async () => (await (await CampusAPI.request('/api/conversations')).json()).conversations[0].id")
        b_context,b_page = context_for('student_b1')
        b_page.goto(origin+'/tutor')
        expect(b_page.locator('#student')).to_be_visible()
        expect(b_page.locator('#agent option[value="'+str(fixture['agent_id'])+'"]')).to_have_count(0)
        b_token = b_page.evaluate("sessionStorage.getItem('campusclaw.token')")
        assert b_context.request.get(origin+'/api/conversations/'+str(owned),headers={'Authorization':'Bearer '+b_token}).status==404
        assert b_context.request.get(origin+'/api/materials/'+fixture['material_id']+'/file',headers={'Authorization':'Bearer '+b_token}).status==404
        checks.append('B class cannot select A assistant or read A conversation/material')
        # No evidence must use a fresh conversation, excluding old query context.
        page.locator('#create').click()
        expect(page.locator('#messages .message')).to_have_count(0)
        page.locator('#question').fill('完全不存在的材料')
        page.locator('#send-button').click()
        expect(page.locator('.message').last.locator('p')).to_have_text('资料中未找到相关内容')
        expect(page.locator('.message').last.locator('.sources button')).to_have_count(0)
        checks.append('no evidence fixed text and no invented sources')
        token=page.evaluate("sessionStorage.getItem('campusclaw.token')")
        headers={'Authorization':'Bearer '+token}
        assert student_context.request.put(origin+'/api/agents/'+str(fixture['agent_id']),headers=headers,data={}).status==403
        assert student_context.request.put(origin+'/api/skills/1',headers=headers,data={'enabled':True}).status==403
        # Preserve single-shot entry with actual old request contract.
        page.goto(origin+'/knowledge')
        page.locator('#question').fill('星河')
        page.get_by_role('button',name='检索',exact=True).click()
        expect(page.locator('#sources')).to_contain_text('切片')
        page.get_by_role('button',name='单次问答',exact=True).click()
        expect(page.locator('#answer')).to_contain_text('单次回答')
        page.goto(origin+'/materials')
        expect(page.locator('#materials')).to_contain_text('browser-source.md')
        checks.append('student mutation rejected and old materials/search/ask pages usable')
        # The proxy retains the existing 20MiB application boundary.
        teacher_token=teacher.evaluate("sessionStorage.getItem('campusclaw.token')")
        result=teacher_context.request.post(origin+'/api/materials',headers={'Authorization':'Bearer '+teacher_token},multipart={'file':{'name':'boundary20.txt','mimeType':'text/plain','buffer':b'x'*(20*1024*1024)}},timeout=60000)
        assert result.status==201
        result=teacher_context.request.post(origin+'/api/materials',headers={'Authorization':'Bearer '+teacher_token},multipart={'file':{'name':'over20.txt','mimeType':'text/plain','buffer':b'x'*(20*1024*1024+1)}},timeout=60000)
        assert result.status==413
        checks.append('Nginx ordinary upload exact 20MiB and oversize413')
        # Stream parser fault checks run in Node; browser authentication clearing is real.
        page.goto(origin+'/tutor')
        page.locator('#agent').select_option(str(fixture['agent_id']))
        page.locator('#create').click()
        expect(page.locator('#notice')).to_have_text('会话已创建。')
        page.evaluate("sessionStorage.setItem('campusclaw.token','invalid')")
        page.locator('#question').fill('星河问题')
        page.locator('#send-button').click()
        page.wait_for_url('**/login')
        assert page.evaluate("sessionStorage.getItem('campusclaw.token')") is None
        assert not errors
        assert all('token' not in r['query'].lower() and not r['cookie'] for r in requests)
        assert all(r['bearer'] for r in requests if r['path']!='/api/auth/login')
        checks.append('Bearer headers, no token URL/cookies,401 clears identity')
        browser.close()
    output={'passed':True,'checks':checks,'real_model_verified':False,'real_mysql_qdrant':True,'via_nginx':True}
    Path(os.environ['WEEK5_BROWSER_RESULT']).write_text(json.dumps(output,ensure_ascii=False,indent=2),encoding='utf-8')
    print('PASS browser:',len(checks),'acceptance groups')

if __name__=='__main__':
    main()
