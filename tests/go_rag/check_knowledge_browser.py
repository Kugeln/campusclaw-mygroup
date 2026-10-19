"""Knowledge UI against isolated Go/MySQL/Qdrant via Nginx; no model calls."""
import hashlib
import json
import os
from pathlib import Path
from urllib.parse import urlparse, parse_qs
from playwright.sync_api import sync_playwright, expect


def main():
    root = Path(__file__).resolve().parents[2]
    origin = os.environ['WEEK5_BROWSER_ORIGIN']
    evidence = Path(os.environ['WEEK5_BROWSER_RESULT']).parent
    fixture = json.loads(Path(os.environ['WEEK5_BROWSER_READY']).read_text(encoding='utf-8-sig'))
    checks, requests, errors = [], [], []
    before = root / 'runtime/ui-round2/before'
    with sync_playwright() as p:
        browser = p.chromium.launch(channel='msedge', headless=True)
        def login(user):
            context = browser.new_context(viewport={'width': 1440, 'height': 1000})
            page = context.new_page()
            page.on('pageerror', lambda e: errors.append(str(e)))
            page.on('request', lambda r: requests.append({'path':urlparse(r.url).path, 'query':urlparse(r.url).query, 'bearer':r.headers.get('authorization','').startswith('Bearer '), 'cookie':'cookie' in r.headers}) if '/api/' in r.url else None)
            page.goto(origin+'/login')
            page.locator('[name=username]').fill(user)
            page.locator('[name=password]').fill(os.environ['WEEK5_ACCOUNT_PASSWORD'])
            page.get_by_role('button', name='登录', exact=True).click()
            page.wait_for_url('**/materials')
            return context, page
        context, page = login('student_a1')
        # Replay only the pre-edit front-end assets against the same real API.
        baseline_routes = [('**/knowledge', 'knowledge.html', 'text/html'), ('**/static/knowledge.js','knowledge.js','text/javascript'), ('**/static/tutor.css','tutor.css','text/css')]
        for pattern, file, kind in baseline_routes:
            page.route(pattern, lambda route, request, file=file, kind=kind: route.fulfill(path=str(before/file), content_type=kind))
        page.goto(origin+'/knowledge'); page.screenshot(path=str(evidence/'knowledge-before-desktop.png'), full_page=True)
        baseline = page.evaluate("({background:getComputedStyle(document.documentElement).backgroundColor, blue:getComputedStyle(document.documentElement).getPropertyValue('--blue'), font:getComputedStyle(document.body).fontFamily})")
        page.set_viewport_size({'width':390,'height':844}); page.screenshot(path=str(evidence/'knowledge-before-mobile.png'), full_page=True)
        for pattern, _, _ in baseline_routes: page.unroute(pattern)
        page.set_viewport_size({'width':1440,'height':1000})
        page.goto(origin+'/knowledge')
        expect(page.locator('#lecture-cards button').first).to_be_visible()
        expect(page.locator('#mode')).to_have_value('hybrid')
        expect(page.locator('[data-class]').first).not_to_have_text('正在确认班级…')
        assert page.evaluate("({background:getComputedStyle(document.documentElement).backgroundColor, blue:getComputedStyle(document.documentElement).getPropertyValue('--blue'), font:getComputedStyle(document.body).fontFamily})") == baseline
        assert page.locator('.materials-logo').get_attribute('src') == '/static/logo.png'
        assert page.locator('#search-button').evaluate('(e)=>getComputedStyle(e).backgroundColor') == 'rgb(37, 99, 235)'
        page.screenshot(path=str(evidence/'knowledge-after-desktop.png'), full_page=True)
        checks.append('initial real class/library, unchanged background/font/brand blue/logo')
        material_id = fixture['material_id']
        token = page.evaluate("sessionStorage.getItem('campusclaw.token')")
        real = context.request.get(origin+'/api/materials/'+material_id, headers={'Authorization':'Bearer '+token}).json()
        listing = context.request.get(origin+'/api/materials', headers={'Authorization':'Bearer '+token}).json()['items']
        assert len(listing) >= 30, 'At least 30 existing isolated records required for pagination cases'
        # Boundary cases use subsets of real existing records, not invented material IDs.
        for count in [0, 1, 9, 10, 30]:
            subset = listing[:count]
            page.route('**/api/materials', lambda route, request, subset=subset: route.fulfill(json={'items':subset}))
            page.locator('#reload-materials').click()
            expect(page.locator('#lecture-cards button')).to_have_count(min(count,9))
            expect(page.locator('#lecture-cards')).to_have_attribute('aria-busy','false')
            if count <= 9:
                expect(page.locator('#library-pagination')).to_be_hidden()
            else:
                expect(page.locator('#library-pagination')).to_be_visible()
                seen = []
                initial_height = page.locator('#lecture-cards').bounding_box()['height']
                pages = (count+8)//9
                for number in range(pages):
                    expect(page.locator('#page-status')).to_contain_text(f'第 {number+1} / {pages} 页')
                    seen.extend(page.locator('#lecture-cards button').evaluate_all('(items)=>items.map(e=>e.dataset.materialId)'))
                    assert page.locator('#lecture-cards').bounding_box()['height'] == initial_height, (count,number,initial_height,page.locator('#lecture-cards').bounding_box()['height'])
                    if number < pages-1: page.locator('#next-page').click()
                assert seen == [item['id'] for item in subset]
                expect(page.locator('#next-page')).to_be_disabled()
                page.locator('#previous-page').click()
            page.unroute('**/api/materials')
        checks.append('0/1/9/10/30 pagination, exact real IDs/counts, controls and stable desktop grid height')
        same_name = [item for item in listing if item['original_name'] == 'browser-source.md'][:2]
        assert len(same_name) == 2 and same_name[0]['id'] != same_name[1]['id']
        page.route('**/api/materials', lambda route: route.fulfill(json={'items':same_name}))
        page.locator('#reload-materials').click(); expect(page.locator('#lecture-cards button')).to_have_count(2)
        for item in same_name:
            with page.expect_response(lambda r, item=item: urlparse(r.url).path == '/api/materials/'+item['id']) as detail:
                page.locator('[data-material-id="'+item['id']+'"]').click()
            expect(page.locator('#source-text')).to_have_text(detail.value.json()['plain_text'])
            page.locator('#close-source').click()
        page.unroute('**/api/materials'); page.locator('#reload-materials').click()
        expect(page.locator('#page-status')).to_contain_text('第 1 /')
        checks.append('same name different IDs both readable; reload resets first page')
        title = next(item['original_name'] for item in listing if item['id'] == material_id)
        page.locator('[data-material-id="'+material_id+'"]').click()
        expect(page.locator('#source-text')).to_have_text(real['plain_text'])
        page.locator('#close-source').click()
        checks.append('real material detail with permission checked API')
        for mode in ['keyword','vector','hybrid']:
            page.locator('#mode').select_option(mode); page.locator('#question').fill('星河')
            with page.expect_response(lambda r: '/api/search?' in r.url) as response:
                page.locator('#question').press('Enter')
            result = response.value.json()
            assert parse_qs(urlparse(response.value.url).query)['mode'] == [mode]
            assert result['items']
            expect(page.locator('.knowledge-result')).to_have_count(len(result['items']))
            expect(page.locator('.knowledge-snippet').first).to_have_text(result['items'][0]['chunk_text'])
            snippet = page.locator('.knowledge-snippet').first
            toggle = page.locator('.snippet-toggle').first
            expect(toggle).to_be_visible()
            assert snippet.evaluate('(e)=>e.clientHeight <= parseFloat(getComputedStyle(e).lineHeight)*4+1')
            expect(snippet.locator('mark').first).to_have_text('星河')
            toggle.click(); expect(toggle).to_have_text('收起')
            assert snippet.evaluate('(e)=>e.clientHeight >= e.scrollHeight-1')
            expect(snippet).to_have_text(result['items'][0]['chunk_text'])
            toggle.click(); expect(toggle).to_have_text('展开全文')
            page.locator('.knowledge-result').first.get_by_role('button', name='查看来源 [1]',exact=True).click()
            expect(page.locator('#source-status')).to_contain_text('下方为材料原文')
            expect(page.locator('#source-text')).not_to_be_empty()
            page.locator('#close-source').click()
        checks.append('Enter search uses all 3 real modes and real snippets/scores/sources')
        page.screenshot(path=str(evidence/'knowledge-results-desktop.png'),full_page=True)
        page.locator('.snippet-toggle').first.click()
        page.locator('#search-button').click(); expect(page.locator('.snippet-toggle').first).to_have_attribute('aria-expanded','false')
        checks.append('4-line summaries, expansion/collapse, exact raw text retained, query highlighting and search resets')
        page.set_viewport_size({'width':390,'height':844})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
        page.screenshot(path=str(evidence/'knowledge-after-mobile.png'),full_page=True)
        page.evaluate('window.scrollTo(0,0)'); page.screenshot(path=str(evidence/'knowledge-after-390.png'))
        page.set_viewport_size({'width':600,'height':900}); assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
        page.screenshot(path=str(evidence/'knowledge-after-600.png'))
        page.set_viewport_size({'width':390,'height':844})
        checks.append('desktop and 390px no horizontal overflow')
        page.locator('#question').fill('星河'); page.locator('#ask-button').click()
        expect(page.locator('#search-button')).to_be_enabled()
        expect(page.locator('#answer')).not_to_be_empty()
        expect(page.locator('.knowledge-result').first).to_be_visible()
        checks.append('existing one-shot ask preserved')
        # Controlled transport faults validate UI states; never invent displayed materials.
        pending = []
        page.route('**/api/search?*', lambda route: pending.append(route))
        page.locator('#search-button').click()
        expect(page.locator('#search-button')).to_be_disabled()
        expect(page.locator('#mode')).to_be_disabled()
        page.wait_for_timeout(100); assert len(pending) == 1
        pending[0].continue_(); page.unroute('**/api/search?*')
        expect(page.locator('#search-button')).to_be_enabled()
        checks.append('loading disables duplicate submits and mode changes')
        page.locator('#mode').select_option('keyword'); page.locator('#question').fill('nonexistent_unique_knowledge_ui_987654')
        page.locator('#search-button').click(); expect(page.locator('#answer')).to_have_text('资料中未找到相关内容')
        expect(page.locator('.knowledge-result')).to_have_count(0)
        checks.append('real keyword no-hit contract')
        page.route('**/api/search?*', lambda route: route.fulfill(status=403, json={'error':'Forbidden'}))
        page.locator('#search-button').click(); expect(page.locator('#notice')).to_contain_text('权限不足')
        expect(page.locator('#search-button')).to_be_enabled(); page.unroute('**/api/search?*')
        page.route('**/api/search?*', lambda route: route.abort())
        page.locator('#search-button').click(); expect(page.locator('#notice')).not_to_be_empty()
        expect(page.locator('#search-button')).to_be_enabled(); page.unroute('**/api/search?*')
        page.route('**/api/materials', lambda route: route.fulfill(json={'items':[]}))
        page.locator('#reload-materials').click(); expect(page.locator('#materials-status')).to_have_text('本班暂无材料。')
        page.unroute('**/api/materials')
        page.route('**/api/materials', lambda route: route.fulfill(status=503, json={'error':'Unavailable'}))
        page.locator('#reload-materials').click(); expect(page.locator('#materials-status')).to_contain_text('请求失败')
        page.unroute('**/api/materials'); page.locator('#reload-materials').click()
        expect(page.locator('#lecture-cards button').first).to_be_visible()
        checks.append('403/network/library empty/error/retry transport state tests')
        # Hostile text is a controlled display-only fixture to test HTML injection protection.
        hostile = '<img src=x onerror=window.injected=true> 星河 <script>window.injected=true</script>'
        sample = result['items'][0].copy(); sample['chunk_text'] = hostile
        page.route('**/api/search?*',lambda route: route.fulfill(json={'items':[sample], 'mode':'keyword'}))
        page.locator('#question').fill('星河'); page.locator('#search-button').click()
        expect(page.locator('.knowledge-snippet')).to_have_text(hostile)
        assert page.locator('.knowledge-snippet img, .knowledge-snippet script').count() == 0
        assert page.evaluate('window.injected === undefined')
        expect(page.locator('.snippet-toggle')).to_be_hidden()
        page.unroute('**/api/search?*')
        checks.append('literal highlighting blocks HTML injection; short snippet hides toggle')
        bc, b = login('student_b1'); b.goto(origin+'/knowledge')
        expect(b.locator('#lecture-cards')).to_have_attribute('aria-busy','false')
        b_listing = bc.request.get(origin+'/api/materials',headers={'Authorization':'Bearer '+b.evaluate("sessionStorage.getItem('campusclaw.token')")}).json()['items']
        assert material_id not in [item['id'] for item in b_listing]
        bt = b.evaluate("sessionStorage.getItem('campusclaw.token')")
        assert bc.request.get(origin+'/api/materials/'+material_id,headers={'Authorization':'Bearer '+bt}).status == 404
        b.locator('#mode').select_option('keyword'); b.locator('#question').fill('星河'); b.locator('#search-button').click()
        expect(b.locator('#answer')).to_have_text('资料中未找到相关内容')
        checks.append('real B-class list/search/detail isolation')
        # Targeted unchanged tutor SSE smoke; never calls the real model gateway.
        _, tutor = login('student_a1'); tutor.goto(origin+'/tutor')
        assert tutor.locator('#student').evaluate('(e)=>getComputedStyle(e).gridTemplateColumns.split(" ").length') == 2
        tutor.screenshot(path=str(evidence/'tutor-after-desktop.png'))
        for width in [600,390]:
            tutor.set_viewport_size({'width':width,'height':844})
            expect(tutor.locator('#conversation-disclosure')).not_to_have_attribute('open','')
            tutor.locator('summary').click(); expect(tutor.locator('#conversations')).to_be_visible()
            assert tutor.evaluate('document.documentElement.scrollWidth <= innerWidth')
            tutor.locator('summary').click()
            assert tutor.evaluate('document.documentElement.scrollWidth <= innerWidth')
        tutor.locator('summary').click()
        tutor.locator('#agent').select_option(str(fixture['agent_id'])); tutor.locator('#create').click()
        expect(tutor.locator('#notice')).to_have_text('会话已创建。')
        expect(tutor.locator('#conversations')).to_be_hidden()
        tutor.locator('#question').fill('星河如何解题'); tutor.locator('#send-button').click()
        expect(tutor.locator('.message[data-state=generating] p')).to_have_text('先分析条件，', timeout=2000)
        expect(tutor.locator('.message[data-state=completed] .state').last).to_contain_text('已完成',timeout=8000)
        tutor.locator('.sources button').first.click(); expect(tutor.locator('#source-text')).to_contain_text('星河解题先分析')
        checks.append('unchanged tutor real incremental SSE completion and citation smoke')
        tutor.set_viewport_size({'width':390,'height':844})
        current_width = tutor.evaluate('document.documentElement.scrollWidth'); assert current_width == 390
        tutor.evaluate('window.scrollTo(0,0)'); tutor.screenshot(path=str(evidence/'tutor-after-390.png'))
        # Display-only stress test for unbroken strings in citations and messages.
        tutor.evaluate("""() => {
          const text = document.querySelector('.message p'); text.dataset.original = text.textContent; text.textContent = 'longword'.repeat(80);
          const button = document.querySelector('.sources button'); button.dataset.original = button.textContent; button.textContent = 'long-source-title'.repeat(30);
        }""")
        assert tutor.evaluate('document.documentElement.scrollWidth <= innerWidth')
        tutor.evaluate("""() => { for (const e of document.querySelectorAll('[data-original]')) { e.textContent=e.dataset.original; delete e.dataset.original; } }""")
        checks.append('390px unbroken long message/citation display wraps without horizontal overflow')
        tutor.locator('summary').click()
        assert tutor.locator('#conversations button[aria-current=true]').count() == 1
        tutor.locator('#conversations button').nth(1).click(); expect(tutor.locator('#conversations')).to_be_hidden()
        tutor.locator('summary').click(); tutor.locator('#conversations button').first.click()
        expect(tutor.locator('.message')).to_have_count(2)
        tutor.set_viewport_size({'width':600,'height':844}); assert tutor.evaluate('document.documentElement.scrollWidth <= innerWidth')
        tutor.evaluate('window.scrollTo(0,0)'); tutor.screenshot(path=str(evidence/'tutor-after-600.png'))
        tutor.set_viewport_size({'width':1440,'height':1000}); expect(tutor.locator('#conversations')).to_be_visible()
        tutor.evaluate('window.scrollTo(0,0)'); tutor.screenshot(path=str(evidence/'tutor-after-desktop-conversation.png'))
        checks.append('390/600/desktop, collapse/open/create/switch/history/current selection; no overflow')
        for pattern,file,kind in [('**/tutor','tutor.html','text/html'),('**/static/tutor.js','tutor.js','text/javascript'),('**/static/tutor.css','tutor.css','text/css')]:
            tutor.route(pattern,lambda route,request,file=file,kind=kind: route.fulfill(path=str(before/file),content_type=kind))
        tutor.set_viewport_size({'width':390,'height':844})
        tutor.reload(); tutor.locator('#conversations button').first.click()
        expect(tutor.locator('.message')).to_have_count(2)
        tutor.locator('.sources button').first.click(); expect(tutor.locator('#source-text')).to_contain_text('星河解题先分析')
        baseline_width = tutor.evaluate('document.documentElement.scrollWidth')
        tutor.evaluate('window.scrollTo(0,0)'); tutor.screenshot(path=str(evidence/'tutor-before-390.png'))
        tutor.set_viewport_size({'width':1440,'height':1000}); tutor.evaluate('window.scrollTo(0,0)'); tutor.screenshot(path=str(evidence/'tutor-before-desktop.png'))
        checks.append(f'tutor mobile overflow fixed: before {baseline_width}px, after {current_width}px at 390px viewport')
        page.route('**/api/search?*', lambda route: route.fulfill(status=401, json={'error':'Unauthorized'}))
        page.locator('#search-button').click(); page.wait_for_url('**/login')
        assert page.evaluate("sessionStorage.getItem('campusclaw.token')") is None
        checks.append('401 clears bearer session and redirects')
        assert all(r['bearer'] and not r['cookie'] for r in requests if r['path'] not in ['/api/auth/login'])
        assert not errors, errors
        checks.append('API Bearer only; no JavaScript errors')
        browser.close()
    protected = json.loads((root/'runtime/ui-round2/protected-hashes.json').read_text(encoding='utf-8-sig'))
    assert all(hashlib.sha256((root/item['path']).read_bytes()).hexdigest().upper() == item['hash'] for item in protected)
    checks.append('public CSS/logo/auth/SSE/other page templates/OpenSpec unchanged by hash')
    (evidence/'knowledge-browser.json').write_text(json.dumps({'passed':True,'checks':checks,'theme':baseline,'model_calls':0},ensure_ascii=False,indent=2),encoding='utf-8')
    print('PASS knowledge UI: real APIs, modes, citations, isolation, states, responsive theme; tutor/SSE smoke passed')


if __name__ == '__main__': main()
