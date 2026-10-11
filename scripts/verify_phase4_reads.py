"""Phase 4 read/recovery real MySQL acceptance. Creates only new isolated resources; deletes none."""
import os
from pathlib import Path
import re
import secrets
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]


def main():
    name = 'campusclaw-phase4-read-' + secrets.token_hex(4)
    env = os.environ.copy()
    env.update(DB_HOST=name, DB_PORT='3306', DB_NAME='phase4_read', DB_USER='phase4_read',
               DB_PASSWORD=secrets.token_hex(24), MYSQL_ROOT_PASSWORD=secrets.token_hex(24),
               JWT_SIGNING_SECRET=secrets.token_hex(32), UPLOAD_ROOT='/tmp/unused',
               PHASE4_READ_ISOLATED='1', PHASE4_READ_SERVER='/tmp/phase4-read-server', GOPROXY='https://goproxy.cn')
    env.update(MYSQL_DATABASE=env['DB_NAME'], MYSQL_USER=env['DB_USER'], MYSQL_PASSWORD=env['DB_PASSWORD'])
    env.pop('SECRET_KEY', None)

    def run(args, check=True, timeout=60):
        started = time.monotonic()
        process = subprocess.Popen(['docker', *args], cwd=ROOT, env=env,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        while True:
            try:
                out, err = process.communicate(timeout=15)
                break
            except subprocess.TimeoutExpired:
                print(f'Waiting for isolated docker {args[0]}: {int(time.monotonic()-started)}s', flush=True)
                if time.monotonic()-started >= timeout:
                    process.kill()
                    process.communicate()
                    raise RuntimeError('Isolated command timed out; resources retained')
        output = out + err
        if any(env[k] in output for k in ('DB_PASSWORD', 'MYSQL_ROOT_PASSWORD', 'JWT_SIGNING_SECRET')) or re.search(r'eyJ[\w-]+\.[\w-]+\.[\w-]+', output):
            raise RuntimeError('Sensitive output suppressed')
        if check and process.returncode:
            print(output, flush=True)
            raise RuntimeError('Isolated verification failed')
        return process.returncode, output

    created = False
    print('Isolated resources: ' + name + '; volume: ' + name + '-data', flush=True)
    try:
        run(['network', 'create', '--internal', name])
        run(['volume', 'create', name+'-data'])
        run(['run', '-d', '--name', name, '--network', name,
             '--mount', 'type=volume,source='+name+'-data,target=/var/lib/mysql',
             '-e', 'MYSQL_DATABASE', '-e', 'MYSQL_USER', '-e', 'MYSQL_PASSWORD', '-e', 'MYSQL_ROOT_PASSWORD',
             'mysql:8.4.4', '--max-allowed-packet=67108864', '--log-bin-trust-function-creators=1'])
        created = True
        for _ in range(90):
            code, _ = run(['exec', name, 'sh', '-c', 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --protocol=TCP -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -e "SELECT 1"'], check=False)
            if code == 0:
                break
            time.sleep(1)
        else:
            raise RuntimeError('Isolated MySQL did not become ready')
        print('PASS isolated MySQL 8.4.4 ready; no host ports or Demo mounts', flush=True)
        # The runner uses the default bridge for dependency downloads plus the
        # internal test network; the database has only the internal network.
        run(['create', '--name', name+'-go', '--network', 'bridge',
             '--mount', f'type=bind,source={ROOT},target=/src,readonly', '-w', '/src',
             *sum((['-e', k] for k in ('DB_HOST', 'DB_PORT', 'DB_NAME', 'DB_USER', 'DB_PASSWORD',
                                     'JWT_SIGNING_SECRET', 'UPLOAD_ROOT', 'PHASE4_READ_ISOLATED', 'PHASE4_READ_SERVER', 'GOPROXY')), []),
             'golang:1.26.0-alpine3.23', 'sh', '-c',
             'go build -mod=readonly -o /tmp/phase4-read-server ./cmd/server && '
             'go list -mod=readonly -deps ./cmd/server > /tmp/server-deps && '
             '! grep -E "modernc.org|internal/migration|python" /tmp/server-deps && '
             'go test -mod=readonly ./internal/storage ./internal/material ./internal/http ./internal/repository/mysql ./internal/recovery && go test -mod=readonly -count=1 -timeout=180s -v -tags phase4readintegration ./internal/repository/mysql -run TestPhase4ReadRecoveryRealMySQL'])
        run(['network', 'connect', name, name+'-go'])
        _, output = run(['start', '-a', name+'-go'], timeout=600)
        # docker start -a does not reliably propagate the container exit status.
        _, code = run(['inspect', '--format', '{{.State.ExitCode}}', name+'-go'])
        print(output, flush=True)
        if code.strip() != '0':
            raise RuntimeError('Go integration runner failed')
        print('PASS Phase 4 read/recovery real MySQL acceptance and server dependency boundary', flush=True)
    finally:
        if created:
            run(['stop', name], check=False)
        print('Retained test containers/network/volume (not deleted): ' + name, flush=True)


if __name__ == '__main__':
    main()
