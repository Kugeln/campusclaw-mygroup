import math
from .http import HTTP
from app.retrieval.types import Unavailable


class Embedding:
    def __init__(self, base_url, key, model, dimension, api_format='ark_multimodal'):
        if api_format not in ('ark_multimodal', 'openai'):
            raise ValueError('Invalid EMBEDDING_API_FORMAT')
        self.api_format = api_format
        self.http, self.model, self.dimension = HTTP(base_url, key), model, dimension

    def embed(self, text):
        try:
            if self.api_format == 'ark_multimodal':
                value = self.http.request('POST', '/embeddings/multimodal', dict(
                    model=self.model, input=[dict(type='text', text=text)]))['data']['embedding']
            else:
                value = self.http.request('POST', '/embeddings', dict(
                    model=self.model, input=text))['data'][0]['embedding']
            if not isinstance(value, list) or len(value) != self.dimension:
                raise ValueError()
            if any(type(v) not in (int, float) or not math.isfinite(v) for v in value) or not any(value):
                raise ValueError()
            return value
        except (KeyError, IndexError, TypeError, ValueError):
            raise Unavailable('Invalid embedding response') from None
