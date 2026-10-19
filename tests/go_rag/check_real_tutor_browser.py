"""Four real gateway dialogue calls, real embeddings/Qdrant, no mock model."""
import json,os,re,time
from pathlib import Path
from playwright.sync_api import sync_playwright,expect

def main():
    origin=os.environ['WEEK5_BROWSER_ORIGIN']
    ready=Path(os.environ['WEEK5_BROWSER_READY']);fixture=json.loads(ready.read_text(encoding='utf-8-sig'))
    out=Path(os.environ['WEEK5_BROWSER_RESULT']);root=out.parent
    evidence={'passed':False,'real_model_verified':False,'answers':[],'requests_budget':{'dialogue':4,'embedding':8}}
    try:
        with sync_playwright() as p:
            browser=p.chromium.launch(channel='msedge',headless=True)
            def login(user):
                context=browser.new_context();page=context.new_page()
                page.goto(origin+'/login');page.locator('[name=username]').fill(user);page.locator('[name=password]').fill(os.environ['WEEK5_ACCOUNT_PASSWORD']);page.get_by_role('button',name='登录',exact=True).click();page.wait_for_url('**/materials');return context,page
            tc,teacher=login('teacher_a')
            teacher.goto(origin+'/tutor');teacher.locator('#edit-agent').select_option(str(fixture['agent_id']))
            teacher.locator('#prompt').fill('你是耐心的数学学习助手。每次回答以“学习提示：”开头，尽量不超过120字。依据当前讲义回答并标注来源。')
            teacher.get_by_role('button',name='保存助手',exact=True).click();expect(teacher.locator('#notice')).to_contain_text('助手已保存')
            teacher.locator('#skills input').check();expect(teacher.locator('#skills input')).to_be_enabled()
            sc,page=login('student_a1');page.goto(origin+'/tutor');page.locator('#agent').select_option(str(fixture['agent_id']));page.locator('#create').click();expect(page.locator('#notice')).to_have_text('会话已创建。')
            questions=['解方程2x+3=11，请直接给出最终答案。','同一题2x+3=11，请给解题提示，我应先做什么？','我已得到2x=8，接下来怎样思考？请给提示。']
            for index,question in enumerate(questions):
                start=time.monotonic();page.locator('#question').fill(question);page.locator('#send-button').click()
                draft=page.locator('.message[data-state=generating] p')
                expect(draft).not_to_have_text('',timeout=120000)
                first=time.monotonic()-start
                expect(page.locator('#send-button')).to_be_enabled(timeout=120000)
                latest=page.locator('#messages .message').last
                state=latest.get_attribute('data-state');answer=latest.locator('p').inner_text()
                evidence['answers'].append({'question':question,'skill':'enabled','state':state,'answer':answer,'first_visible_seconds':first,'completed_seconds':time.monotonic()-start})
                assert state=='completed','real model failed generation/citation validation'
                assert answer.startswith('学习提示'),'teacher prompt prefix not observed'
                assert not re.search(r'(?:x\s*[=＝]\s*4|(?:答案|结果|数值|未知数)[是为：: ]*(?:4|四))',answer),'enabled guidance leaked final answer'
                assert latest.locator('.sources button').count()>0,'missing real citation display'
            teacher.locator('#skills input').uncheck();expect(teacher.locator('#skills input')).to_be_enabled()
            start=time.monotonic();page.locator('#question').fill('现在请完整解出2x+3=11，给出x的最终数值。');page.locator('#send-button').click();expect(page.locator('#send-button')).to_be_enabled(timeout=120000)
            last=page.locator('#messages .message').last;answer=last.locator('p').inner_text()
            evidence['answers'].append({'question':'关闭技能后同题直接求解','skill':'disabled','state':last.get_attribute('data-state'),'answer':answer,'completed_seconds':time.monotonic()-start})
            assert last.get_attribute('data-state')=='completed'
            assert re.search(r'x\s*[=＝]\s*4|(?:答案|结果|数值)[是为：: ]*4',answer),'disabled same-problem conclusion not observed'
            assert '解题引导' not in last.locator('.state').inner_text()
            last.locator('.sources button').first.click();expect(page.locator('#source-text')).to_contain_text('2x+3=11')
            page.screenshot(path=str(root/'real-teaching.png'),full_page=True)
            own=page.evaluate("async()=> (await (await CampusAPI.request('/api/conversations')).json()).conversations[0].id")
            history=page.evaluate("async id=>(await (await CampusAPI.request('/api/conversations/'+id)).json())",own)
            assistants=[m for m in history['messages'] if m['role']=='assistant']
            assert len(assistants)==4 and all(m['skill_name']=='解题引导' for m in assistants[:3]) and assistants[-1]['skill_name']==''
            evidence['history_audit']=[{'id':m['id'],'state':m['state'],'prompt_revision':m['prompt_revision'],'skill_name':m['skill_name'],'citations':m['citations']} for m in assistants]
            bc,b=login('student_b1');token=b.evaluate("sessionStorage.getItem('campusclaw.token')")
            headers={'Authorization':'Bearer '+token}
            assert bc.request.get(origin+'/api/conversations/'+str(own),headers=headers).status==404
            assert bc.request.get(origin+'/api/materials/'+fixture['material_id'],headers=headers).status==404
            result=bc.request.get(origin+'/api/search?q=2x%2B3%3D11&mode=vector',headers=headers)
            assert result.status==200 and result.json()['items']==[]
            before=json.loads((root/'real-stats.json').read_text())['dialogue_requests']
            page.locator('#agent').select_option(str(fixture['no_evidence_agent_id']));page.locator('#create').click();expect(page.locator('#messages .message')).to_have_count(0)
            page.locator('#question').fill('请解2x+3=11');page.locator('#send-button').click();expect(page.locator('#send-button')).to_be_enabled(timeout=30000)
            expect(page.locator('#messages .message').last.locator('p')).to_have_text('资料中未找到相关内容')
            assert json.loads((root/'real-stats.json').read_text())['dialogue_requests']==before
            token=page.evaluate("sessionStorage.getItem('campusclaw.token')")
            assert sc.request.post(origin+'/api/conversations/'+str(own)+'/messages',headers={'Authorization':'Bearer '+token},data={'content':' '}).status==400
            evidence.update({'passed':True,'real_model_verified':True,'class_isolation':True,'no_evidence_zero_dialogue':True,'invalid_request400':True})
            browser.close()
    finally:
        out.write_text(json.dumps(evidence,ensure_ascii=False,indent=2),encoding='utf-8')
    print('PASS real gateway teaching:3 enabled questions +1 disabled answer, browser/citations/isolation/no-evidence')

if __name__=='__main__':main()
