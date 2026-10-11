"""New MySQL volume + host Go/Edge, on random loopback ports; retain resources."""
import os
from pathlib import Path
import re
import secrets
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]


def main():
    suffix = secrets.token_hex(4)
    name = 'campusclaw-phase4-ui-' + suffix
    runtime = ROOT / 'runtime' / name
    runtime.mkdir(parents=True, exist_ok=False)
    env = os.environ.copy()
    env.update(DB_HOST='127.0.0.1', DB_NAME='phase4_ui_' + suffix, DB_USER='phase4_ui',
               DB_PASSWORD=secrets.token_hex(24), MYSQL_ROOT_PASSWORD=secrets.token_hex(24),
               JWT_SIGNING_SECRET=secrets.token_hex(32), UPLOAD_ROOT=str(runtime / 'unused'),
               PHASE4_UI_ISOLATED='1', PHASE4_UI_PASSWORD=secrets.token_hex(24),
               PHASE4_UI_SERVER=str(runtime / 'server.exe'),
               PHASE4_UI_PYTHON=str(ROOT / '.venv/Scripts/python.exe'),
               PHASE4_UI_BROWSER_SCRIPT=str(ROOT / 'scripts/check_phase4_go_browser.py'))
    env.update(MYSQL_DATABASE=env['DB_NAME'], MYSQL_USER=env['DB_USER'], MYSQL_PASSWORD=env['DB_PASSWORD'])
    env.pop('SECRET_KEY', None)
    env.pop('MAX_UPLOAD_BYTES', None)

    def run(args, check=True, timeout=60):
        start = time.monotonic()
        process = subprocess.Popen(args, cwd=ROOT, env=env, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, text=True, encoding='utf-8', errors='replace')
        while True:
            try:
                out, err = process.communicate(timeout=15)
                break
            except subprocess.TimeoutExpired:
                print(f'Waiting for isolated {args[0]}: {int(time.monotonic()-start)}s', flush=True)
                if time.monotonic() - start >= timeout:
                    process.kill()
                    process.communicate()
                    raise RuntimeError('Isolated command timeout; resources retained')
        output = out + err
        if any(env[k] in output for k in ('DB_PASSWORD', 'MYSQL_ROOT_PASSWORD', 'JWT_SIGNING_SECRET', 'PHASE4_UI_PASSWORD')) or re.search(r'eyJ[\w-]+\.[\w-]+\.[\w-]+', output):
            raise RuntimeError('Sensitive output suppressed')
        if check and process.returncode:
            print(output, flush=True)
            raise RuntimeError('Isolated verification failed')
        return process.returncode, output

    created = False
    print('Isolated resources: ' + name + '; volume: ' + name + '-data', flush=True)
    try:
        run(['docker', 'network', 'create', name])
        run(['docker', 'volume', 'create', name + '-data'])
        run(['docker', 'run', '-d', '--name', name, '--network', name,
             '-p', '127.0.0.1::3306', '--mount', 'type=volume,source=' + name + '-data,target=/var/lib/mysql',
             '-e', 'MYSQL_DATABASE', '-e', 'MYSQL_USER', '-e', 'MYSQL_PASSWORD', '-e', 'MYSQL_ROOT_PASSWORD',
             'mysql:8.4.4', '--max-allowed-packet=67108864'])
        created = True
        _, endpoint = run(['docker', 'port', name, '3306/tcp'])
        assert endpoint.strip().startswith('127.0.0.1:')
        env['DB_PORT'] = endpoint.strip().rsplit(':', 1)[1]
        for _ in range(90):
            code, _ = run(['docker', 'exec', name, 'sh', '-c', 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --protocol=TCP -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -e "SELECT 1"'], check=False)
            if code == 0:
                break
            time.sleep(1)
        else:
            raise RuntimeError('Isolated MySQL not ready')
        print('PASS MySQL 8.4.4 isolated new volume, random loopback port, no Demo mounts', flush=True)
        run(['go', 'build', '-mod=readonly', '-o', env['PHASE4_UI_SERVER'], './cmd/server'], timeout=180)
        _, output = run(['go', 'test', '-mod=readonly', '-count=1', '-timeout=240s', '-v', '-tags', 'phase4uiintegration',
                         './internal/repository/mysql', '-run', 'TestPhase4UIRealMySQL'], timeout=300)
        print(output, flush=True)
        print('PASS Phase 4 UI and material permission matrix', flush=True)
    finally:
        if created:
            run(['docker', 'stop', name], check=False)
        print('Retained container/network/volume and test binary: ' + name, flush=True)


if __name__ == '__main__':
    main()
