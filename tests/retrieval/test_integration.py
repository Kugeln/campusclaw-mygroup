"""Small real MySQL/Qdrant chain, explicitly enabled by the integration runner."""
from contextlib import closing
from io import BytesIO
from pathlib import Path
import os
import secrets
from unittest.mock import patch

import pytest

from app import create_app
from app.db import connect
from app.accounts import seed_accounts
from app.recovery import recover_storage
from app.retrieval.types import Unavailable
from scripts.init_db import initialize
from gateway import Gateway


pytestmark = pytest.mark.skipif(os.environ.get('RETRIEVAL_INTEGRATION') != '1', reason='Real MySQL/Qdrant required')


def test_small_real_chain(tmp_path):
    password = secrets.token_urlsafe(24)
    database, uploads = tmp_path / 'identity.db', tmp_path / 'uploads'
    initialize(database)
    with closing(connect(database)) as db:
        with db:
            seed_accounts(db, lambda _: password)
    with Gateway() as gateway:
        app = create_app(dict(TESTING=True, JWT_SIGNING_SECRET=secrets.token_urlsafe(48),
            DATABASE=str(database), UPLOAD_ROOT=str(uploads), MATERIAL_BACKEND='mysql',
            EMBEDDING_BASE_URL=gateway.url, EMBEDDING_API_KEY='test', EMBEDDING_MODEL='test', EMBEDDING_DIMENSION=3,
            DIALOGUE_BASE_URL=gateway.url, DIALOGUE_API_KEY='test-dialogue', DIALOGUE_MODEL='test'))
        storage = app.extensions['material_storage']
        storage.initialize()
        storage.initialize()
        with closing(connect(database)) as db:
            recover_storage(db, uploads, storage)
        client = app.test_client()
        tokens = {}
        for name in ('teacher_a', 'student_a1', 'teacher_b', 'student_b1'):
            response = client.post('/api/auth/login', json=dict(username=name, password=password))
            assert response.status_code == 200
            tokens[name] = dict(Authorization='Bearer ' + response.json['access_token'])
        assert client.get('/api/search?q=x').status_code == 401
        assert client.post('/api/ask', json=dict(question='x')).status_code == 401
        for url in ('/materials', '/knowledge'):
            assert client.get(url).status_code == 200
        before = len(gateway.calls)
        for query in ('', '   '):
            assert client.get('/api/search', query_string=dict(q=query), headers=tokens['teacher_a']).status_code == 400
        assert client.post('/api/ask', json=dict(question='x', mode='invalid'), headers=tokens['teacher_a']).status_code == 400
        assert len(gateway.calls) == before
        # Two documents, about 1 KB each, one chunk each. No large corpus.
        a_text = ('# 星河读书活动\n星河活动周三下午三点在图书馆二楼举行。联系人是林老师。'
                  '请携带纸质笔记本。活动包括安静阅读、分组交流和分享读书笔记。'
                  '报名截止周二中午，参与者在班级材料页面阅读通知。' +
                  '每位同学选择一段自己喜欢的文字，说明选择理由，并认真倾听同学发言。' * 9)
        b_text = ('# 海风实践活动\n海风活动周五上午九点在实验楼三楼举行。联系人是陈老师。'
                  '请携带实验记录本。参与者先阅读安全提示，再按照老师安排完成分组观察。' +
                  '记录观察时间和实验条件，保持桌面整洁，交流时先描述事实再提出问题。' * 10)
        assert 1000 <= len(a_text.encode()) <= 3000
        assert 1000 <= len(b_text.encode()) <= 3000
        material_ids = {}
        indexing = app.extensions['indexing']
        original_embed = indexing.embedding.embed
        # Assert relational original is already readable before the first model call.
        def committed_embed(text):
            assert storage.list_materials(1)
            return original_embed(text)
        with patch.object(indexing.embedding, 'embed', side_effect=committed_embed):
            for name, text, filename in [('teacher_a', a_text, 'demo-a.md'), ('teacher_b', b_text, 'demo-b.txt')]:
                response = client.post('/api/materials', data=dict(file=(BytesIO(text.encode()), filename)), headers=tokens[name])
                assert response.status_code == 201, response.json
                assert response.json['index_status'] == 'ready'
                material_ids[name] = response.json['material_id']
        a_id, b_id = material_ids.values()
        assert len(storage.keyword(1, '星河', 20)) == 1
        assert storage.keyword(1, '海风', 20) == []
        with storage.transaction() as cur:
            cur.execute('SELECT @@ngram_token_size AS size')
            assert cur.fetchone()['size'] == 2
        # Composite constraints reject a foreign-class entry.
        with pytest.raises(Unavailable):
            with storage.transaction() as cur:
                cur.execute('UPDATE knowledge_entries SET class_id=2 WHERE material_id=%s', (a_id,))
        vectors = indexing.vectors
        vectors.ensure()
        scroll = vectors.http.request('POST', vectors.collection + '/points/scroll', dict(limit=10, with_payload=True, with_vector=False))
        for point in scroll['result']['points']:
            assert set(point['payload']) == {'class_id','material_id','knowledge_entry_id','chunk_id','chunk_index'}
            assert point['id'] == point['payload']['chunk_id']
            assert 'vector' not in point or point['vector'] is None
        # Full small A/B and role matrix, using only the two tiny documents.
        for name, headers in tokens.items():
            own, other, foreign_id = ('星河', '海风', b_id) if name.endswith(('a', 'a1')) else ('海风', '星河', a_id)
            class_id = 2 if own == '星河' else 1
            forged = dict(headers, **{'X-Class-Id':str(class_id), 'X-Role':'teacher'})
            client.set_cookie('class_id', str(class_id))
            client.set_cookie('user_id', '999')
            client.set_cookie('role', 'teacher')
            for mode in ('keyword', 'vector', 'hybrid'):
                response = client.get('/api/search', query_string=dict(q=own, mode=mode, class_id=class_id, user_id=999, role='teacher'), headers=forged)
                assert response.status_code == 200
                assert response.json['items'] and all(other not in item['chunk_text'] for item in response.json['items'])
                before = sum(path.endswith('/chat/completions') for path, _ in gateway.calls)
                response = client.post('/api/ask', json=dict(question=other, mode=mode, class_id=class_id, user_id=999, role='teacher'), headers=forged)
                assert response.status_code == 200
                assert response.json == dict(answer='资料中未找到相关内容', sources=[])
                assert sum(path.endswith('/chat/completions') for path, _ in gateway.calls) == before
            assert client.get('/api/materials/' + foreign_id, headers=headers).status_code == 404
            assert client.get('/api/materials/' + foreign_id + '/file', headers=headers).status_code == 404
            if name.startswith('student'):
                assert client.post('/api/materials', headers=forged).status_code == 403
                assert client.post('/api/materials/' + a_id + '/reindex', json={}, headers=forged).status_code == 403
        response = client.post('/api/ask', json=dict(question='星河活动何时举行？'), headers=tokens['teacher_a'])
        assert response.status_code == 200, response.json
        assert '[1]' in response.json['answer']
        assert response.json['sources'][0]['title'] == 'demo-a.md'
        assert response.json['sources'][0]['chunk_text'] == a_text
        dialogue_input = [data for path, data in gateway.calls if path.endswith('/chat/completions')][-1]
        assert '海风' not in str(dialogue_input)
        for answer in ('', '没有引用', '错误[9]'):
            gateway.dialogue_value = answer
            assert client.post('/api/ask', json=dict(question='星河'), headers=tokens['teacher_a']).status_code == 502
        gateway.dialogue_value = None
        # Same-class hydration discards a malicious hit even if its payload claims A.
        foreign = storage.keyword(2, '海风', 1)[0]
        with patch.object(vectors, 'search', return_value=[dict(id=foreign.chunk_id, score=.99,
                payload=dict(class_id=1, material_id=foreign.material_id, knowledge_entry_id=foreign.knowledge_entry_id,
                             chunk_id=foreign.chunk_id, chunk_index=foreign.chunk_index))]):
            before = len(gateway.calls)
            response = client.post('/api/ask', json=dict(question='星河', mode='vector'), headers=tokens['teacher_a'])
            assert response.json == dict(answer='资料中未找到相关内容', sources=[])
            assert len(gateway.calls) == before + 1  # query embedding only
        # Keyword neither embeds nor visits vector service; no failure downgrade.
        with patch.object(vectors, 'search', side_effect=Unavailable()), patch.object(indexing.embedding, 'embed', side_effect=Unavailable()):
            assert client.get('/api/search?q=星河&mode=keyword', headers=tokens['teacher_a']).status_code == 200
            for mode in ('vector', 'hybrid'):
                assert client.post('/api/ask', json=dict(question='星河', mode=mode), headers=tokens['teacher_a']).status_code == 503
        # Rebuild keeps old data on invalid strategy and denies foreign scope.
        for target in (b_id, 'missing'):
            assert client.post('/api/materials/' + target + '/reindex', json={}, headers=tokens['teacher_a']).status_code == 404
        original_ids = [e.chunk_id for e in storage.keyword(1, '星河', 20)]
        assert client.post('/api/materials/' + a_id + '/reindex', json=dict(size=0), headers=tokens['teacher_a']).status_code == 400
        with indexing.lock(a_id):
            assert client.post('/api/materials/' + a_id + '/reindex', json={}, headers=tokens['teacher_a']).status_code == 409
        # Fail the second embedding of a small two-chunk rebuild; no vector upsert.
        gateway.fail_embedding_at = sum('/embeddings' in path for path, _ in gateway.calls) + 2
        with patch.object(vectors, 'upsert', wraps=vectors.upsert) as upsert:
            response = client.post('/api/materials/' + a_id + '/reindex', json=dict(size=300, overlap=30), headers=tokens['teacher_a'])
            assert response.status_code == 503
            assert upsert.call_count == 0
        assert storage.keyword(1, '星河', 20) == []
        with storage.transaction() as cur:
            cur.execute('SELECT DISTINCT index_status FROM knowledge_chunks WHERE material_id=%s', (a_id,))
            assert cur.fetchall() == [dict(index_status='failed')]
        assert client.get('/api/materials/' + a_id + '/file', headers=tokens['teacher_a']).data == a_text.encode()
        gateway.fail_embedding_at = None
        response = client.post('/api/materials/' + a_id + '/reindex', json=dict(strategy='markdown'), headers=tokens['teacher_a'])
        assert response.status_code == 200, response.json
        assert all(e.chunk_id not in original_ids for e in storage.keyword(1, '星河', 20))
        # Recovery must use MySQL, despite an empty SQLite material snapshot.
        with closing(connect(database)) as db:
            recover_storage(db, uploads, storage)
        assert (uploads / 'files' / a_id).read_text(encoding='utf-8') == a_text
        assert client.get('/api/materials/' + a_id, headers=tokens['teacher_a']).json['plain_text'] == a_text
