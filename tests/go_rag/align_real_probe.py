"""Exactly one course chat request, then one embedding; fail closed, never retry."""
import datetime
import json
import os
from pathlib import Path
import sys
import urllib.error
import urllib.request


def main():
    root = Path(__file__).resolve().parents[2]
    out = Path(os.environ['ALIGN_REAL_EVIDENCE']).resolve()
    out.relative_to(root / 'runtime')
    out.mkdir(exist_ok=False)
    config = {}
    for line in (root / '.env.course.local').read_text(encoding='utf-8-sig').splitlines():
        if '=' in line and not line.startswith('#'):
            key, value = line.split('=', 1)
            config[key] = value.strip().strip('"').strip("'")
    assert config['DIALOGUE_BASE_URL'] == config['EMBEDDING_BASE_URL'] == 'https://ai-gateway.devops.hello1023.com/v1'
    assert config['DIALOGUE_MODEL'] == 'course-chat' and config['EMBEDDING_MODEL'] == 'course-embedding'
    assert config['EMBEDDING_API_FORMAT'] == 'openai' and config['EMBEDDING_DIMENSION'] == '2048'
    state = {'dialogues': 0, 'embeddings': 0, 'limits': {'dialogues': 12, 'embeddings': 8},
             'blocked': False, 'requests': [], 'automatic_retries': 0}

    def save():
        temporary = out / 'budget.tmp'
        temporary.write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding='utf-8')
        temporary.replace(out / 'budget.json')

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args, **kwargs):
            return None

    save()
    for kind, path, body, key in [
        ('dialogues', '/chat/completions', {'model': 'course-chat', 'messages': [{'role': 'user', 'content': '请只回复：课程网关连通。'}], 'max_tokens': 64, 'stream': False}, 'DIALOGUE_API_KEY'),
        ('embeddings', '/embeddings', {'model': 'course-embedding', 'input': '课程连通性验收：用等式性质理解一元一次方程。'}, 'EMBEDDING_API_KEY'),
    ]:
        assert not state['blocked'] and state[kind] < state['limits'][kind]
        state[kind] += 1
        row = {'kind': kind, 'stage': 'connectivity', 'path': path, 'request': body,
               'utc': datetime.datetime.now(datetime.timezone.utc).isoformat()}
        state['requests'].append(row)
        save()  # Reserve even an unsuccessful attempt before touching the network.
        try:
            request = urllib.request.Request(config['DIALOGUE_BASE_URL'] + path,
                      data=json.dumps(body).encode(), headers={'Content-Type': 'application/json',
                      'Authorization': 'Bearer ' + config[key]}, method='POST')
            with urllib.request.build_opener(NoRedirect).open(request, timeout=120) as response:
                row['http_status'] = response.status
                data = json.load(response)
            row['response_model'] = data.get('model')
            row['usage'] = data.get('usage')
            row['reported_cost'] = {k: data[k] for k in ('cost', 'total_cost', 'billing') if k in data}
            if kind == 'dialogues':
                answer = data['choices'][0]['message']['content']
                assert isinstance(answer, str) and answer.strip()
                row['answer'] = answer
                row['response_format_valid'] = True
            else:
                vector = data['data'][0]['embedding']
                assert isinstance(vector, list) and len(vector) == 2048
                assert all(isinstance(v, (float, int)) for v in vector)
                row['actual_dimension'] = len(vector)
                row['response_format_valid'] = True
            row['passed'] = True
        except urllib.error.HTTPError as error:
            row.update(http_status=error.code, passed=False, error_type='HTTPError')
            state['blocked'] = True
        except Exception as error:
            row.update(passed=False, error_type=type(error).__name__)
            state['blocked'] = True
        save()
        print(json.dumps({k: row[k] for k in ('kind', 'http_status', 'actual_dimension', 'passed', 'error_type') if k in row}), flush=True)
        if state['blocked']:
            return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
