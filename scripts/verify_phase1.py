"""Isolated infrastructure verification; no business schema or real data."""
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def main():
    env = os.environ.copy()
    env.pop('SECRET_KEY', None)
    env['DB_PASSWORD'] = secrets.token_hex(24)
    env['MYSQL_ROOT_PASSWORD'] = secrets.token_hex(24)
    project = 'campusclaw-phase1-test-' + secrets.token_hex(4)
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        env['DEV_HTTP_PORT'] = str(sock.getsockname()[1])
    prefix = ['docker', 'compose', '-p', project, '-f', str(ROOT / 'docker-compose.yml')]
    def run(args, check=True):
        result = subprocess.run(prefix + args, cwd=ROOT, env=env, capture_output=True, text=True)
        output = result.stdout + result.stderr
        for key in ('DB_PASSWORD', 'MYSQL_ROOT_PASSWORD'):
            if env[key] in output:
                raise RuntimeError('Credential appeared in command output (suppressed)')
        if check and result.returncode:
            lines = output.splitlines()
            print('\n'.join(lines[:20] + lines[-25:]))
            raise RuntimeError('Verification command failed: ' + args[0])
        return result
    try:
        run(['config', '--quiet'])
        print('PASS compose config', flush=True)
        run(['build', 'app'])
        print('PASS Go image build', flush=True)
        run(['up', '-d', '--wait', '--wait-timeout', '240'])
        run(['exec', '-T', 'nginx', 'nginx', '-t'])
        print('PASS Nginx configuration and healthy startup without SECRET_KEY', flush=True)
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        def health():
            try:
                response = opener.open('http://127.0.0.1:' + env['DEV_HTTP_PORT'] + '/health', timeout=8)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                return response.status, json.load(response), response.headers.get('Set-Cookie')
        for attempt in range(15):
            try:
                result = health()
                break
            except urllib.error.URLError:
                if attempt == 14:
                    raise
                time.sleep(1)
        assert result == (200, {'status': 'ok'}, None)
        for service in ('app', 'mysql'):
            container = run(['ps', '-q', service]).stdout.strip()
            inspected = subprocess.run(['docker', 'inspect', '--format', '{{json .HostConfig.PortBindings}}', container], capture_output=True, text=True, check=True)
            assert json.loads(inspected.stdout) in ({}, None)
        print('PASS real MySQL health and no app/mysql published ports', flush=True)
        missing = run(['run', '--rm', '--no-deps', '-e', 'DB_PASSWORD=', 'app'], check=False)
        assert missing.returncode != 0 and 'missing required configuration: DB_PASSWORD' in missing.stderr
        print('PASS missing configuration refuses startup', flush=True)
        run(['stop', 'mysql'])
        assert health() == (503, {'status': 'unhealthy'}, None)
        unavailable = run(['run', '--rm', '--no-deps', 'app'], check=False)
        assert unavailable.returncode != 0 and 'startup dependencies unavailable' in unavailable.stderr
        print('PASS MySQL outage returns 503 and prevents new app startup', flush=True)
        run(['logs', '--no-color'])
        print('PASS generated verification credentials absent from container logs', flush=True)
    except Exception:
        print(run(['logs', '--no-color', '--tail', '15', 'app'], check=False).stdout)
        raise
    finally:
        # Only this invocation's randomly named test project and its volumes.
        run(['down', '--volumes', '--remove-orphans'])
        print('Temporary test project cleaned up: ' + project, flush=True)


if __name__ == '__main__':
    main()
