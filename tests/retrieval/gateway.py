"""Tiny controlled HTTP gateway for tests only; never a demo model fallback."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread
import json


class Gateway:
    def __init__(self):
        self.calls = []
        self.embedding_value = None
        self.dialogue_value = None
        self.fail_embedding_at = None
        self.status = 200
        owner = self
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass
            def do_POST(self):
                data = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                owner.calls.append((self.path, data))
                status = owner.status
                if '/embeddings' in self.path:
                    count = sum('/embeddings' in path for path, _ in owner.calls)
                    if count == owner.fail_embedding_at:
                        status = 503
                    text = data['input'][0]['text'] if isinstance(data.get('input'), list) else data.get('input')
                    vector = [1., 0., 0.] if '星河' in text else [0., 1., 0.] if '海风' in text else [0., 0., 1.]
                    if owner.embedding_value is not None:
                        vector = owner.embedding_value
                    result = dict(data=dict(embedding=vector)) if self.path.endswith('/multimodal') else dict(data=[dict(embedding=vector)])
                else:
                    answer = owner.dialogue_value if owner.dialogue_value is not None else '星河活动周三下午三点在图书馆二楼举行。[1]'
                    result = dict(choices=[dict(message=dict(content=answer))])
                payload = json.dumps(result).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.url = 'http://127.0.0.1:' + str(self.server.server_port)
        self.thread = Thread(target=self.server.serve_forever, daemon=True)

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *args):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()
