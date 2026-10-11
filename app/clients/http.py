import json
from urllib.request import Request, urlopen
from urllib.error import URLError

from app.retrieval.types import Unavailable


class HTTP:
    def __init__(self, base_url, key=None, timeout=15):
        self.base_url, self.key, self.timeout = base_url.rstrip('/'), key, timeout

    def request(self, method, path, body=None):
        headers = {'Content-Type': 'application/json'}
        if self.key:
            headers['Authorization'] = 'Bearer ' + self.key
        try:
            req = Request(self.base_url + path, data=None if body is None else json.dumps(body).encode(),
                          headers=headers, method=method)
            with urlopen(req, timeout=self.timeout) as response:
                return json.load(response)
        except (URLError, OSError, ValueError, TypeError):
            raise Unavailable('Gateway unavailable') from None
