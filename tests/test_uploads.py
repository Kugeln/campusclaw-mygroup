from io import BytesIO
from pathlib import Path
from contextlib import closing
import sqlite3
import pytest
from werkzeug.datastructures import FileStorage
from app.materials import prepare_upload

from app.db import connect
from conftest import auth_headers, sign_in


def snapshot(app):
    # Preserve every business table, index and trigger. Only session lifecycle
    # writes are excluded; they have separate assertions below.
    with closing(connect(app.config['DATABASE'])) as db, closing(sqlite3.connect(':memory:')) as copy:
        db.backup(copy)
        copy.execute('DROP TABLE sessions')
        records = list(copy.iterdump())
    files = {str(p): p.read_bytes() for p in Path(app.config['UPLOAD_ROOT']).rglob('*') if p.is_file()}
    return records, files


def session_snapshot(app):
    with closing(connect(app.config['DATABASE'])) as db:
        return [tuple(row) for row in db.execute('SELECT * FROM sessions ORDER BY sid_hash')]


def upload(client, content=b'New teaching material', name='new.txt', **fields):
    return client.post('/api/materials', data={'file': (BytesIO(content), name), **fields}, headers=auth_headers(client))


def test_upload_identity(app, client):
    before = snapshot(app)
    sign_in(client, app, 'student_a1')
    sessions = session_snapshot(app)
    assert upload(client).status_code == 403
    assert upload(client, role='teacher').status_code == 403
    assert snapshot(app) == before
    assert session_snapshot(app) == sessions
    sign_in(client, app)
    for field in ('class_id', 'uploader_id', 'role'):
        assert upload(client, **{field: '999'}).status_code == 400
        assert snapshot(app) == before


def test_size_collision_and_traversal(app, client):
    from uuid import UUID
    import pytest
    from werkzeug.exceptions import RequestEntityTooLarge
    assert app.config['MAX_UPLOAD_BYTES'] == 20 * 1024 * 1024
    sign_in(client, app)
    before = snapshot(app)
    app.config['MAX_UPLOAD_BYTES'] = 8
    assert upload(client, b'x' * 9).status_code == 413
    assert snapshot(app) == before
    # FileStorage has no declared length: the streamed bytes enforce the limit.
    with pytest.raises(RequestEntityTooLarge):
        with prepare_upload(app.config['UPLOAD_ROOT'], FileStorage(stream=BytesIO(b'x' * 9), filename='large.txt'), 8):
            pass
    assert snapshot(app) == before
    ids = []
    for name, content in [('same.txt', b'one'), ('same.txt', b'two'), ('../../outside.txt', b'three'), ('..\\..\\outside.txt', b'four')]:
        response = upload(client, content, name)
        assert response.status_code == 201
        material_id = response.json['material_id']
        UUID(material_id)
        ids.append(material_id)
        assert client.get(f'/api/materials/{material_id}/file').data == content
        detail = client.get(f'/api/materials/{material_id}').json
        assert '/' not in detail['original_name'] and '\\' not in detail['original_name']
    assert len(set(ids)) == 4
    assert not (Path(app.config['UPLOAD_ROOT']).parent / 'outside.txt').exists()


def test_successful_upload_is_committed(app, client):
    sign_in(client, app)
    with connect(app.config['DATABASE']) as db:
        counts = [db.execute(f'SELECT count(*) FROM {table}').fetchone()[0] for table in ('materials', 'knowledge_entries')]
    content = b'\xef\xbb\xbf# Lesson\n<script>text only</script>'
    response = upload(client, content, 'lesson.md')
    assert response.status_code == 201
    material_id = response.json['material_id']
    # A separate connection observes both rows only after commit.
    with connect(app.config['DATABASE']) as db:
        assert [db.execute(f'SELECT count(*) FROM {table}').fetchone()[0] for table in ('materials', 'knowledge_entries')] == [n + 1 for n in counts]
        row = db.execute('SELECT m.*,k.plain_text,k.class_id AS text_class FROM materials m JOIN knowledge_entries k ON k.material_id=m.id WHERE m.id=?', (material_id,)).fetchone()
        user = db.execute("SELECT id,class_id FROM users WHERE username='teacher_a'").fetchone()
        assert row['uploader_id'] == user['id']
        assert row['class_id'] == row['text_class'] == user['class_id']
        assert row['plain_text'] == content.decode('utf-8-sig')
        assert (Path(app.config['UPLOAD_ROOT']) / 'files' / row['storage_key']).read_bytes() == content
    assert material_id in {row['id'] for row in client.get('/api/materials').json['items']}


def test_upload_validation(app, client):
    sign_in(client, app)
    before = snapshot(app)
    assert client.post('/api/materials', headers=auth_headers(client)).status_code == 400
    for name, content in [('empty.txt', b''), ('bad.pdf', b'text'), ('bad.txt', b'\xff'), ('nul.md', b'hello\x00world')]:
        assert upload(client, content, name).status_code == 400
        assert snapshot(app) == before


def test_teacher_student_page_and_read_only(app, client):
    sign_in(client, app)
    assert b'id="upload"' in client.get('/materials').data
    response = upload(client, b'<script>alert(1)</script>', '<script>alert(1)</script>.txt')
    assert response.status_code == 201
    material_id = response.json['material_id']
    assert material_id.encode() not in client.get('/materials').data
    sign_in(client, app, 'student_a1')
    page = client.get('/materials')
    assert b'id="upload-card" hidden' in page.data
    assert material_id.encode() not in page.data
    assert b'<script>alert(1)</script>' not in page.data
    assert b'.textContent = material.plain_text' in client.get('/static/materials.js').data
    before = snapshot(app)
    assert upload(client).status_code == 403
    for method in ('put', 'patch', 'delete', 'post'):
        assert getattr(client, method)(f'/api/materials/{material_id}', data={'plain_text': 'overwrite'}, headers=auth_headers(client)).status_code == 405
    assert snapshot(app) == before
    # Legacy metadata is escaped by the template too.
    with connect(app.config['DATABASE']) as db:
        db.execute('UPDATE materials SET original_name=? WHERE id=?', ('<script>filename</script>.txt', material_id))
    assert b'<script>filename</script>' not in client.get('/materials').data
    assert b'cell.textContent = value' in client.get('/static/materials.js').data
    items = client.get('/api/materials').json['items']
    assert items == sorted(items, key=lambda r: (r['created_at'], r['id']), reverse=True)


def test_valid_text_parsing(app):
    before = snapshot(app)
    for name, content, expected in [('a.TXT', b'\xef\xbb\xbfhello', 'hello'), ('b.Md', b'# heading\n<script>x</script>', '# heading\n<script>x</script>')]:
        file = FileStorage(stream=BytesIO(content), filename=name)
        with prepare_upload(app.config['UPLOAD_ROOT'], file) as (path, text):
            assert text == expected
            assert path.read_bytes() == content
        assert snapshot(app) == before


@pytest.mark.parametrize('group', ['a', 'b'])
def test_forged_identity_cannot_grant_upload(app, client, group):
    username = f'student_{group}1'
    with connect(app.config['DATABASE']) as db:
        teacher = db.execute('SELECT id,class_id FROM users WHERE username=?', (f'teacher_{"b" if group == "a" else "a"}',)).fetchone()
        student = db.execute('SELECT id,class_id FROM users WHERE username=?', (username,)).fetchone()
    forged = {'role': 'teacher', 'class_id': str(teacher['class_id']), 'user_id': str(teacher['id'])}
    client.get('/login')
    response = client.post('/api/auth/login', json={'username': username, 'password': app.test_password, **forged}, headers=auth_headers(client))
    assert response.status_code == 200
    client.environ_base['HTTP_AUTHORIZATION'] = 'Bearer ' + response.json['access_token']
    principal = client.get('/api/auth/me').json
    assert (principal['user_id'], principal['class_id'], principal['role']) == (*student, 'student')
    # Self-declared identity cookies/headers are not session identity.
    for name, value in forged.items():
        client.set_cookie(name, value)
    before, sessions = snapshot(app), session_snapshot(app)
    responses = [
        upload(client), upload(client, **forged),
        client.post('/api/materials', json=forged, headers=auth_headers(client)),
        client.post('/api/materials', query_string=forged, data={'file': (BytesIO(b'valid'), 'a.txt')}, headers=auth_headers(client)),
        client.post('/api/materials', data={'file': (BytesIO(b'valid'), 'a.txt')}, headers={**auth_headers(client), 'X-Role': 'teacher', 'X-User-Id': forged['user_id'], 'X-Class-Id': forged['class_id']}),
        client.post('/api/materials', data={'file': (BytesIO(b'valid'), 'a.txt')}),
    ]
    for denied in responses:
        assert denied.status_code == 403
        assert denied.json == {'error': 'Teacher role required'}
    assert snapshot(app) == before and session_snapshot(app) == sessions
    sign_in(client, app, f'teacher_{group}')
    before = snapshot(app)
    for field in ('user_id', 'role', 'class_id', 'uploader_id'):
        assert upload(client, **{field: '999'}).status_code == 400
        assert snapshot(app) == before
