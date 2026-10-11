from .http import HTTP
from app.retrieval.types import Unavailable
import math
from uuid import UUID


class Qdrant:
    collection = '/collections/campusclaw_chunks'

    def __init__(self, base_url, dimension):
        self.http, self.dimension = HTTP(base_url), dimension

    @staticmethod
    def scope_filter(class_id, material_id=None):
        if type(class_id) is not int or class_id <= 0:
            raise ValueError('Trusted class required')
        must = [dict(key='class_id', match=dict(value=class_id))]
        if material_id is not None:
            must.append(dict(key='material_id', match=dict(value=material_id)))
        return dict(must=must)

    def check_configuration(self):
        result = self.http.request('GET', self.collection + '/exists')
        if not result['result']['exists']:
            return False
        info = self.http.request('GET', self.collection)['result']['config']['params']['vectors']
        if info.get('size') != self.dimension or info.get('distance') != 'Cosine':
            raise Unavailable('Vector collection configuration mismatch')
        return True

    def ensure(self):
        if not self.check_configuration():
            self.http.request('PUT', self.collection, dict(vectors=dict(size=self.dimension, distance='Cosine')))
        self.http.request('PUT', self.collection + '/index?wait=true', dict(field_name='class_id', field_schema='integer'))

    def validate_vector(self, vector):
        if (not isinstance(vector, list) or len(vector) != self.dimension
                or any(type(v) not in (int, float) or not math.isfinite(v) for v in vector)
                or not any(vector)):
            raise Unavailable('Invalid vector dimensions or values')

    @staticmethod
    def completed(result):
        if result.get('result', {}).get('status') != 'completed':
            raise Unavailable('Vector operation not confirmed')

    def upsert(self, class_id, pieces, vectors):
        self.scope_filter(class_id)
        points = []
        for piece, vector in zip(pieces, vectors, strict=True):
            self.validate_vector(vector)
            if str(UUID(piece['chunk_id'])) != piece['chunk_id']:
                raise Unavailable('Invalid point identifier')
            payload = {key: piece[key] for key in ('material_id', 'knowledge_entry_id', 'chunk_id', 'chunk_index')}
            payload['class_id'] = class_id
            points.append(dict(id=piece['chunk_id'], vector=vector, payload=payload))
        if points:
            self.ensure()
            self.completed(self.http.request('PUT', self.collection + '/points?wait=true', dict(points=points)))

    def delete(self, class_id, material_id):
        self.completed(self.http.request('POST', self.collection + '/points/delete?wait=true',
                                        dict(filter=self.scope_filter(class_id, material_id))))

    def search(self, class_id, vector, limit):
        self.validate_vector(vector)
        self.check_configuration()
        return self.http.request('POST', self.collection + '/points/search', dict(
            vector=vector, limit=limit, filter=self.scope_filter(class_id), score_threshold=.35,
            with_vector=False, with_payload=True))['result']
