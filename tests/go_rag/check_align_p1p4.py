"""Codex-only P1-P4 simulation. No genuine peer or real model evidence."""
import hashlib
import json
import os
import re
import subprocess
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse

from playwright.sync_api import sync_playwright, expect


def main():
    origin = os.environ['ALIGN_BROWSER_ORIGIN']
    fixture = json.loads(Path(os.environ['ALIGN_BROWSER_READY']).read_text(encoding='utf-8-sig'))
    result = Path(os.environ['ALIGN_BROWSER_RESULT'])
    root = result.parent
    password = os.environ['ALIGN_BROWSER_PASSWORD']
    database = fixture['database']
    container = os.environ['ALIGN_STORAGE_CONTAINER']
    assert urlparse(origin).hostname == '127.0.0.1'
    assert re.fullmatch(r'align_w5_[a-f0-9]{16}', database)
    assert re.fullmatch(r'campusclaw-align-storage-[a-f0-9]{8}', container)
    assert not any(k in os.environ for k in ('DIALOGUE_API_KEY', 'EMBEDDING_API_KEY'))
    budget = Path('runtime/align-real-26bcce67/budget.json')
    budget_hash = hashlib.sha256(budget.read_bytes()).hexdigest()
    ledger = json.loads(budget.read_text(encoding='utf-8'))
    assert ledger['blocked'] and (ledger['dialogues'], ledger['embeddings']) == (11, 8)
    state = {'operator': 'Codex automated simulation', 'genuine_peers': [],
             'peer_confirmation': False, 'real_models_called': False,
             'started_utc': datetime.now(timezone.utc).isoformat(), 'timezone': 'Asia/Shanghai (UTC+08:00)',
             'environment': {'origin': origin, **fixture, 'model': 'local controlled HTTP',
                             'head': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
                             'worktree': subprocess.check_output(['git', 'status', '--short'], text=True)},
             'steps': [], 'requests': [], 'streams': [], 'snapshots': {}}
    planned = ['P1.own_create_send', 'P1.refresh_read_continue', 'P1.other_list', 'P1.other_read', 'P1.other_send',
               'P2.A_B_bindings', 'P2.override_agent', 'P2.disabled_new', 'P2.disabled_continue_current',
               'P2.no_enabled_400', 'P2.history_preserved',
               'P3.foreign_read', 'P3.foreign_send', 'P3.foreign_agent', 'P3.foreign_skill',
               'P3.foreign_material', 'P3.foreign_download', 'P3.foreign_reference', 'P3.foreign_create',
               'P3.class_override', 'P3.user_override', 'P3.material_binding_atomic', 'P3.skill_binding_atomic',
               'P3.no_side_effects', 'P4.enabled', 'P4.disabled', 'P4.unbound', 'P4.history_preserved']
    state['steps'] = [{'id': x, 'status': 'not_executed'} for x in planned]
    active = None

    def save():
        state['updated_utc'] = datetime.now(timezone.utc).isoformat()
        result.write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding='utf-8')

    def start(key):
        nonlocal active
        active = next(x for x in state['steps'] if x['id'] == key)
        active['started_utc'] = datetime.now(timezone.utc).isoformat()
        save()

    def passed(**details):
        active.update(status='passed', details=details)
        save()

    def sql(query):
        assert query.startswith('SELECT ') and ';' not in query
        args = ['docker', 'exec', container, 'sh', '-c',
                'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot --default-character-set=utf8mb4 --batch --skip-column-names "$1" -e "$2"',
                '_', database, query]
        return subprocess.check_output(args).decode('utf-8').splitlines()

    columns = {}

    def rows(table, where='1', order='id'):
        assert table in ('conversations', 'conversation_messages', 'assistants', 'assistant_skill_bindings', 'assistant_material_bindings', 'skills')
        if table not in columns:
            columns[table] = sql("SELECT COLUMN_NAME FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='" + table + "' ORDER BY ordinal_position")
        pairs = ','.join("'" + c + "',`" + c + '`' for c in columns[table])
        return [json.loads(bytes.fromhex(line).decode()) for line in sql(f'SELECT HEX(CAST(JSON_OBJECT({pairs}) AS CHAR)) FROM {table} WHERE {where} ORDER BY {order}')]

    def snapshot(key, cid=None, aid=None):
        data = {'conversations': rows('conversations', '1' if cid is None else f'id={int(cid)}'),
                'messages': rows('conversation_messages', '1' if cid is None else f'conversation_id={int(cid)}')}
        if aid is not None:
            data.update(agent=rows('assistants', f'id={int(aid)}'),
                        skill_bindings=rows('assistant_skill_bindings', f'agent_id={int(aid)}', 'skill_id'),
                        material_bindings=rows('assistant_material_bindings', f'agent_id={int(aid)}', 'material_id'))
        state['snapshots'][key] = data
        save()
        return data

    with sync_playwright() as pw:
        browser = pw.chromium.launch(channel='msedge', headless=True)
        state['environment']['browser'] = browser.version
        pages = {}

        def api(who, method, path, data=None, expected=200):
            item = {'utc': datetime.now(timezone.utc).isoformat(), 'actor': who, 'method': method, 'path': path,
                    'input': data, 'expected_status': expected}
            state['requests'].append(item)
            response = pages[who].evaluate('''async ([method,path,data])=>{
              const r=await CampusAPI.request(path,{method,...(data===null?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(data)})});
              const text=await r.text(); let body;try{body=JSON.parse(text)}catch{body=text}
              return {status:r.status,body};}''', [method, path, data])
            item['response'] = response
            save()
            assert response['status'] == expected, f'{who} {method} {path}: expected {expected}, got {response["status"]}'
            return response['body']

        def control(mode=None):
            # Fixture control is deliberately outside CampusAPI's /api/ allowlist.
            # It never carries credentials and is not a production API.
            return pages['T-A'].evaluate('''async mode=>{
              const r=await fetch('/__phase4/control',mode===null?{}:{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode})});
              if(r.status!==200)throw new Error('Controlled gateway control unavailable');
              return await r.json();}''', mode)

        def config(aid):
            return api('T-A', 'GET', f'/api/agents/{aid}')

        def update(aid, **changes):
            current = config(aid)
            value = {k: current[k] for k in ('name', 'prompt', 'enabled', 'skill_ids', 'material_ids')}
            value.update(changes)
            return api('T-A', 'PUT', f'/api/agents/{aid}', value)

        def create(aid):
            return api('S-A1', 'POST', '/api/conversations', {'agent_id': aid}, 201)['id']

        def stream(cid, question, material, selected=False, label=''):
            control('p1p4')
            before = snapshot('before_stream_' + label, cid)
            response = pages['S-A1'].evaluate('''async ([id,content])=>{
              const response=await CampusAPI.request('/api/conversations/'+id+'/messages',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({content})});
              const events=[];const start=performance.now();let text='';
              let panel=document.getElementById('p1p4-live');if(!panel){panel=document.createElement('section');panel.id='p1p4-live';document.body.prepend(panel)}panel.textContent='';
              await CampusStream.consume(response,(kind,payload)=>{events.push({kind,payload,ms:performance.now()-start});if(kind==='delta'){text+=payload.text;panel.textContent=text}});
              return {status:response.status,events,text};}''', [cid, question])
            state['streams'].append({'label': label, 'conversation_id': cid, 'question': question, **response})
            (root / (label + '.sse')).write_text(''.join('event: '+e['kind']+'\ndata: '+json.dumps(e['payload'], ensure_ascii=False)+'\n\n' for e in response['events']), encoding='utf-8')
            after = snapshot('after_stream_' + label, cid)
            captured = control()
            state['streams'][-1]['controlled_gateway'] = captured
            save()
            kinds = [e['kind'] for e in response['events']]
            assert response['status'] == 200 and kinds[0] == 'meta' and kinds[-1] == 'done' and 'error' not in kinds
            assert kinds.count('delta') > 1
            meta, done = response['events'][0]['payload'], response['events'][-1]['payload']
            assert {c['material_id'] for c in meta['citations']} == {material}
            assert {c['material_id'] for c in done['citations']} == {material}
            assert bool(meta['skill_name']) == selected and bool(done['skill_name']) == selected
            message = after['messages'][-1]
            assert message['status'] == 'completed' and message['content'] == response['text']
            assert message['citations'] == done['citations'] and bool(message['skill_name']) == selected
            assert len(after['messages']) == len(before['messages']) + 2 and after['conversations'][0]['active_turn_id'] is None
            assert after['messages'][:-2] == before['messages']
            assert captured['meta_flushed_ns'] < captured['open_invoked_ns'] < captured['first_delta_ns'] < captured['done_ns']
            assert response['events'][1]['ms'] < response['events'][-1]['ms']
            system = captured['last_messages'][0]['content']
            assert ('服务端解题引导边界' in system) == selected
            assert '[1]' in response['text']
            pages['S-A1'].screenshot(path=str(root / (label + '.png')), full_page=True)
            return response

        try:
            for role, username in [('T-A', 'ta'), ('S-A1', 'sa'), ('S-A2', 'sa2'), ('S-B1', 'sb'), ('T-B-setup', 'tb')]:
                context = browser.new_context(viewport={'width': 1280, 'height': 900})
                page = context.new_page()
                page.goto(origin + '/login')
                page.locator('[name=username]').fill(username)
                page.locator('[name=password]').fill(password)
                page.get_by_role('button', name='登录', exact=True).click()
                page.wait_for_url('**/materials')
                page.goto(origin + '/tutor')
                pages[role] = page
            state['identities'] = {r: api(r, 'GET', '/api/auth/me') for r in pages}
            assert state['identities']['S-A1']['user_id'] != state['identities']['S-A2']['user_id']

            def upload(marker, title):
                value = pages['T-A'].evaluate('''async ([text,title])=>{const form=new FormData();form.append('file',new File([text],title,{type:'text/plain'}));const r=await CampusAPI.request('/api/materials',{method:'POST',body:form});return {status:r.status,body:await r.json()};}''', [marker + ' 星河方程 x+2=5，最终答案 x=3。', title])
                state['requests'].append({'actor': 'T-A', 'method': 'POST', 'path': '/api/materials', 'input': {'marker': marker, 'filename': title}, 'expected_status': 201, 'response': value})
                save()
                assert value['status'] == 201 and value['body']['index_status'] == 'ready'
                return value['body']['material_id']

            ma = upload('P1P4-A-RED', 'p1p4-A-red.md')
            mb = upload('P1P4-B-BLUE', 'p1p4-B-blue.md')
            skill = api('T-A', 'POST', '/api/skills', {'source': '名称：课程解题引导\n适用条件：星河,方程\n启用：是\n\n每次只给一个思路或检查问题，不给最终答案或完整解法。'}, 201)['id']
            foreign_skill = api('T-B-setup', 'POST', '/api/skills', {'source': '名称：B班专用技能\n适用条件：海风\n启用：是\n\n仅用于B班。'}, 201)['id']
            agent_a = api('T-A', 'POST', '/api/agents', {'name': 'P1P4-Agent-A', 'prompt': '', 'enabled': True, 'skill_ids': [skill], 'material_ids': [ma]}, 201)['id']
            agent_b = api('T-A', 'POST', '/api/agents', {'name': 'P1P4-Agent-B', 'prompt': '', 'enabled': True, 'skill_ids': [], 'material_ids': [mb]}, 201)['id']
            agent_c = api('T-B-setup', 'GET', f'/api/agents/{fixture["foreign_agent_id"]}')
            assert agent_c['material_ids']
            foreign_material = agent_c['material_ids'][0]
            state['resources'] = {'agent_a': agent_a, 'agent_b': agent_b, 'agent_c': agent_c['id'], 'material_a': ma, 'material_b': mb, 'material_c': foreign_material, 'skill_a': skill, 'skill_c': foreign_skill}
            question = '星河方程 x+2=5，请解题并直接给答案。'

            start('P1.own_create_send')
            cid = create(agent_a)
            state['resources']['c1'] = cid
            stream(cid, question, ma, True, 'P1-own')
            passed(conversation_id=cid)
            start('P1.refresh_read_continue')
            pages['S-A1'].reload()
            own = api('S-A1', 'GET', f'/api/conversations/{cid}')
            assert own['conversation']['agent_id'] == agent_a
            stream(cid, question, ma, True, 'P1-continue')
            passed()
            protected = snapshot('P1-reject-before', cid)
            start('P1.other_list')
            listing = api('S-A2', 'GET', '/api/conversations')
            assert cid not in [x['id'] for x in listing['conversations']]
            passed()
            for key, method, path, data in [('P1.other_read', 'GET', f'/api/conversations/{cid}', None), ('P1.other_send', 'POST', f'/api/conversations/{cid}/messages', {'content': question})]:
                start(key)
                api('S-A2', method, path, data, 404)
                assert snapshot(key + '-after', cid) == protected
                passed(database_unchanged=True)

            start('P2.A_B_bindings')
            bcid = create(agent_b)
            banswer = stream(bcid, question, mb, False, 'P2-agent-B')
            assert 'P1P4-B-BLUE' in banswer['text'] and 'P1P4-A-RED' not in banswer['text']
            assert 'P1P4-A-RED' in state['streams'][0]['text'] and 'P1P4-B-BLUE' not in state['streams'][0]['text']
            passed()
            start('P2.override_agent')
            before = snapshot('P2-override-before', cid)
            api('S-A1', 'POST', f'/api/conversations/{cid}/messages', {'content': question, 'agent_id': agent_b}, 400)
            assert snapshot('P2-override-after', cid) == before
            passed(database_unchanged=True)
            start('P2.disabled_new')
            update(agent_a, enabled=False)
            api('S-A1', 'POST', '/api/conversations', {'agent_id': agent_a}, 404)
            passed()
            start('P2.disabled_continue_current')
            update(agent_a, enabled=False, material_ids=[mb], skill_ids=[])
            stream(cid, question, mb, False, 'P2-disabled-current')
            assert api('S-A1', 'GET', f'/api/conversations/{cid}')['conversation']['agent_id'] == agent_a
            passed(current_material=mb, fixed_agent=agent_a)
            start('P2.no_enabled_400')
            agents = api('T-A', 'GET', '/api/agents')['agents']
            for a in agents:
                if a['enabled']:
                    update(a['id'], enabled=False)
            no_enabled_before = snapshot('P2-no-enabled-before')
            api('S-A1', 'POST', '/api/conversations', {'agent_id': agent_a}, 400)
            assert snapshot('P2-no-enabled-after') == no_enabled_before
            for a in agents:
                if a['enabled']:
                    update(a['id'], enabled=True)
            update(agent_a, enabled=True, material_ids=[ma], skill_ids=[skill])
            passed()
            start('P2.history_preserved')
            assert snapshot('P2-old-history-after', cid)['messages'][:len(before['messages'])] == before['messages']
            passed()

            global_before = snapshot('P3-before', aid=agent_a)
            deny = [('P3.foreign_read', 'S-B1', 'GET', f'/api/conversations/{cid}', None, 404),
                    ('P3.foreign_send', 'S-B1', 'POST', f'/api/conversations/{cid}/messages', {'content': question}, 404),
                    ('P3.foreign_agent', 'S-B1', 'GET', f'/api/agents/{agent_a}', None, 404),
                    ('P3.foreign_skill', 'S-B1', 'GET', f'/api/skills/{skill}', None, 404),
                    ('P3.foreign_material', 'S-B1', 'GET', f'/api/materials/{ma}', None, 404),
                    ('P3.foreign_download', 'S-B1', 'GET', f'/api/materials/{ma}/file', None, 404),
                    ('P3.foreign_reference', 'S-B1', 'GET', '/api/materials/'+state['streams'][0]['events'][-1]['payload']['citations'][0]['material_id']+'/file', None, 404),
                    ('P3.foreign_create', 'S-A1', 'POST', '/api/conversations', {'agent_id': agent_c['id']}, 404),
                    ('P3.class_override', 'S-A1', 'POST', f'/api/conversations/{cid}/messages', {'content': question, 'class_id': 2}, 400),
                    ('P3.user_override', 'S-A1', 'POST', f'/api/conversations/{cid}/messages', {'content': question, 'user_id': state['identities']['S-A2']['user_id']}, 400)]
            for key, actor, method, path, data, expected in deny:
                start(key)
                api(actor, method, path, data, expected)
                assert snapshot(key + '-after', aid=agent_a) == global_before
                passed(database_unchanged=True)
            for key, change in [('P3.material_binding_atomic', {'material_ids': [ma, foreign_material]}), ('P3.skill_binding_atomic', {'skill_ids': [skill, foreign_skill]})]:
                start(key)
                a = config(agent_a)
                data = {k: a[k] for k in ('name', 'prompt', 'enabled', 'skill_ids', 'material_ids')}
                data.update(change)
                api('T-A', 'PUT', f'/api/agents/{agent_a}', data, 404)
                assert config(agent_a) == a
                assert snapshot(key + '-after', aid=agent_a) == global_before
                passed(full_configuration_unchanged=True)
            start('P3.no_side_effects')
            assert snapshot('P3-after', aid=agent_a) == global_before
            passed()

            p4before = snapshot('P4-history-before')
            p4ids = []
            for key, enabled, bound in [('P4.enabled', True, True), ('P4.disabled', False, True), ('P4.unbound', True, False)]:
                start(key)
                api('T-A', 'PUT', f'/api/skills/{skill}', {'enabled': enabled})
                update(agent_a, material_ids=[ma], skill_ids=[skill] if bound else [])
                pcid = create(agent_a)
                p4ids.append(pcid)
                answer = stream(pcid, question, ma, enabled and bound, key)
                if enabled and bound:
                    assert '请先思考' in answer['text'] and 'x=3' not in answer['text']
                else:
                    assert 'x=3' in answer['text']
                passed(conversation_id=pcid, controlled_model_only=True)
            start('P4.history_preserved')
            after = snapshot('P4-history-after')
            old = {m['id']: m for m in p4before['messages']}
            assert all(old[m['id']] == m for m in after['messages'] if m['id'] in old)
            first = next(s for s in state['streams'] if s['label'] == 'P4.enabled')
            first_message = rows('conversation_messages', f'conversation_id={p4ids[0]}')[-1]
            assert first_message['content'] == first['text'] and first_message['skill_name'] == '课程解题引导'
            passed(old_message_count=len(old), enabled_snapshot_retained=True)
            assert hashlib.sha256(budget.read_bytes()).hexdigest() == budget_hash
            state['budget_sha256_unchanged'] = budget_hash
            state['status'] = 'passed'
            state['counts'] = {k: sum(s['status'] == k for s in state['steps']) for k in ('passed', 'failed', 'not_executed')}
            save()
        except Exception as exc:
            if active:
                active.update(status='failed', error=type(exc).__name__ + ': ' + str(exc))
            state['status'] = 'failed'
            state['counts'] = {k: sum(s['status'] == k for s in state['steps']) for k in ('passed', 'failed', 'not_executed')}
            save()
            raise
        finally:
            browser.close()


if __name__ == '__main__':
    main()
