"""Real same-origin Swagger authorization, Try it out and incremental SSE."""
import json
import os
from pathlib import Path
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright,expect

def main():
    origin=os.environ['WEEK5_BROWSER_ORIGIN']
    assert urlparse(origin).hostname=='127.0.0.1'
    fixture=json.loads(Path(os.environ['WEEK5_BROWSER_READY']).read_text(encoding='utf-8-sig'))
    password=os.environ['WEEK5_ACCOUNT_PASSWORD']
    with sync_playwright() as p:
        browser=p.chromium.launch(channel='msedge',headless=True)
        context=browser.new_context()
        page=context.new_page();errors=[];api_requests=[]
        page.on('pageerror',lambda e:errors.append(type(e).__name__))
        page.on('request',lambda r:api_requests.append(r.url) if '/api/' in r.url else None)
        page.goto(origin+'/swagger/')
        expect(page.locator('.swagger-ui .info .title')).to_contain_text('CampusClaw Go API')
        assert context.request.get(origin+'/openapi.json').json()['openapi']=='3.0.3'
        page.locator('#operations-认证-get_api_auth_me').locator('.opblock-summary').click()
        op=page.locator('#operations-认证-get_api_auth_me')
        op.get_by_role('button',name='Try it out',exact=True).click()
        op.get_by_role('button',name='Execute',exact=True).click()
        expect(op.locator('.live-responses-table')).to_contain_text('401')
        result=context.request.post(origin+'/api/auth/login',data={'username':'student_a1','password':password})
        assert result.status==200
        token=result.json()['access_token']
        page.locator('.scheme-container').get_by_role('button',name='Authorize').click()
        dialog=page.locator('.dialog-ux')
        dialog.locator('input').fill(token)
        dialog.locator('.auth-btn-wrapper button.authorize').click()
        dialog.locator('.auth-btn-wrapper button.btn-done').click()
        op.get_by_role('button',name='Execute',exact=True).click()
        expect(op.locator('.live-responses-table')).to_contain_text('200')
        expect(op.locator('.live-responses-table')).to_contain_text('student')
        # Teacher config endpoints are also usable through the real UI engine.
        teacher=context.request.post(origin+'/api/auth/login',data={'username':'teacher_a','password':password}).json()['access_token']
        page.evaluate('(token)=>CampusSwagger.preauthorizeApiKey("bearerAuth",token)',teacher)
        page.locator('#operations-教师配置-get_api_agents .opblock-summary').click()
        agents=page.locator('#operations-教师配置-get_api_agents')
        agents.get_by_role('button',name='Try it out',exact=True).click()
        agents.get_by_role('button',name='Execute',exact=True).click()
        expect(agents.locator('.live-responses-table')).to_contain_text('200')
        page.evaluate('(token)=>CampusSwagger.preauthorizeApiKey("bearerAuth",token)',token)
        page.locator('#operations-教师配置-post_api_agents .opblock-summary').click()
        forbidden=page.locator('#operations-教师配置-post_api_agents')
        forbidden.get_by_role('button',name='Try it out',exact=True).click()
        forbidden.get_by_role('button',name='Execute',exact=True).click()
        expect(forbidden.locator('.live-responses-table')).to_contain_text('403')
        page.locator('#operations-教师配置-get_api_agents_id .opblock-summary').click()
        missing=page.locator('#operations-教师配置-get_api_agents_id')
        missing.get_by_role('button',name='Try it out',exact=True).click()
        missing.locator('.parameters input').fill('999999999')
        missing.get_by_role('button',name='Execute',exact=True).click()
        expect(missing.locator('.live-responses-table')).to_contain_text('404')
        headers={'Authorization':'Bearer '+token}
        c=context.request.post(origin+'/api/conversations',headers=headers,data={'agent_id':fixture['agent_id']})
        assert c.status==201
        page.locator('#conversation-id').fill(str(c.json()['id']))
        page.locator('#stream-content').fill('星河如何解题')
        page.locator('#stream-send').click()
        expect(page.locator('#stream-events')).to_contain_text('delta:',timeout=2000)
        assert 'done:' not in page.locator('#stream-events').inner_text()
        expect(page.locator('#stream-status')).to_have_text('已完成',timeout=8000)
        events=page.locator('#stream-events').inner_text()
        assert events.index('meta:')<events.index('delta:')<events.index('done:') and events.count('delta:')==3
        assert 'citations' in events
        page.locator('#stream-content').fill('星河失败')
        page.locator('#stream-send').click()
        expect(page.locator('#stream-status')).to_have_text('回答失败',timeout=8000)
        assert 'error:' in page.locator('#stream-events').inner_text() and 'done:' not in page.locator('#stream-events').inner_text()
        assert not errors
        assert all('token' not in urlparse(url).query.lower() for url in api_requests)
        assert page.evaluate('sessionStorage.length+localStorage.length')==0
        browser.close()
    Path(os.environ['WEEK5_BROWSER_RESULT']).write_text(json.dumps({'passed':True,'swagger_authorize':True,'try_it_out_401_200':True,'try_it_out_403_404':True,'teacher_agents':True,'real_incremental_sse':True,'error_no_done':True,'token_not_persisted':True,'real_model_verified':False},indent=2),encoding='utf-8')
    print('PASS Swagger browser Authorize, Try it out, real delta and error')

if __name__=='__main__':main()
