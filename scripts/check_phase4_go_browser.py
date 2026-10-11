"""Real isolated browser acceptance; invoked only by the Phase 4 test harness."""
import os
from pathlib import Path
import sys
import re
import traceback
from urllib.parse import urlparse

from playwright.sync_api import expect, sync_playwright


def main():
    origin = os.environ['PHASE4_UI_ORIGIN']
    parsed = urlparse(origin)
    assert parsed.hostname == '127.0.0.1' and parsed.port not in (80, 443, 8080)
    assert os.environ.get('PHASE4_UI_ISOLATED') == '1'
    password = os.environ['PHASE4_UI_PASSWORD']
    with sync_playwright() as engine:
        browser = engine.chromium.launch(channel='msedge', headless=True)
        context = browser.new_context(accept_downloads=True)
        context.add_cookies([{'name': 'session', 'value': 'obsolete', 'url': origin}])
        context.add_init_script("""(() => {
          const original = window.fetch;
          window.requestOptions = [];
          window.fetch = (url, options) => {
            window.requestOptions.push({credentials: options.credentials, redirect: options.redirect});
            return original(url, options);
          };
          const revoke = URL.revokeObjectURL;
          window.revokedBlobs = 0;
          URL.revokeObjectURL = value => { window.revokedBlobs++; return revoke(value); };
        })();""")
        page = context.new_page()
        requests, errors = [], []
        page.on('pageerror', lambda error: errors.append(type(error).__name__))

        def record(request):
            if '/api/' in request.url:
                headers = request.all_headers()
                requests.append(dict(path=request.url.split(origin)[-1], method=request.method,
                                     bearer=headers.get('authorization', '').startswith('Bearer '),
                                     cookie='cookie' in headers))

        page.on('request', record)

        def login(username):
            page.goto(origin + '/login')
            expect(page.locator('h2')).to_have_text('登录 CampusClaw')
            assert page.locator('.login-logo img').evaluate('(el) => el.complete && el.naturalWidth > 0')
            page.locator('[name=username]').fill(username)
            page.locator('[name=password]').fill(password)
            page.get_by_role('button', name='登录', exact=True).click()
            page.wait_for_url('**/materials')
            expect(page.locator('#role')).to_have_text('教师' if username.startswith('teacher') else '学生')
            expect(page.locator('#materials tr').first).to_be_visible()
            expect(page.locator('[data-class]').first).to_contain_text('A' if '_a' in username else 'B')
            assert page.locator('a[href="/knowledge"]').count() == 0
            assert page.evaluate('CampusAPI.hasToken()')
            assert page.evaluate("localStorage.getItem('campusclaw.token') === null")

        page.goto(origin + '/materials')
        page.wait_for_url('**/login')
        login('teacher_a')
        expect(page.locator('#upload-card')).to_be_visible()
        expect(page.locator('#materials')).to_contain_text('中文材料-<img')
        assert page.locator('#materials img').count() == 0
        assert page.evaluate('window.pwned === undefined')
        page.reload()
        expect(page.locator('#role')).to_have_text('教师')

        content = '\ufeff中文浏览器原文\n<script>window.pwned=true</script>'.encode('utf-8')
        page.locator('[name=file]').set_input_files({'name': 'browser-中文.md', 'mimeType': 'text/plain', 'buffer': content})
        page.get_by_role('button', name='上传', exact=True).click()
        expect(page.locator('#message')).to_have_text('上传成功。')
        row = page.locator('#materials tr').filter(has_text='browser-.md')
        expect(row).to_have_count(1)
        row.get_by_role('button', name='阅读文本').click()
        expect(page.locator('#text')).to_have_text(content.decode('utf-8-sig'))
        assert page.evaluate('window.pwned === undefined')
        with page.expect_download() as event:
            row.get_by_role('button', name='下载', exact=True).click()
        download = event.value
        assert download.suggested_filename == 'browser-.md'
        assert Path(download.path()).read_bytes() == content
        assert download.url.startswith('blob:')
        # Do not ask the browser to eval a string under the real strict CSP.
        for _ in range(30):
            if page.evaluate('() => window.revokedBlobs > 0'):
                break
            page.wait_for_timeout(100)
        assert page.evaluate('() => window.revokedBlobs > 0')
        options = page.evaluate('window.requestOptions')
        assert options and all(item == {'credentials': 'omit', 'redirect': 'error'} for item in options)
        old = page.evaluate("sessionStorage.getItem('campusclaw.token')")
        page.get_by_role('button', name='退出登录').click()
        page.wait_for_url('**/login')
        assert page.evaluate("sessionStorage.getItem('campusclaw.token') === null")
        assert context.request.get(origin + '/api/auth/me', headers={'Authorization': 'Bearer ' + old}).status == 401

        for username in ('student_a1', 'teacher_b', 'student_b1'):
            login(username)
            if username.startswith('student'):
                expect(page.locator('#upload-card')).to_be_hidden()
                expect(page.locator('#student-note')).to_be_visible()
                assert page.evaluate("async () => (await CampusAPI.request('/api/materials', {method:'POST'})).status") == 403
                assert page.url.endswith('/materials') and page.evaluate('CampusAPI.hasToken()')
            else:
                expect(page.locator('#upload-card')).to_be_visible()
            if '_b' in username:
                expect(page.locator('#materials')).not_to_contain_text('browser-.md')
            page.locator('#materials tr').first.get_by_role('button', name='阅读文本').click()
            expect(page.locator('#text')).not_to_be_empty()
        # An API 403 reports insufficient permission and leaves the login intact.
        page.route('**/api/materials/*/file', lambda route: route.fulfill(status=403, content_type='application/json', body='{"error":"Teacher role required"}'))
        page.locator('#materials tr').first.get_by_role('button', name='下载', exact=True).click()
        expect(page.locator('#message')).to_have_text('权限不足。')
        assert page.evaluate('CampusAPI.hasToken()') and page.url.endswith('/materials')
        page.unroute('**/api/materials/*/file')
        assert page.evaluate("async () => { try { await CampusAPI.request('https://example.com/api/steal'); return false; } catch { return true; } }")

        page.route('**/api/auth/logout', lambda route: route.abort())
        page.get_by_role('button', name='退出登录').click()
        page.wait_for_url('**/login')
        expect(page.locator('#message')).to_contain_text('服务端退出未确认')
        assert page.evaluate("sessionStorage.getItem('campusclaw.token') === null")
        page.unroute('**/api/auth/logout')
        login('teacher_a')
        token = page.evaluate("sessionStorage.getItem('campusclaw.token')")
        assert context.request.post(origin + '/api/auth/logout', headers={'Authorization': 'Bearer ' + token}).status == 204
        page.reload()
        page.wait_for_url('**/login')
        assert page.evaluate("sessionStorage.getItem('campusclaw.token') === null")
        assert page.locator('#materials').count() == 0
        protected = [item for item in requests if item['path'] != '/api/auth/login']
        assert protected and all(item['bearer'] and not item['cookie'] for item in protected)
        assert all('?' not in item['path'] for item in requests)
        for path in ('/api/auth/me', '/api/materials', '/api/auth/logout'):
            assert any(item['path'] == path for item in protected)
        assert any(item['path'].endswith('/file') for item in protected)
        assert not errors
        context.close()
        browser.close()
    print('PASS real Edge browser: Chinese pages, both roles/classes, upload/read/Blob download, XSS text, Bearer-only, 403, logout failure and 401 clearing')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Do not print Playwright traces that may include credential/token input.
        frames = [f for f in traceback.extract_tb(error.__traceback__) if Path(f.filename).resolve() == Path(__file__).resolve()]
        detail = str(error)
        for key in ('PHASE4_UI_PASSWORD', 'JWT_SIGNING_SECRET', 'DB_PASSWORD', 'MYSQL_ROOT_PASSWORD'):
            if os.environ.get(key):
                detail = detail.replace(os.environ[key], '[redacted]')
        detail = re.sub(r'eyJ[\w-]+\.[\w-]+\.[\w-]+', '[redacted-token]', detail)
        print('FAIL isolated browser workflow: ' + type(error).__name__ + ' at line ' + str(frames[-1].lineno if frames else 0))
        print(detail[:1800])
        sys.exit(1)
