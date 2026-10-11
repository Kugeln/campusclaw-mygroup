from app.accounts import create_class, create_user
from app.db import connect
from conftest import sign_in
import pytest
from test_uploads import upload


def test_class_lists(app, client):
    for username, expected, excluded in [('student_a1', 'Class-A', 'Class-B'), ('student_b1', 'Class-B', 'Class-A')]:
        sign_in(client, app, username)
        response = client.get('/api/materials')
        assert response.status_code == 200
        assert len(response.json['items']) == 1
        assert expected in response.get_data(as_text=True)
        assert excluded not in response.get_data(as_text=True)
        assert client.get('/api/materials?class_id=999').status_code == 400
        assert client.get('/api/materials?role=teacher').status_code == 400
    with connect(app.config['DATABASE']) as db:
        class_id = create_class(db, 'Empty')
        create_user(db, 'empty', app.test_password, 'student', class_id)
    sign_in(client, app, 'empty')
    response = client.get('/api/materials')
    assert response.status_code == 200 and response.json == {'items': []}


def test_scoped_details_and_files(app, client):
    sign_in(client, app, 'student_b1')
    b = client.get('/api/materials').json['items'][0]['id']
    for username in ('teacher_a', 'student_a1'):
        sign_in(client, app, username)
        a = client.get('/api/materials').json['items'][0]['id']
        for suffix in ('', '/file'):
            denied = client.get(f'/api/materials/{b}{suffix}')
            missing = client.get(f'/api/materials/absent{suffix}')
            assert denied.status_code == missing.status_code == 404
            assert denied.data == missing.data
            assert 'Class-B' not in denied.get_data(as_text=True)
            assert b not in denied.get_data(as_text=True)
            assert client.get(f'/api/materials/{a}{suffix}').status_code == 200
        detail = client.get(f'/api/materials/{a}').json
        assert 'Class A' in detail['plain_text']
        assert 'storage_key' not in detail and 'path' not in detail


def test_private_attachment(app, client):
    from app.materials import persist_material
    content = b'<script>alert(1)</script>\n# Markdown remains text'
    with connect(app.config['DATABASE']) as db:
        user = db.execute("SELECT id,class_id FROM users WHERE username='teacher_a'").fetchone()
        material_id = persist_material(db, app.config['UPLOAD_ROOT'], 'script.md', content, user['class_id'], user['id'])
    sign_in(client, app)
    for path in (f'/uploads/files/{material_id}', f'/static/uploads/files/{material_id}', f'/files/{material_id}'):
        assert client.get(path).status_code == 404
    response = client.get(f'/api/materials/{material_id}/file')
    assert response.data == content
    assert response.headers['Content-Disposition'].startswith('attachment;')
    assert response.mimetype == 'text/plain'
    assert response.headers['X-Content-Type-Options'] == 'nosniff'


@pytest.mark.parametrize('group', ['a', 'b'])
def test_both_roles_read_new_class_upload(app, client, group):
    teacher, student = f'teacher_{group}', f'student_{group}1'
    assert sign_in(client, app, teacher).status_code == 200
    content = f'New material for {group}'.encode()
    response = upload(client, content)
    assert response.status_code == 201
    material_id = response.json['material_id']
    with connect(app.config['DATABASE']) as db:
        user = db.execute('SELECT id,class_id FROM users WHERE username=?', (teacher,)).fetchone()
        row = db.execute('SELECT uploader_id,class_id FROM materials WHERE id=?', (material_id,)).fetchone()
        assert tuple(row) == tuple(user)
    for username in (teacher, student):
        assert sign_in(client, app, username).status_code == 200
        page = client.get('/materials')
        assert page.status_code == 200 and material_id.encode() not in page.data
        listing = client.get('/api/materials')
        assert listing.status_code == 200 and material_id in {r['id'] for r in listing.json['items']}
        detail = client.get(f'/api/materials/{material_id}')
        assert detail.status_code == 200 and detail.json['plain_text'] == content.decode()
        with client.get(f'/api/materials/{material_id}/file') as download:
            assert download.status_code == 200 and download.data == content


@pytest.mark.parametrize('group', ['a', 'b'])
def test_bidirectional_class_isolation(app, client, group):
    other = 'b' if group == 'a' else 'a'
    with connect(app.config['DATABASE']) as db:
        other_class = db.execute('SELECT class_id FROM users WHERE username=?', (f'teacher_{other}',)).fetchone()[0]
        foreign_ids = {r[0] for r in db.execute('SELECT id FROM materials WHERE class_id=?', (other_class,))}
        own_ids = {r[0] for r in db.execute('SELECT id FROM materials WHERE class_id<>?', (other_class,))}
    for username in (f'teacher_{group}', f'student_{group}1'):
        assert sign_in(client, app, username).status_code == 200
        assert {r['id'] for r in client.get('/api/materials').json['items']} == own_ids
        for field in ('class_id', 'role', 'user_id'):
            assert client.get('/api/materials', query_string={field: other_class}).status_code == 400
        for material_id in foreign_ids:
            for suffix in ('', '/file'):
                missing = client.get(f'/api/materials/absent{suffix}')
                for query in ({}, {'class_id': other_class, 'role': 'teacher', 'user_id': 999}):
                    denied = client.get(f'/api/materials/{material_id}{suffix}', query_string=query)
                    assert denied.status_code == missing.status_code == 404
                    assert denied.data == missing.data
                    assert material_id not in denied.get_data(as_text=True)
