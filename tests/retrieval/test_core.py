from dataclasses import replace
from pathlib import Path
import ast
import math

import pytest

from app.retrieval.chunking import chunks, validate_strategy
from app.retrieval.ranking import rrf
from app.retrieval.service import Retrieval
from app.retrieval.answering import Answering, NO_EVIDENCE
from app.retrieval.types import Evidence, Scope, InvalidInput, InvalidAnswer, Unavailable


def test_chunking():
    text = ''.join(chr(0x4e00 + i) for i in range(1601))
    result = chunks(text)
    assert list(map(len, result)) == [800, 800, 161]
    assert result[0][-80:] == result[1][:80]
    assert result[0] + result[1][80:] + result[2][80:] == text
    assert chunks(text) == result
    assert chunks('a.*b.*尾', dict(strategy='delimiter', delimiter='.*')) == ['a.*', 'b.*', '尾']
    markdown = '# 一\n正文\n```md\n# 围栏内\n```\n## 二\n尾'
    assert chunks(markdown, dict(strategy='markdown')) == ['# 一\n正文\n```md\n# 围栏内\n```\n', '## 二\n尾']
    assert chunks(' \n\t') == []
    assert chunks('😀' * 9, dict(size=8, overlap=0)) == ['😀' * 8, '😀']
    for opts in [dict(strategy='bad'), dict(size=True), dict(size=0), dict(size=8, overlap=8),
                 dict(overlap=-1), dict(strategy='delimiter', delimiter=''), []]:
        with pytest.raises(InvalidInput):
            validate_strategy(opts)


def test_rrf():
    scores = dict(rrf(['a', 'a', 'b'], ['b', 'c', 'a']))
    assert scores['a'] == 1 / 61 + 1 / 63
    assert scores['c'] == 1 / 62
    assert rrf([], ['b', 'a']) == [('b', 1 / 61), ('a', 1 / 62)]
    assert rrf(['b'], ['a']) == [('a', 1 / 61), ('b', 1 / 61)]


class Store:
    def __init__(self):
        self.entries = {key: Evidence(key, 'material', 1, 'demo.md', i, '活动资料', 2.)
                        for i, key in enumerate('abcde', 1)}
    def keyword(self, class_id, query, limit):
        assert class_id == 1
        return list(self.entries.values())
    def hydrate(self, class_id, ids):
        assert class_id == 1
        return {key: value for key, value in self.entries.items() if key in ids}


class Embedding:
    def embed(self, query):
        return [1., 0.]


class Vectors:
    def search(self, class_id, vector, limit):
        assert class_id == 1
        return [dict(id=key, score=score, payload=dict(class_id=1, material_id='material',
                knowledge_entry_id=1, chunk_id=key, chunk_index=i))
                for i, (key, score) in enumerate([('a', .34), ('b', .35), ('c', .8), ('foreign', .99)], 1)]


def test_retrieval_and_answering():
    scope = Scope(1, 'teacher', 1)
    store = Store()
    retrieval = Retrieval(store, Embedding(), Vectors())
    assert [e.chunk_id for e in retrieval.search(scope, '活动', 'vector')] == ['c', 'b']
    before = [e.chunk_id for e in retrieval.search(scope, '活动')]
    store.entries = {key: replace(e, score=e.score * 1000) for key, e in store.entries.items()}
    assert [e.chunk_id for e in retrieval.search(scope, '活动')] == before
    class Dialogue:
        calls = 0
        response = '活动资料 [1]'
        def answer(self, question, evidence):
            self.calls += 1
            assert len(evidence) == 4
            assert set(evidence[0]) == {'number', 'title', 'chunk_index', 'chunk_text'}
            return self.response
    dialogue = Dialogue()
    answering = Answering(retrieval, dialogue)
    result = answering.ask(scope, '活动')
    assert result['sources'][0]['number'] == 1
    assert len(result['sources']) == 1
    for response in ('', '没有引用', '[0]', '[5]'):
        dialogue.response = response
        with pytest.raises(InvalidAnswer):
            answering.ask(scope, '活动')
    store.entries = {}
    calls = dialogue.calls
    assert answering.ask(scope, '不存在') == dict(answer=NO_EVIDENCE, sources=[])
    assert dialogue.calls == calls
    class Broken:
        def embed(self, query):
            raise Unavailable()
    retrieval.embedding = Broken()
    with pytest.raises(Unavailable):
        answering.ask(scope, '活动')
    assert retrieval.search(scope, '活动', 'keyword') == []
    for query, mode in [('', 'vector'), (' ', 'hybrid'), ([], 'hybrid'), ('q', 'bad')]:
        with pytest.raises(InvalidInput):
            retrieval.search(scope, query, mode)


def test_core_import_boundary():
    for path in (Path(__file__).resolve().parents[2] / 'app/retrieval').glob('*.py'):
        tree = ast.parse(path.read_text(encoding='utf-8'))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                modules = [name.name for name in node.names]
            elif isinstance(node, ast.ImportFrom):
                modules = [node.module or '']
            else:
                continue
            assert not any(module.split('.')[0] in ('flask', 'sqlite3', 'pymysql') for module in modules)
