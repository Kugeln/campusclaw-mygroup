"""Remaining contracts against disposable MySQL/Qdrant, never classroom data."""
from contextlib import closing
from io import BytesIO
from itertools import count
from pathlib import Path
from unittest.mock import patch
from uuid import uuid4
import os
import secrets

import pytest
import pymysql
from werkzeug.security import generate_password_hash

from app import create_app
from app.db import connect
from app.materials import persist_material
from app.recovery import recover_storage
from app.retrieval.types import Scope, Unavailable
from app.storage.migration import migrate_materials, timestamp
from app.clients.qdrant import Qdrant
from scripts.init_db import initialize
from gateway import Gateway

pytestmark = pytest.mark.skipif(os.environ.get('RETRIEVAL_INTEGRATION') != '1', reason='Real isolated MySQL/Qdrant required')
classes = count(1000, 10)


@pytest.fixture
def setup(tmp_path):
    assert os.environ.get('RETRIEVAL_TEST_PROJECT', '').startswith('campusclaw-retrieval-check-')
    class_id = next(classes)
    database, uploads = tmp_path / 'identity.db', tmp_path / 'uploads'
    initialize(database)
    password = secrets.token_urlsafe(20)
    with closing(connect(database)) as db, db:
        db.executemany('INSERT INTO classes VALUES (?,?)', [(class_id, 'A'), (class_id+1, 'B')])
        hashed = generate_password_hash(password)
        db.executemany('INSERT INTO users(id,username,password_hash,role,class_id) VALUES (?,?,?,?,?)', [
            (class_id, 'teacher_a', hashed, 'teacher', class_id),
            (class_id+1, 'teacher_b', hashed, 'teacher', class_id+1),
            (class_id+2, 'student_a1', hashed, 'student', class_id)])
    with Gateway() as gateway:
        app = create_app(dict(TESTING=True, JWT_SIGNING_SECRET=secrets.token_urlsafe(48),
            DATABASE=str(database), UPLOAD_ROOT=str(uploads), MATERIAL_BACKEND='mysql',
            EMBEDDING_BASE_URL=gateway.url, EMBEDDING_API_KEY='test', EMBEDDING_MODEL='test',
            EMBEDDING_DIMENSION=3, EMBEDDING_API_FORMAT='ark_multimodal',
            DIALOGUE_BASE_URL=gateway.url, DIALOGUE_API_KEY='test-dialogue', DIALOGUE_MODEL='test'))
        storage = app.extensions['material_storage']
        storage.initialize()
        for folder in (uploads / 'files', uploads / 'tmp'):
            folder.mkdir(parents=True)
        client = app.test_client()
        response = client.post('/api/auth/login', json=dict(username='teacher_a', password=password))
        assert response.status_code == 200
        client.environ_base['HTTP_AUTHORIZATION'] = 'Bearer ' + response.json['access_token']
        yield app, client, storage, gateway, Scope(class_id, 'teacher', class_id)


def upload(client, text='星河活动周三下午三点在图书馆二楼举行。', name='small.md'):
    return client.post('/api/materials', data=dict(file=(BytesIO(text.encode()), name)))


def points(vectors, scope, material_id):
    return vectors.http.request('POST', vectors.collection + '/points/scroll', dict(
        filter=vectors.scope_filter(scope.class_id, material_id), limit=20, with_payload=True,
        with_vector=False))['result']['points']


def test_upload_embedding_failure_and_committed_original(setup):
    app, client, storage, gateway, scope = setup
    indexing = app.extensions['indexing']
    text = '星河阅读活动。' * 120  # 840 Unicode characters; two small chunks
    original = indexing.embedding.embed
    observed = []
    def committed(text):
        rows = storage.list_materials(scope.class_id)
        assert len(rows) == 1
        mid = rows[0]['id']
        assert client.get('/api/materials/' + mid).status_code == 200
        observed.append(mid)
        return original(text)
    gateway.fail_embedding_at = 2
    with patch.object(indexing.embedding, 'embed', side_effect=committed), patch.object(indexing.vectors, 'upsert', wraps=indexing.vectors.upsert) as write:
        result = upload(client, text)
        assert result.status_code == 201 and result.json['index_status'] == 'failed'
        assert len(observed) == 2 and write.call_count == 0
    mid = result.json['material_id']
    with storage.transaction() as cur:
        cur.execute('SELECT index_status FROM knowledge_chunks WHERE material_id=%s', (mid,))
        assert [row['index_status'] for row in cur.fetchall()] == ['failed', 'failed']
    assert client.get('/api/materials/' + mid + '/file').data == text.encode()
    assert storage.keyword(scope.class_id, '星河', 20) == []


def test_qdrant_contract_and_dimension_preservation(setup):
    app, client, storage, gateway, scope = setup
    vectors = app.extensions['indexing'].vectors
    result = upload(client)
    assert result.json['index_status'] == 'ready'
    mid = result.json['material_id']
    before = points(vectors, scope, mid)
    info = vectors.http.request('GET', vectors.collection)['result']
    assert info['config']['params']['vectors'] == dict(size=3, distance='Cosine')
    assert info['payload_schema']['class_id']['data_type'] == 'integer'
    with patch.object(vectors, 'dimension', 4):
        response = client.post('/api/materials/' + mid + '/reindex', json={})
        assert response.status_code == 503
    assert points(vectors, scope, mid) == before
    assert storage.find_material(mid, scope.class_id)['index_status'] == 'ready'
    with patch.object(vectors.http, 'request', wraps=vectors.http.request) as http:
        with pytest.raises(Unavailable):
            vectors.upsert(scope.class_id, [dict(chunk_id=str(uuid4()), material_id=mid, knowledge_entry_id=1, chunk_index=1)], [[1.,0.]])
        assert http.call_count == 0
    wrong = Qdrant(os.environ['QDRANT_URL'], 3)
    wrong.collection = '/collections/distance_check_' + uuid4().hex
    wrong.http.request('PUT', wrong.collection, dict(vectors=dict(size=3, distance='Dot')))
    with pytest.raises(Unavailable):
        wrong.ensure()
    assert wrong.http.request('GET', wrong.collection)['result']['config']['params']['vectors']['distance'] == 'Dot'
    vectors.delete(scope.class_id, mid)
    assert points(vectors, scope, mid) == []


@pytest.mark.parametrize('failure', ['partial', 'cleanup', 'ready', 'interrupted'])
def test_index_failure_recovery(setup, failure):
    app, client, storage, gateway, scope = setup
    indexing = app.extensions['indexing']
    vectors = indexing.vectors
    text = '星河活动说明。' * 120
    mid = upload(client, text).json['material_id']
    old = points(vectors, scope, mid)
    original_upsert, original_state = vectors.upsert, storage.state
    def partial(class_id, pieces, values):
        original_upsert(class_id, pieces[:1], values[:1])
        raise Unavailable('Simulated partial write')
    def ready_failure(class_id, material_id, state, cleanup=False):
        if state == 'ready':
            raise Unavailable('Simulated ready transaction failure')
        return original_state(class_id, material_id, state, cleanup)
    if failure == 'interrupted':
        storage.state(scope.class_id, mid, 'indexing')
    elif failure == 'ready':
        with patch.object(storage, 'state', side_effect=ready_failure):
            assert client.post('/api/materials/' + mid + '/reindex', json={}).status_code == 503
    elif failure == 'partial':
        with patch.object(vectors, 'upsert', side_effect=partial):
            assert client.post('/api/materials/' + mid + '/reindex', json={}).status_code == 503
    else:
        with patch.object(vectors, 'delete', side_effect=Unavailable()), patch.object(storage, 'replace_chunks', wraps=storage.replace_chunks) as replace_chunks:
            assert client.post('/api/materials/' + mid + '/reindex', json={}).status_code == 503
            assert replace_chunks.call_count == 0
            assert storage.find_material(mid, scope.class_id)['cleanup_required'] == 1
        assert points(vectors, scope, mid) == old
    assert storage.keyword(scope.class_id, '星河', 20) == []
    assert client.get('/api/materials/' + mid + '/file').data == text.encode()
    indexing.recover()
    assert points(vectors, scope, mid) == []
    assert client.post('/api/materials/' + mid + '/reindex', json={'strategy':'markdown'}).status_code == 200
    assert storage.find_material(mid, scope.class_id)['index_status'] == 'ready'
    assert all(p['id'] not in {p['id'] for p in old} for p in points(vectors, scope, mid))


def test_keyword_states_ordering_and_unavailable_vectors(setup):
    app, client, storage, gateway, scope = setup
    index = app.extensions['indexing']
    mids = [upload(client, text).json['material_id'] for text in
            ('星河星河星河阅读', '星河其他活动说明', '星河待处理', '星河失败')]
    storage.state(scope.class_id, mids[2], 'pending')
    storage.state(scope.class_id, mids[3], 'failed')
    with patch.object(index.vectors.http, 'request', side_effect=Unavailable()), patch.object(index.embedding, 'embed', side_effect=Unavailable()) as embed:
        response = client.get('/api/search?q=星河&mode=keyword')
        assert response.status_code == 200
        items = response.json['items']
        assert {i['material_id'] for i in items} == set(mids[:2])
        assert items == sorted(items, key=lambda item:(-item['score'], item['chunk_id']))
        assert embed.call_count == 0
    # Unavailable Qdrant specifically, with the embedding gateway still working.
    with patch.object(index.vectors.http, 'request', side_effect=Unavailable()), patch.object(app.extensions['answering'].dialogue, 'answer') as dialogue:
        for mode in ('vector', 'hybrid'):
            assert client.post('/api/ask', json=dict(question='星河', mode=mode)).status_code == 503
        assert dialogue.call_count == 0


def test_mysql_upload_transactions_and_authoritative_recovery(setup):
    app, client, storage, gateway, scope = setup
    root = Path(app.config['UPLOAD_ROOT'])
    def snapshot():
        return storage.list_materials(scope.class_id), {p.name:p.read_bytes() for p in (root/'files').iterdir()}
    before = snapshot()
    with patch('pathlib.Path.replace', side_effect=OSError()):
        assert upload(client).status_code == 500
    assert snapshot() == before
    real_connect = pymysql.connect
    def fail_commit(**options):
        connection = real_connect(**options)
        connection.commit = lambda: (_ for _ in ()).throw(pymysql.OperationalError('commit failure'))
        return connection
    with patch('app.storage.mysql.pymysql.connect', side_effect=fail_commit):
        assert upload(client).status_code == 500
    assert snapshot() == before
    mid = upload(client).json['material_id']
    with closing(connect(app.config['DATABASE'])) as db:
        assert db.execute('SELECT COUNT(*) FROM materials').fetchone()[0] == 0
        # Recovery sees all records in this disposable MySQL database. Use a repository
        # view of this test's class because other tests use their own upload roots.
        rows = [r for r in storage.recovery_rows() if r['storage_key'] == mid]
        (root/'tmp'/'interrupted').write_text('temporary')
        with patch.object(storage, 'recovery_rows', return_value=rows):
            recover_storage(db, root, storage)
    assert (root/'files'/mid).is_file() and not list((root/'tmp').iterdir())


def test_explicit_migration_and_mysql_seed(setup, tmp_path):
    app, client, storage, gateway, scope = setup
    source, files = tmp_path/'source.db', tmp_path/'legacy-uploads'
    initialize(source)
    with closing(connect(source)) as db, db:
        db.execute('INSERT INTO classes VALUES (?,?)', (scope.class_id, 'Source'))
        db.execute('INSERT INTO users(id,username,password_hash,role,class_id) VALUES (?,?,?,?,?)',
                   (scope.user_id,'source-teacher','unchanged-hash','teacher',scope.class_id))
        mid = persist_material(db, files, 'legacy.md', '星河旧材料完整正文'.encode(), scope.class_id, scope.user_id)
        # Avoid entry IDs used by other tests; all source IDs must be preserved.
        db.execute('UPDATE knowledge_entries SET id=?', (scope.class_id * 100,))
        before = list(db.iterdump())
    file_before = (files/'files'/mid).read_bytes()
    result = migrate_materials(source, files, storage)
    assert result == dict(materials=1,inserted=1,unchanged=0)
    assert migrate_materials(source, files, storage) == dict(materials=1,inserted=0,unchanged=1)
    row = storage.find_material(mid, scope.class_id)
    assert row['knowledge_entry_id'] == scope.class_id * 100
    assert row['plain_text'] == '星河旧材料完整正文'
    with closing(connect(source)) as db:
        assert list(db.iterdump()) == before
        original = db.execute('SELECT * FROM materials WHERE id=?',(mid,)).fetchone()
        assert row['created_at'] == timestamp(original['created_at'])
        assert row['uploader_id'] == original['uploader_id'] and row['storage_key'] == original['storage_key']
    assert (files/'files'/mid).read_bytes() == file_before
    with closing(connect(source)) as db, db:
        db.execute('UPDATE materials SET original_name=?', ('conflict.md',))
    with pytest.raises(ValueError, match='Conflicting'):
        migrate_materials(source, files, storage)
    assert storage.find_material(mid, scope.class_id)['original_name'] == 'legacy.md'
    with closing(connect(app.config['DATABASE'])) as db:
        identity_before = list(db.iterdump())
        storage.seed_materials(db, app.config['UPLOAD_ROOT'])
        count_before = len(storage.list_materials(scope.class_id))
        storage.seed_materials(db, app.config['UPLOAD_ROOT'])
        assert len(storage.list_materials(scope.class_id)) == count_before
        assert list(db.iterdump()) == identity_before
    assert client.get('/api/auth/me').status_code == 200  # token remains valid
