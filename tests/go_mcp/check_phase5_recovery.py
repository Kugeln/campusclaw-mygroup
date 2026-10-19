"""Supplemental browser checks for tool display after transport recovery."""
import json
import os
from pathlib import Path
import time
import traceback
import urllib.request
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright, expect


def main():
    result=Path(os.environ['W6_BROWSER_RESULT']); root=result.parent
    ready=json.loads((root/'ready.json').read_text(encoding='utf-8'))
    origin=os.environ['W6_BROWSER_ORIGIN']; password=os.environ['W6_BROWSER_PASSWORD']; token=os.environ['MCP_AUTH_TOKEN']
    assert urlparse(origin).hostname=='127.0.0.1' and urlparse(origin).port not in (80,443,8080,8081)
    report={'passed':False,'checks':[],'real_paid_model_calls':0,'database':ready['database']}
    def save(): result.write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
    def record(name): report['checks'].append({'name':name,'status':'passed'}); save(); print('PASS '+name,flush=True)
    def control(mode=None):
        request=urllib.request.Request(origin+'/__phase5MCP/control',data=None if mode is None else json.dumps({'mode':mode}).encode(),headers={'Content-Type':'application/json'})
        with urllib.request.urlopen(request,timeout=5) as response:return json.load(response)
    try:
      with sync_playwright() as pw:
        browser=pw.chromium.launch(channel='msedge',headless=True)
        def login(who):
            page=browser.new_page(viewport={'width':1440,'height':1000});page.goto(origin+'/login')
            page.locator('[name=username]').fill(ready['users'][who]);page.locator('[name=password]').fill(password)
            page.get_by_role('button',name='登录',exact=True).click();page.wait_for_url('**/materials');page.goto(origin+'/tutor')
            expect(page.locator('#'+('teacher' if who=='teacher' else 'student'))).to_be_visible()
            return page
        def api(page,method,path,data=None):
            return page.evaluate('''async([method,path,data])=>{const response=await CampusAPI.request(path,{method,...(data===null?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(data)})});if(!response.ok)throw new Error('fixture setup HTTP '+response.status);return response.json();}''',[method,path,data])
        teacher=login('teacher')
        server=api(teacher,'POST','/api/mcp/servers',{'name':'recovery-demo','base_url':'http://mcp-demo:8090','auth_token':token})
        api(teacher,'POST',f'/api/mcp/servers/{server["id"]}/connect')
        skill=api(teacher,'POST','/api/skills',{'source':'名称：恢复测试\n适用条件：星河,方程\n工具：formula_hint@recovery-demo\n启用：是\n\n只提示下一步。'})
        cfg=api(teacher,'GET',f'/api/agents/{ready["agent_id"]}')
        cfg={key:cfg[key] for key in ('name','prompt','enabled','material_ids')};cfg['skill_ids']=[skill['id']]
        api(teacher,'PUT',f'/api/agents/{ready["agent_id"]}',cfg)
        student=login('student');student.locator('#agent').select_option(str(ready['agent_id']));student.locator('#create').click();expect(student.locator('#send-button')).to_be_enabled()
        for mode in ('lost_done','cancel','disconnect'):
            before=control(mode)['tool_runs']
            student.locator('#question').fill('星河方程恢复检查 '+mode);student.locator('#send-button').click()
            if mode=='cancel':
                expect(student.locator('.message').last.locator('.tool-call')).to_have_count(1)
                expect(student.locator('.message').last.locator(':scope > p')).to_contain_text('先分析')
                student.locator('#cancel').click()
            if mode=='lost_done':expect(student.locator('#notice')).to_have_text('已从历史确认回答完成。',timeout=15000)
            expect(student.locator('#send-button')).to_be_enabled(timeout=15000)
            expect(student.locator('.message').last).to_have_attribute('data-state','completed' if mode=='lost_done' else 'interrupted',timeout=15000)
            expect(student.locator('.message').last.locator('.tool-call')).to_have_count(1)
            expect(student.locator('.message').last.locator('.tool-call')).to_have_attribute('data-status','ok')
            assert control()['tool_runs']==before+1,'transport recovery replayed tools'
            student.reload();student.locator('#conversations button').first.click()
            expect(student.locator('.message').last.locator('.tool-call')).to_have_count(1)
            assert control()['tool_runs']==before+1,'history reload replayed tools'
            record(mode+': terminal/prefix and tool snapshot restored; no retry/replay')
        student.screenshot(path=str(root/'recovered-tool-history.png'),full_page=True)
        stats=control();assert stats['cancelled']>=1 and not stats['identity_header_violation']
        report['tool_runs']=stats['tool_runs'];report['cancelled_models']=stats['cancelled'];report['passed']=True;save();browser.close()
    except Exception:
        detail=traceback.format_exc().replace(password,'<redacted>').replace(token,'<redacted>')
        report['checks'].append({'name':'recovery acceptance aborted','status':'failed','detail':detail});save();raise RuntimeError(detail) from None


if __name__=='__main__':main()
