"""Two real HTTP services, equivalent fixture; no in-process Flask test_client."""
from contextlib import closing
from pathlib import Path
from threading import Thread
import hashlib
import http.client
import json
import os
import sqlite3
import subprocess
import time
from urllib.parse import urlsplit

from werkzeug.security import generate_password_hash
from werkzeug.serving import make_server, WSGIRequestHandler


class QuietHandler(WSGIRequestHandler):
    def log(self, *args, **kwargs):
        pass  # Never log request URLs/credentials.


class Response:
    def __init__(self, status, headers, body):
        self.status, self.headers, self.body = status, dict((k.lower(), v) for k, v in headers), body

    def json(self):
        return json.loads(self.body)

    def summary(self):
        result = {'status': self.status, 'media_type': self.headers.get('content-type', '').split(';')[0]}
        if result['media_type'] == 'application/json':
            value = self.json()
            # Authentication responses are checked in memory, never serialized.
            if isinstance(value, dict) and 'access_token' in value:
                value = dict(value, access_token='[redacted]')
            result['json'] = value
        return result


class HTTPAdapter:
    def __init__(self, name, origin, secret, password, root, database):
        url = urlsplit(origin)
        if url.hostname != '127.0.0.1' or url.port in (None, 80, 443, 8080):
            raise RuntimeError('isolated random loopback origin required')
        self.name, self.origin, self.secret, self.password = name, origin, secret, password
        self.host, self.port, self.root, self.database = url.hostname, url.port, Path(root), database
        self.tokens = {}
        self.trace = []

    def request(self, method, path, token=None, body=b'', headers=None):
        assert path.startswith('/') and '://' not in path
        values = dict(headers or {})
        if token is not None:
            values['Authorization'] = 'Bearer ' + token
        conn = http.client.HTTPConnection(self.host, self.port, timeout=30)
        try:
            conn.request(method, path, body=body, headers=values)
            response = conn.getresponse()
            result = Response(response.status, response.getheaders(), response.read())
            observation = result.summary()
            observation['method'] = method
            observation['path'] = path
            observation['headers'] = {k: result.headers[k] for k in ('allow', 'cache-control', 'last-modified', 'www-authenticate', 'x-content-type-options') if k in result.headers}
            observation['etag_present'] = 'etag' in result.headers
            self.trace.append(observation)
            return result
        finally:
            conn.close()

    def login(self, username='teacher_a', extra=None):
        r = self.request('POST', '/api/auth/login', body=json.dumps(dict(username=username, password=self.password, **(extra or {}))).encode(), headers={'Content-Type': 'application/json'})
        if r.status != 200:
            raise AssertionError('fixture HTTP login failed')
        self.tokens[username] = r.json()['access_token']
        return self.tokens[username]

    def token(self, username='teacher_a'):
        return self.tokens.get(username) or self.login(username)

    def state(self):
        # Read-only verification of denied-request effects in this test fixture.
        with closing(self.database()) as db:
            cursor = db.cursor()
            tables = {}
            for table in ('materials', 'knowledge_entries'):
                cursor.execute('SELECT * FROM ' + table + ' ORDER BY id')
                tables[table] = hashlib.sha256(repr(cursor.fetchall()).encode()).hexdigest()
        files = {str(p.relative_to(self.root)): hashlib.sha256(p.read_bytes()).hexdigest()
                 for directory in ('tmp', 'files') for p in (self.root / directory).iterdir() if p.is_file()}
        return tables, files

    def version(self, user_id=1):
        with closing(self.database()) as db:
            cursor = db.cursor()
            marker = '?' if self.name == 'Flask' else '%s'
            cursor.execute('SELECT session_version FROM users WHERE id=' + marker, (user_id,))
            return cursor.fetchone()[0]


def start_flask(project, runtime, fixture, secret, password):
    from app import create_app
    database = runtime / 'flask.db'
    root = runtime / 'flask-uploads'
    for part in ('tmp', 'files'):
        (root / part).mkdir(parents=True, exist_ok=False)
    with closing(sqlite3.connect(database)) as db:
        db.executescript((project / 'app/schema.sql').read_text(encoding='utf-8'))
        db.executescript((project / 'app/session_schema.sql').read_text(encoding='utf-8'))
        hashed = generate_password_hash(password, method='pbkdf2:sha256:1000000')
        with db:
            db.executemany('INSERT INTO classes(id,name) VALUES (?,?)', [(v['id'], v['name']) for v in fixture['classes']])
            db.executemany('INSERT INTO users(id,username,password_hash,role,class_id,session_version) VALUES (?,?,?,?,?,0)', [(v['id'],v['username'],hashed,v['role'],v['class_id']) for v in fixture['users']])
            for index, v in enumerate(fixture['materials'], 1):
                raw = v['raw'].encode('utf-8')
                path = root / 'files' / v['id']
                path.write_bytes(raw)
                os.utime(path, (fixture['mtime'], fixture['mtime']))
                db.execute('INSERT INTO materials(id,class_id,uploader_id,original_name,size,content_type,storage_key,created_at) VALUES (?,?,?,?,?,?,?,?)', (v['id'],v['class_id'],v['uploader_id'],v['name'],len(raw),'text/plain',v['id'],fixture['created_at']))
                db.execute('INSERT INTO knowledge_entries(id,material_id,class_id,plain_text,created_at) VALUES (?,?,?,?,?)', (index,v['id'],v['class_id'],v['raw'].removeprefix('\ufeff'),fixture['created_at']))
    app = create_app({'MATERIAL_BACKEND':'sqlite', 'JWT_SIGNING_SECRET':secret, 'DATABASE':str(database), 'UPLOAD_ROOT':str(root), 'TESTING':False})
    app.logger.disabled = True
    server = make_server('127.0.0.1', 0, app, threaded=True, request_handler=QuietHandler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    adapter = HTTPAdapter('Flask', f'http://127.0.0.1:{server.server_port}', secret, password, root, lambda: sqlite3.connect(database))
    def reject_secret(value):
        # create_app validates before touching any database; never start a route.
        previous = os.environ.get('SECRET_KEY')
        os.environ['SECRET_KEY'] = 'synthetic-legacy-key-must-not-be-used'
        try:
            try:
                create_app({'MATERIAL_BACKEND':'sqlite','JWT_SIGNING_SECRET':value,
                            'DATABASE':str(database),'UPLOAD_ROOT':str(root)})
            except RuntimeError as error:
                return 'JWT_SIGNING_SECRET' in str(error)
            return False
        finally:
            if previous is None:
                os.environ.pop('SECRET_KEY', None)
            else:
                os.environ['SECRET_KEY'] = previous
    adapter.reject_secret = reject_secret
    def stop():
        server.shutdown(); thread.join(timeout=5); server.server_close()
    return adapter, stop


def start_go(executable, env, database):
    import socket
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    if port == 8080:
        raise RuntimeError('refuse Demo port')
    process_env = dict(env, LISTEN_ADDR=f'127.0.0.1:{port}', STARTUP_MODE='')
    process = subprocess.Popen([str(executable)], env=process_env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    adapter = HTTPAdapter('Go', f'http://127.0.0.1:{port}', env['JWT_SIGNING_SECRET'], env['CONTRACT_PASSWORD'], env['UPLOAD_ROOT'], database)
    def reject_secret(value):
        invalid = dict(env, LISTEN_ADDR='127.0.0.1:0', STARTUP_MODE='', SECRET_KEY='synthetic-legacy-key-must-not-be-used')
        if value is None:
            invalid.pop('JWT_SIGNING_SECRET', None)
        else:
            invalid['JWT_SIGNING_SECRET'] = value
        # config validation precedes recovery/database/storage operations.
        result = subprocess.run([str(executable)],env=invalid,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=5)
        return result.returncode != 0 and b'JWT_SIGNING_SECRET' in result.stderr
    adapter.reject_secret = reject_secret
    def stop():
        if process.poll() is None:
            process.terminate()
        process.wait(timeout=10)
    for _ in range(100):
        if process.poll() is not None:
            break
        try:
            if adapter.request('GET','/health').status == 200:
                return adapter, stop
        except OSError:
            pass
        time.sleep(.1)
    stop()
    raise RuntimeError('isolated Go startup failed')
