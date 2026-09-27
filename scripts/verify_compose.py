"""Verify the existing local Compose deployment without printing credentials."""
import argparse
from http.cookiejar import CookieJar
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
from urllib.error import HTTPError
from urllib.request import build_opener, HTTPCookieProcessor, HTTPRedirectHandler, Request

ROOT = Path(__file__).resolve().parents[1]
BASE = 'http://localhost:8080'


def command(args, env=None, check=True):
    result = subprocess.run(args, cwd=ROOT, env=env, capture_output=True, text=True, encoding='utf-8', errors='replace')
    if check and result.returncode:
        raise RuntimeError(f'Command failed (exit {result.returncode}): {args[0:3]}')
    return result


def runtime_environment():
    # The container's server environment is read in memory, never printed/saved.
    env = dict(os.environ)
    env.setdefault('SECRET_KEY', secrets.token_urlsafe(48))
    container = command(['docker', 'compose', 'ps', '-q', 'app'], env).stdout.strip()
    if not container:
        raise RuntimeError('Start the Compose service before verification')
    config = json.loads(command(['docker', 'inspect', container], env).stdout)[0]
    for item in config['Config']['Env']:
        key, _, value = item.partition('=')
        if key == 'SECRET_KEY' or key.startswith('SEED_') or key == 'MAX_UPLOAD_BYTES':
            env[key] = value
    return env


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


class Browser:
    def __init__(self):
        self.opener = build_opener(HTTPCookieProcessor(CookieJar()), NoRedirect())

    def request(self, path, data=None, headers=None, method=None):
        request = Request(BASE + path, data=data, headers=headers or {}, method=method)
        try:
            response = self.opener.open(request, timeout=10)
        except HTTPError as error:
            response = error
        with response:
            return response.status, response.read(), response.headers

    def token(self, page='/materials'):
        status, body, _ = self.request(page)
        assert status == 200
        return re.search(rb'name="csrf_token" value="([^"]+)"', body).group(1).decode()

    def login(self, username, password):
        token = self.token('/login')
        status, _, _ = self.request('/api/auth/login', json.dumps({'username': username, 'password': password}).encode(), {'Content-Type': 'application/json', 'X-CSRF-Token': token})
        assert status == 200, 'Login failed'

    def upload(self, content, filename='verification.txt'):
        boundary = 'boundary-' + secrets.token_hex(16)
        body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\nContent-Type: text/plain\r\n\r\n'.encode() + content + f'\r\n--{boundary}--\r\n'.encode())
        return self.request('/api/materials', body, {'Content-Type': f'multipart/form-data; boundary={boundary}', 'X-CSRF-Token': self.token()})


def database_snapshot(env):
    code = "import sqlite3,os,json,hashlib,pathlib; db=sqlite3.connect(os.environ['DATABASE']); tables=('users','materials','knowledge_entries'); counts={t:db.execute('SELECT count(*) FROM '+t).fetchone()[0] for t in tables}; dump='\\n'.join(db.iterdump()); files={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in pathlib.Path(os.environ['UPLOAD_ROOT']).rglob('*') if p.is_file()}; print(json.dumps({'counts':counts,'database_digest':hashlib.sha256(dump.encode()).hexdigest(),'files':files}))"
    return json.loads(command(['docker', 'compose', 'exec', '-T', 'app', 'python', '-c', code], env).stdout)


def verify_persistence(env):
    teacher = Browser()
    teacher.login('teacher_a', env['SEED_TEACHER_A_PASSWORD'])
    content = ('Compose persistence ' + secrets.token_hex(12) + '\n').encode()
    status, body, _ = teacher.upload(content)
    assert status == 201
    material_id = json.loads(body)['material_id']
    before = database_snapshot(env)
    command(['docker', 'compose', 'down'], env)
    command(['docker', 'compose', 'up', '--build', '-d', '--wait', '--wait-timeout', '90'], env)
    assert database_snapshot(env) == before
    for username, variable in [('teacher_a', 'SEED_TEACHER_A_PASSWORD'), ('student_a1', 'SEED_STUDENT_A1_PASSWORD')]:
        browser = Browser()
        browser.login(username, env[variable])
        assert material_id in {r['id'] for r in json.loads(browser.request('/api/materials')[1])['items']}
        status, detail, _ = browser.request(f'/api/materials/{material_id}')
        assert status == 200 and json.loads(detail)['plain_text'] == content.decode()
        status, downloaded, _ = browser.request(f'/api/materials/{material_id}/file')
        assert status == 200 and downloaded == content
    print('6.5 PASS: down/up --build retained accounts, database associations, parsed text and exact file bytes')


def verify_startup(env):
    command(['docker', 'compose', 'up', '--build', '-d', '--wait', '--wait-timeout', '90'], env)
    browser = Browser()
    status, body, _ = browser.request('/health')
    assert status == 200 and json.loads(body) == {'status': 'ok'}
    for username, variable in [('teacher_a', 'SEED_TEACHER_A_PASSWORD'), ('student_a1', 'SEED_STUDENT_A1_PASSWORD')]:
        browser.login(username, env[variable])
        assert browser.request('/materials')[0] == 200
    missing = dict(env)
    missing.pop('SECRET_KEY', None)
    assert command(['docker', 'compose', 'config', '--quiet'], missing, check=False).returncode != 0
    result = command(['docker', 'compose', 'run', '--rm', '--no-deps', '-e', 'SECRET_KEY=', 'app', 'python', 'scripts/serve.py'], env, check=False)
    assert result.returncode != 0 and 'Missing required configuration: SECRET_KEY' in result.stderr
    print('6.4 PASS: Compose startup, health, teacher/student login, missing-secret rejection')


def verify_scenarios(env):
    anonymous = Browser()
    status, _, headers = anonymous.request('/materials')
    assert status == 302 and headers['Location'] == '/login'
    for path, method in [('/api/materials', 'GET'), ('/api/materials', 'POST'), ('/api/materials/missing', 'GET'), ('/api/materials/missing/file', 'GET')]:
        assert anonymous.request(path, method=method)[0] == 401
    teacher, student, other = Browser(), Browser(), Browser()
    teacher.login('teacher_a', env['SEED_TEACHER_A_PASSWORD'])
    student.login('student_a1', env['SEED_STUDENT_A1_PASSWORD'])
    other.login('student_b1', env['SEED_STUDENT_B1_PASSWORD'])
    b_id = json.loads(other.request('/api/materials')[1])['items'][0]['id']
    for browser in (teacher, student):
        for suffix in ('', '/file'):
            forbidden = browser.request(f'/api/materials/{b_id}{suffix}')
            missing = browser.request(f'/api/materials/missing{suffix}')
            assert forbidden[:2] == missing[:2] and forbidden[0] == 404
    before = database_snapshot(env)
    assert student.upload(b'Student must not create this')[0] == 403
    assert database_snapshot(env) == before
    content = b'Integration acceptance material\n'
    status, body, _ = teacher.upload(content, 'integration.md')
    assert status == 201
    material_id = json.loads(body)['material_id']
    after = database_snapshot(env)
    for table in ('materials', 'knowledge_entries'):
        assert after['counts'][table] == before['counts'][table] + 1
    assert material_id in {r['id'] for r in json.loads(student.request('/api/materials')[1])['items']}
    assert student.request(f'/api/materials/{material_id}/file')[1] == content
    command(['docker', 'compose', 'down'], env)
    command(['docker', 'compose', 'up', '--build', '-d', '--wait', '--wait-timeout', '90'], env)
    assert database_snapshot(env) == after
    student.login('student_a1', env['SEED_STUDENT_A1_PASSWORD'])
    assert json.loads(student.request(f'/api/materials/{material_id}')[1])['plain_text'] == content.decode()
    assert student.request(f'/api/materials/{material_id}/file')[1] == content
    print('7.1 PASS: anonymous 302/401, cross-class 404, student 403/no writes, teacher 201/two rows, same-class visibility, Compose persistence')


def verify_security(env):
    secret_values = [env['SECRET_KEY']] + [v for k, v in env.items() if k.startswith('SEED_') and v]
    # Include untracked deliverables: this new repository may have no commit yet.
    files = command(['git', '-c', f'safe.directory={ROOT.as_posix()}', 'ls-files', '--cached', '--others', '--exclude-standard', '-z']).stdout.split('\x00')
    for name in files:
        path = ROOT / name
        if name and path.is_file():
            content = path.read_bytes()
            assert not any(value.encode() in content for value in secret_values), 'Credential found in repository file'
    image = command(['docker', 'compose', 'images', '-q', 'app'], env).stdout.strip()
    image_config = command(['docker', 'image', 'inspect', image], env).stdout
    logs = command(['docker', 'compose', 'logs', '--no-color', 'app'], env).stdout
    assert not any(value in image_config + logs for value in secret_values)
    assert 'pbkdf2:' not in logs and 'session=' not in logs
    browser = Browser()
    outputs = [browser.request('/login')[1], browser.request('/health')[1]]
    browser.login('teacher_a', env['SEED_TEACHER_A_PASSWORD'])
    for path in ('/materials', '/api/materials'):
        outputs.append(browser.request(path)[1])
    for item in json.loads(browser.request('/api/materials')[1])['items']:
        outputs.append(browser.request('/api/materials/' + item['id'])[1])
    before = database_snapshot(env)
    for content, filename, expected in [(b'\xff', 'invalid.txt', 400), (b'', 'empty.md', 400), (b'unsupported', 'file.pdf', 400), (b'x' * (int(env.get('MAX_UPLOAD_BYTES', 20971520)) + 1), 'large.txt', 413)]:
        status, body, _ = browser.upload(content, filename)
        assert status == expected
        outputs.append(body)
        assert database_snapshot(env) == before
    assert not any(value.encode() in body for value in secret_values for body in outputs)
    assert not any(b'pbkdf2:' in body for body in outputs)
    audit = '''import os,sqlite3
from werkzeug.security import check_password_hash
db=sqlite3.connect(os.environ['DATABASE'])
accounts=[('teacher_a','SEED_TEACHER_A_PASSWORD'),('student_a1','SEED_STUDENT_A1_PASSWORD'),('teacher_b','SEED_TEACHER_B_PASSWORD'),('student_b1','SEED_STUDENT_B1_PASSWORD')]
for username,variable in accounts:
    hashed=db.execute('SELECT password_hash FROM users WHERE username=?',(username,)).fetchone()[0]
    assert hashed.startswith('pbkdf2:sha256:')
    assert check_password_hash(hashed,os.environ[variable])
    assert not check_password_hash(hashed,'incorrect')
    assert os.environ[variable].encode() not in open(os.environ['DATABASE'],'rb').read()
print('Password storage audit passed')
'''
    command(['docker', 'compose', 'exec', '-T', 'app', 'python', '-c', audit], env)
    result = command(['docker', 'compose', 'run', '--rm', '--no-deps', '-v', f'{ROOT.as_posix()}/tests:/app/tests:ro', 'app', 'env', '-u', 'DATABASE', '-u', 'UPLOAD_ROOT', '-u', 'COOKIE_SECURE', 'python', '-m', 'pytest', '/app/tests', '-q', '-p', 'no:cacheprovider'], env, check=False)
    if result.returncode:
        failures = [line for line in result.stdout.splitlines() if line.startswith('FAILED ')]
        raise RuntimeError('Automated checks failed: ' + '; '.join(failures))
    print(result.stdout.strip())
    print('7.2 PASS: password hashes, no runtime secrets in repository/image/browser/API/logs, HTTP 400/413 cleanup, automated 5xx rollback tests')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('stage', choices=['startup', 'persistence', 'scenarios', 'security'])
    args = parser.parse_args()
    {'startup': verify_startup, 'persistence': verify_persistence, 'scenarios': verify_scenarios, 'security': verify_security}[args.stage](runtime_environment())
