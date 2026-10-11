"""Real isolated Chromium + HTTP server; no deployed app or existing browser profile."""
from contextlib import closing
from pathlib import Path
from threading import Thread

import pytest
from playwright.sync_api import expect, sync_playwright
from werkzeug.serving import make_server

from app.db import connect


@pytest.fixture
def browser_session(token_app):
    server = make_server('127.0.0.1', 0, token_app, threaded=True)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    origin = f'http://127.0.0.1:{server.server_port}'
    with sync_playwright() as engine:
        browser = engine.chromium.launch(channel='msedge', headless=True)
        context = browser.new_context(accept_downloads=True)
        # A real ambient cookie proves that credentials: omit applies to fetch.
        context.add_cookies([{'name': 'session', 'value': 'obsolete-cookie', 'url': origin}])
        context.add_init_script("""(() => {
          const original = window.fetch;
          window.requestOptions = [];
          window.fetch = (url, options) => {
            window.requestOptions.push({credentials: options.credentials, redirect: options.redirect});
            return original(url, options);
          };
        })();"""
        )
        page = context.new_page()
        requests, errors = [], []
        page.on('pageerror', lambda error: errors.append(str(error)))
        def record(request):
            if '/api/' in request.url:
                headers = request.all_headers()
                requests.append(dict(path=request.url.split(origin)[-1], method=request.method,
                                     bearer=headers.get('authorization', '').startswith('Bearer '),
                                     cookie='cookie' in headers))
        page.on('request', record)
        try:
            yield page, context, origin, requests, errors
        finally:
            context.close()
            browser.close()
            server.shutdown()
            thread.join(timeout=5)
            server.server_close()


def sign_in(page, origin, password, username='teacher_a'):
    page.goto(origin + '/login')
    page.locator('[name=username]').fill(username)
    page.locator('[name=password]').fill(password)
    page.get_by_role('button', name='登录', exact=True).click()
    page.wait_for_url('**/materials')
    expect(page.locator('#role')).to_have_text('教师' if username.startswith('teacher') else '学生')
    expect(page.locator('#materials')).to_contain_text('本班暂无材料。')


def test_real_browser_login_upload_read_download_logout(token_app, browser_session):
    page, context, origin, requests, errors = browser_session
    page.goto(origin + '/materials')
    page.wait_for_url('**/login')
    sign_in(page, origin, token_app.test_password)
    assert page.evaluate("Boolean(sessionStorage.getItem('campusclaw.token'))")
    assert page.evaluate("localStorage.getItem('campusclaw.token') === null")
    page.reload()
    expect(page.locator('#role')).to_have_text('教师')
    content = b'<script>window.pwned = true</script>\nClass A text'
    page.locator('[name=file]').set_input_files({'name': 'browser.txt', 'mimeType': 'text/plain', 'buffer': content})
    page.get_by_role('button', name='上传', exact=True).click()
    expect(page.locator('#materials')).to_contain_text('browser.txt')
    page.get_by_role('button', name='阅读文本').click()
    expect(page.locator('#text')).to_have_text(content.decode())
    assert page.evaluate('window.pwned === undefined')
    with page.expect_download() as event:
        page.get_by_role('button', name='下载', exact=True).click()
    download = event.value
    assert download.suggested_filename == 'browser.txt'
    assert Path(download.path()).read_bytes() == content
    assert download.url.startswith('blob:')
    # Filename rendering is also data-only, even with untrusted historical metadata.
    with closing(connect(token_app.config['DATABASE'])) as db, db:
        db.execute('UPDATE materials SET original_name=?', ('<img src=x onerror="window.pwned=true">.txt',))
    page.reload()
    expect(page.locator('#materials')).to_contain_text('<img src=x')
    assert page.locator('#materials img').count() == 0
    assert page.evaluate('window.pwned === undefined')
    options = page.evaluate('window.requestOptions')
    assert options and all(item == {'credentials': 'omit', 'redirect': 'error'} for item in options)
    token = page.evaluate("sessionStorage.getItem('campusclaw.token')")
    page.get_by_role('button', name='退出登录').click()
    page.wait_for_url('**/login')
    assert page.evaluate("sessionStorage.getItem('campusclaw.token') === null")
    assert context.request.get(origin + '/api/auth/me', headers={'Authorization': 'Bearer ' + token}).status == 401
    protected = [item for item in requests if item['path'] != '/api/auth/login']
    assert protected and all(item['bearer'] and not item['cookie'] for item in protected)
    assert all('?' not in item['path'] for item in requests)
    assert any(item['path'].endswith('/file') for item in protected)
    assert any(item['path'] == '/api/materials' and item['method'] == 'POST' for item in protected)
    assert any(item['path'] == '/api/auth/logout' for item in protected)
    assert not errors


def test_real_browser_403_and_failed_logout(token_app, browser_session):
    page, context, origin, requests, errors = browser_session
    sign_in(page, origin, token_app.test_password, 'student_a1')
    expect(page.locator('#upload-card')).to_be_hidden()
    result = page.evaluate("async () => (await CampusAPI.request('/api/materials', {method: 'POST'})).status")
    assert result == 403
    assert page.url.endswith('/materials') and page.evaluate('CampusAPI.hasToken()')
    assert page.evaluate("async () => { try { await CampusAPI.request('https://example.com/api/steal'); return false; } catch { return true; } }")
    page.route('**/api/auth/logout', lambda route: route.abort())
    page.get_by_role('button', name='退出登录').click()
    page.wait_for_url('**/login')
    expect(page.locator('#message')).to_contain_text('服务端退出未确认')
    assert page.evaluate("sessionStorage.getItem('campusclaw.token') === null")
    assert not errors


def test_real_browser_revocation_401_clears_state(token_app, browser_session):
    page, context, origin, requests, errors = browser_session
    sign_in(page, origin, token_app.test_password)
    token = page.evaluate("sessionStorage.getItem('campusclaw.token')")
    assert context.request.post(origin + '/api/auth/logout', headers={'Authorization': 'Bearer ' + token}).status == 204
    page.reload()
    page.wait_for_url('**/login')
    assert page.evaluate("sessionStorage.getItem('campusclaw.token') === null")
    assert page.locator('#materials').count() == 0
    assert not errors
