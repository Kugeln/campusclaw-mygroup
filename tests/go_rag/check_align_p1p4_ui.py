"""Supplement P1-P4 with production tutor UI rendering, controlled gateway only."""
import json
import os
import subprocess
import uuid
from datetime import datetime, timezone
from pathlib import Path

from playwright.sync_api import sync_playwright, expect


def main():
    root = Path(os.environ['ALIGN_BROWSER_READY']).parent
    fixture = json.loads((root / 'ready.json').read_text(encoding='utf-8-sig'))
    verification = json.loads((root / 'browser.json').read_text(encoding='utf-8'))
    assert verification['status'] == 'passed'
    origin = os.environ['ALIGN_BROWSER_ORIGIN']
    assert origin.startswith('http://127.0.0.1:')
    result = {'operator': 'Codex automated simulation, not genuine peer', 'utc': datetime.now(timezone.utc).isoformat()}
    evidence = root / ('ui-' + uuid.uuid4().hex[:8])
    evidence.mkdir()
    with sync_playwright() as pw:
        browser = pw.chromium.launch(channel='msedge', headless=True)
        page = browser.new_page(viewport={'width': 1280, 'height': 900})
        page.goto(origin + '/login')
        page.locator('[name=username]').fill('sa')
        page.locator('[name=password]').fill(os.environ['ALIGN_BROWSER_PASSWORD'])
        page.get_by_role('button', name='登录', exact=True).click()
        page.wait_for_url('**/materials')
        page.goto(origin + '/tutor')
        expect(page.locator('#student')).to_be_visible()
        # Observe the existing parser without changing its parsing or callbacks.
        # It cancels its reader after done, so CDP response.body is unreliable.
        page.evaluate('''()=>{const consume=CampusStream.consume;window.__p1p4Events=[];
          window.CampusStream=Object.freeze({consume:(response,callback,signal)=>consume(response,(kind,payload)=>{
            window.__p1p4Events.push({kind,payload});callback(kind,payload)},signal)});}''')

        def control(body=None):
            return page.evaluate('''async body=>{const r=await fetch('/__phase4/control',body===null?{}:{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});if(!r.ok)throw new Error('control unavailable');return r.json()}''', body)

        page.locator('#agent').select_option(str(verification['resources']['agent_b']))
        with page.expect_response(lambda r: r.request.method == 'POST' and r.url.endswith('/api/conversations')) as created:
            page.locator('#create').click()
        cid = created.value.json()['id']
        initial = control({'mode': 'hold_open'})
        question = '星河材料绑定，请依据本轮资料回答。'
        page.locator('#question').fill(question)
        with page.expect_response(lambda r: r.request.method == 'POST' and r.url.endswith(f'/api/conversations/{cid}/messages')) as streamed:
            page.locator('#send-button').click()
        draft = page.locator('.message[data-state=generating]').last
        expect(draft.locator('.candidate-sources button')).not_to_have_count(0)
        expect(draft.locator('p')).to_be_empty()
        meta = control()
        assert meta['models'] == initial['models']
        page.screenshot(path=str(evidence / 'UI-meta-before-model.png'), full_page=True)
        control({'release_open': True})
        expect(draft.locator('p')).to_have_text('先分析𠮷条件，')
        assert control()['completed'] == initial['completed']
        container = os.environ['ALIGN_STORAGE_CONTAINER']

        def stored_text(query):
            stored = subprocess.check_output(['docker', 'exec', container, 'sh', '-c', 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot --default-character-set=utf8mb4 --batch --skip-column-names "$1" -e "$2"', '_', fixture['database'], query]).decode().strip()
            return bytes.fromhex(stored).decode()

        prefix = stored_text(f'SELECT HEX(content) FROM conversation_messages WHERE conversation_id={int(cid)} AND role="assistant" ORDER BY id DESC LIMIT 1')
        assert prefix == '先分析𠮷条件，'
        result['visible_prefix_already_persisted'] = prefix
        page.screenshot(path=str(evidence / 'UI-incremental-before-done.png'), full_page=True)
        control({'release_tail': True})
        expect(page.locator('.message[data-state=completed]').last.locator('p')).to_contain_text('[1]')
        events = page.evaluate('window.__p1p4Events')
        text = ''.join('event: '+e['kind']+'\ndata: '+json.dumps(e['payload'],ensure_ascii=False)+'\n\n' for e in events)
        (evidence / 'UI-production.sse').write_text(text, encoding='utf-8')
        history = page.evaluate('''async id=>{const r=await CampusAPI.request('/api/conversations/'+id);return r.json()}''', cid)
        final = history['messages'][-1]
        answer = ''.join(e['payload']['text'] for e in events if e['kind'] == 'delta')
        assert events[0]['kind'] == 'meta' and events[-1]['kind'] == 'done'
        assert final['state'] == 'completed' and final['content'] == answer
        assert {c['material_id'] for c in final['citations']} == {verification['resources']['material_b']}
        query = f'SELECT HEX(content) FROM conversation_messages WHERE id={int(final["id"])}'
        assert stored_text(query) == answer
        page.reload()
        expect(page.locator('#conversations button')).not_to_have_count(0)
        page.locator('#conversations button').first.click()
        expect(page.locator('.message[data-state=completed]').last.locator('p')).to_have_text(answer)
        page.screenshot(path=str(evidence / 'UI-completed-history.png'), full_page=True)
        result.update(status='passed', browser=browser.version, conversation_id=cid, history=history, events=events,
                      meta_before_model=True, incremental_before_completion=True, delta_equals_database=True,
                      reload_history_matches=True, controlled_gateway=control())
        result['evidence_directory'] = str(evidence)
        (evidence / 'result.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
        (root / 'ui-production.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
        browser.close()


if __name__ == '__main__':
    main()
